//go:build integration

package sandbox

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	// Side-effect: register the codex agent and the openai proxy.
	_ "github.com/jbeshir/demesne/internal/agents/codex"
	"github.com/jbeshir/demesne/internal/mcpproxy"
	proxymcp "github.com/jbeshir/demesne/internal/proxies/mcp"
	proxyopenai "github.com/jbeshir/demesne/internal/proxies/openai"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	codexMCPFixtureServer     = "codex-mcp-fixture"
	codexMCPFixtureTool       = "return_fixture_marker"
	codexMCPFixtureMarker     = "DEMESNE_CODEX_MCP_FIXTURE_RESULT_V1"
	codexLongMCPFixtureMarker = "DEMESNE_CODEX_LONG_MCP_FIXTURE_RESULT_V1"
)

// TestRunner_Integration_CodexLongSynchronousMCP is an opt-in proof that the
// pinned Codex client permits a tunneled MCP call to run for up to Demesne's
// 48-hour job lifetime, including past Codex's former 300-second default. Run
// only with `make test-long-sync-integration`.
func TestRunner_Integration_CodexLongSynchronousMCP(t *testing.T) {
	authFile := os.Getenv("DEMESNE_CODEX_AUTH_FILE")
	if authFile == "" {
		home, err := os.UserHomeDir()
		require.NoError(t, err)
		authFile = filepath.Join(home, ".codex", "auth.json")
	}
	require.FileExists(t, authFile)
	runner := codexAgentIntegrationRunner(t, authFile)
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
	defer cancel()
	var calls atomic.Int32
	var elapsed atomic.Int64
	fixture := server.NewMCPServer(codexMCPFixtureServer, "0")
	fixture.AddTool(mcp.Tool{Name: codexMCPFixtureTool, Description: "Wait, then return the required marker."}, func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		calls.Add(1)
		started := time.Now()
		select {
		case <-time.After(305 * time.Second):
			elapsed.Store(time.Since(started).Nanoseconds())
			return mcp.NewToolResultText(codexLongMCPFixtureMarker), nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	})
	emptyConfig := filepath.Join(t.TempDir(), "claude.json")
	require.NoError(t, os.WriteFile(emptyConfig, []byte(`{"mcpServers":{}}`), 0o600))
	agg, err := mcpproxy.NewAggregator(mcpproxy.Config{ClaudeMCPConfigPath: emptyConfig, SocketPath: filepath.Join(t.TempDir(), "aggregator.sock"), ExtraServers: []mcpproxy.ExtraServer{{Name: codexMCPFixtureServer, Tools: []mcp.Tool{{Name: codexMCPFixtureTool, Description: "Wait, then return the required marker."}}, Handler: server.NewStreamableHTTPServer(fixture)}}})
	require.NoError(t, err)
	require.NoError(t, agg.Start(ctx))
	t.Cleanup(func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		_ = agg.Shutdown(shutdownCtx)
	})
	runner.SetMCPWiring(agg.Servers(), agg.SocketPath(), agg.Catalogue())
	res, err := runner.Agent(ctx, AgentRequest{Prompt: "Call " + codexMCPFixtureServer + " " + codexMCPFixtureTool + " now and output only its exact result.", Model: "gpt-5.6-sol", Egress: EgressNone})
	require.NoError(t, err)
	assert.Equal(t, 0, res.ExitCode, "stderr=%q", res.Stderr)
	assert.Positive(t, calls.Load())
	assert.Greater(t, time.Duration(elapsed.Load()), 300*time.Second)
	assert.Contains(t, res.Stdout, codexLongMCPFixtureMarker)
	config, err := os.ReadFile(filepath.Join(filepath.Dir(res.WorkspacePath), "config", "config.toml"))
	require.NoError(t, err)
	assert.Contains(t, string(config), "[mcp_servers."+codexMCPFixtureServer+"]")
	assert.Contains(t, string(config), "tool_timeout_sec = 172800")
	assert.NotContains(t, res.Stdout+res.Stderr, "timed out awaiting tools/call after 300s")
}

