//go:build integration

package sandbox

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	// Side-effect: register the codex agent and the openai proxy.
	_ "github.com/jbeshir/demesne/internal/agents/codex"
	proxyopenai "github.com/jbeshir/demesne/internal/proxies/openai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRunner_Integration_CodexAgent exercises the full sandbox_agent path for
// the Codex agent: a real ChatGPT OAuth token set is loaded, the sidecar proxy
// is wired with it, and a Codex-in-sandbox round-trip is performed. The test
// FAILs (not skips) when auth is absent, mirroring TestRunner_Integration_Agent.
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

	runner := codexAgentIntegrationRunner(t, ts)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	res, err := runner.Agent(ctx, AgentRequest{
		Agent:  "codex",
		Prompt: "Reply with the single word PONG and nothing else.",
		Model:  "gpt-5.5",
		Egress: EgressNone,
	})
	require.NoError(t, err)
	assert.Equal(t, 0, res.ExitCode, "stdout=%q", res.Stdout)
	assert.Contains(t, res.Stdout, "PONG")
	for _, p := range []string{res.OutputPath, res.WorkspacePath} {
		assert.DirExists(t, p)
	}
	// usage.json is the robust signal for billing recording. CostUSD is indicative
	// under ChatGPT-OAuth (subscription billing), so a strict > 0 check may be
	// brittle depending on the model's pricing entry.
	assert.FileExists(t, filepath.Join(res.OutputPath, "usage.json"))
	assert.GreaterOrEqual(t, res.CostUSD, 0.0)
}

// codexAgentIntegrationRunner builds a Runner with CodexAuth wired, reading
// the OpenSandbox config from OPEN_SANDBOX_* env vars just like
// agentIntegrationRunner does for the Anthropic path.
func codexAgentIntegrationRunner(t *testing.T, ts proxyopenai.TokenSet) *Runner {
	t.Helper()
	domain := os.Getenv("OPEN_SANDBOX_DOMAIN")
	apiKey := os.Getenv("OPEN_SANDBOX_API_KEY")
	require.NotEmpty(t, domain, "OPEN_SANDBOX_DOMAIN is required for integration tests")
	require.NotEmpty(t, apiKey, "OPEN_SANDBOX_API_KEY is required for integration tests")
	return NewRunner(Config{
		AllowedPaths:        []string{t.TempDir()},
		OutputRoot:          t.TempDir(),
		OpenSandboxDomain:   domain,
		OpenSandboxProtocol: envOr("OPEN_SANDBOX_PROTOCOL", "http"),
		OpenSandboxAPIKey:   apiKey,
		CodexAuth:           ts,
	})
}