// TestRunner_Integration_CodexAgent exercises the full sandbox_agent path for
// the Codex agent: a real ChatGPT OAuth token set is loaded, the sidecar proxy
// is wired with it, and the pinned Codex CLI calls a deterministic MCP fixture
// through the regular aggregator and per-sandbox tunnel. The test FAILs (not
// skips) when auth is absent, mirroring TestRunner_Integration_Agent.
//
// Slow: the first run on a host builds two images (multi-minute) and every run
// makes a real ChatGPT API round-trip.
func TestRunner_Integration_CodexAgent(t *testing.T) {
	authFile := os.Getenv("DEMESNE_CODEX_AUTH_FILE")
	if authFile == "" {
		home, err := os.UserHomeDir()
		require.NoError(t, err, "os.UserHomeDir() failed")
		authFile = filepath.Join(home, ".codex", "auth.json")
	}
	require.FileExists(t, authFile,
		"Codex auth file not found at %s; set DEMESNE_CODEX_AUTH_FILE or place auth.json at ~/.codex/auth.json",
		authFile)

	data, err := os.ReadFile(authFile) //nolint:gosec // path from env or user home
	require.NoError(t, err, "read Codex auth file %s", authFile)

	ts, err := proxyopenai.ParseAuthJSON(data)
	require.NoError(t, err, "parse Codex auth file %s", authFile)
	require.NotEmpty(t, ts.AccessToken,
		"Codex auth file %s: tokens.access_token is empty; check DEMESNE_CODEX_AUTH_FILE / ~/.codex/auth.json",
		authFile)
	require.NotEmpty(t, ts.RefreshToken,
		"Codex auth file %s: tokens.refresh_token is empty; check DEMESNE_CODEX_AUTH_FILE / ~/.codex/auth.json",
		authFile)

	runner := codexAgentIntegrationRunner(t, authFile)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Keep this fixture independent of host MCP and OAuth configuration. Its
	// handler call is host-side evidence that Codex reached the tunnel; its
	// distinct result marker must then make the round trip back to Codex's final
	// response, rather than merely letting the model claim it called a tool.
	var calls atomic.Int32
	fixture := server.NewMCPServer(codexMCPFixtureServer, "0")
	fixture.AddTool(mcp.Tool{
		Name:        codexMCPFixtureTool,
		Description: "Return the required deterministic integration-test marker.",
	}, func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		calls.Add(1)
		return mcp.NewToolResultText(codexMCPFixtureMarker), nil
	})
	emptyConfig := filepath.Join(t.TempDir(), "claude.json")
	require.NoError(t, os.WriteFile(emptyConfig, []byte(`{"mcpServers":{}}`), 0o600))
	agg, err := mcpproxy.NewAggregator(mcpproxy.Config{
		ClaudeMCPConfigPath: emptyConfig,
		SocketPath:          filepath.Join(t.TempDir(), "aggregator.sock"),
		ExtraServers: []mcpproxy.ExtraServer{{
			Name:    codexMCPFixtureServer,
			Tools:   []mcp.Tool{{Name: codexMCPFixtureTool, Description: "Return the required deterministic integration-test marker."}},
			Handler: server.NewStreamableHTTPServer(fixture),
		}},
	})
	require.NoError(t, err)
	require.NoError(t, agg.Start(ctx))
	t.Cleanup(func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		_ = agg.Shutdown(shutdownCtx)
	})
	runner.SetMCPWiring(agg.Servers(), agg.SocketPath(), agg.Catalogue())

	res, err := runner.Agent(ctx, AgentRequest{
		Prompt: "Call the " + codexMCPFixtureServer + " MCP server's " +
			codexMCPFixtureTool + " tool now. Do not answer until it returns. Then output only the exact text the tool call returned, verbatim and with nothing else.",
		Model:  "gpt-5.6-sol",
		Egress: EgressNone,
	})
	require.NoError(t, err)
	assert.Equal(t, 0, res.ExitCode, "stdout=%q", res.Stdout)
	assert.Greater(t, calls.Load(), int32(0), "Codex never invoked the MCP fixture")
	assert.Contains(t, res.Stdout, codexMCPFixtureMarker,
		"Codex did not return the MCP fixture's marker: stdout=%q", res.Stdout)

	// The config is generated by the normal Runner path and copied by the
	// wrapper into the sandbox-local CODEX_HOME before the pinned CLI executes.
	// Checking it here catches a missing/incorrect Codex MCP TOML entry without
	// inspecting any credentials.
	configPath := filepath.Join(filepath.Dir(res.WorkspacePath), "config", "config.toml")
	config, err := os.ReadFile(configPath) //nolint:gosec // path is under t.TempDir()
	require.NoError(t, err)
	assert.Contains(t, string(config), "[mcp_servers."+codexMCPFixtureServer+"]")
	expectedFixtureURL := fmt.Sprintf(
		"url = \"http://127.0.0.1:%d/mcp\"", proxymcp.FirstListenPort,
	)
	assert.Contains(t, string(config), expectedFixtureURL)
	for _, p := range []string{res.OutputPath, res.WorkspacePath} {
		assert.DirExists(t, p)
	}
	// usage.json is the robust signal for billing recording. CostUSD is indicative
	// under ChatGPT-OAuth (subscription billing), so a strict > 0 check may be
	// brittle depending on the model's pricing entry.
	assert.FileExists(t, filepath.Join(res.OutputPath, "usage.json"))
	assert.GreaterOrEqual(t, res.CostUSD, 0.0)
}

// codexAgentIntegrationRunner builds a Runner with CodexAuthFile wired, reading
// the OpenSandbox config from OPEN_SANDBOX_* env vars just like
// agentIntegrationRunner does for the Anthropic path.
func codexAgentIntegrationRunner(t *testing.T, authFile string) *Runner {
	t.Helper()
	domain := os.Getenv("OPEN_SANDBOX_DOMAIN")
	apiKey := os.Getenv("OPEN_SANDBOX_API_KEY")
	require.NotEmpty(t, domain, "OPEN_SANDBOX_DOMAIN is required for integration tests")
	require.NotEmpty(t, apiKey, "OPEN_SANDBOX_API_KEY is required for integration tests")
	return NewRunner(Config{
		CodexEnabled:       true,
		AllowedPaths:        []string{t.TempDir()},
		OutputRoot:          t.TempDir(),
		OpenSandboxDomain:   domain,
		OpenSandboxProtocol: envOr("OPEN_SANDBOX_PROTOCOL", "http"),
		OpenSandboxAPIKey:   apiKey,
		CodexAuthFile:       authFile,
	})
}
