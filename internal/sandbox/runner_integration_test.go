//go:build integration

package sandbox

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	// Side-effect: register the claude-code agent and the anthropic + mcp proxies.
	_ "github.com/jbeshir/demesne/internal/agents/anthropic"
	"github.com/jbeshir/demesne/internal/mcpproxy"
	_ "github.com/jbeshir/demesne/internal/proxies/anthropic"
	_ "github.com/jbeshir/demesne/internal/proxies/mcp"
	"github.com/jbeshir/demesne/internal/sidecar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Integration tests drive a real sandbox end-to-end against an OpenSandbox
// instance. Gated behind the `integration` build tag so they are excluded
// from default `go test ./...` runs. Invoke via `make test-integration`,
// which sets the tag and loads OPEN_SANDBOX_* from `.env`.

// integrationRunner builds a Runner from OPEN_SANDBOX_* env vars. If they
// are unset the test fails — silent skips would hide a misconfigured
// integration environment.
func integrationRunner(t *testing.T) *Runner {
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
	})
}

// TestRunner_Integration_OutputMount verifies that anything written under
// /out inside the sandbox is preserved on the host at the returned OutputPath.
func TestRunner_Integration_OutputMount(t *testing.T) {
	runner := integrationRunner(t)

	res, err := runner.RunScript(context.Background(), ScriptRequest{
		Command: "echo hello > /out/x.txt && cat /out/x.txt",
		Image:   "python",
		Egress:  EgressNone,
	})
	require.NoError(t, err)
	assert.Equal(t, 0, res.ExitCode, "stdout=%q", res.Stdout)
	assert.Contains(t, res.Stdout, "hello")

	outFile := filepath.Join(res.OutputPath, "x.txt")
	contents, err := os.ReadFile(outFile) //nolint:gosec // path is under t.TempDir()
	require.NoError(t, err)
	assert.Contains(t, string(contents), "hello")
}

// TestRunner_Integration_GoModuleProxy proves the Go image + sidecar Go
// module proxy: a script with image=go and egress=none fetches an
// external dependency (only reachable via the sidecar's GOPROXY) and
// builds + runs it. If GOPROXY weren't wired through the sidecar, `go
// mod tidy` would fail under egress=none.
func TestRunner_Integration_GoModuleProxy(t *testing.T) {
	runner := integrationRunner(t)

	cmd := `set -e
cd /tmp && mkdir -p probe && cd probe
go mod init example.com/probe
cat > main.go <<'EOF'
package main
import ("fmt"; "github.com/google/uuid")
func main() { fmt.Println("uuid:", uuid.New().String()) }
EOF
go mod tidy
go build -o probe .
./probe
echo BUILD_OK`

	res, err := runner.RunScript(context.Background(), ScriptRequest{
		Command: cmd,
		Image:   "go",
		Egress:  EgressNone,
	})
	require.NoError(t, err)
	assert.Equal(t, 0, res.ExitCode, "stdout=%q", res.Stdout)
	assert.Contains(t, res.Stdout, "uuid:", "external dep fetched + built")
	assert.Contains(t, res.Stdout, "BUILD_OK")
}

// TestRunner_Integration_EgressNoneBlocksAll verifies that egress="none"
// denies both DNS lookups and raw-IP egress. The raw-IP assertion requires
// OpenSandbox to be configured with `[egress] mode = "dns+nft"`; with the
// packaged `mode = "dns"` default this test will (correctly) fail, because
// dns-only filtering leaks raw-IP traffic.
func TestRunner_Integration_EgressNoneBlocksAll(t *testing.T) {
	runner := integrationRunner(t)

	res, err := runner.RunScript(context.Background(), ScriptRequest{
		Command: `getent hosts pypi.org >/dev/null 2>&1 && echo DNS_OK || echo DNS_BLOCKED
python3 -c "import socket; s=socket.socket(); s.settimeout(3); s.connect(('1.1.1.1', 443))" 2>/dev/null && echo IP_OK || echo IP_BLOCKED`,
		Image:  "python",
		Egress: EgressNone,
	})
	require.NoError(t, err)
	assert.Contains(t, res.Stdout, "DNS_BLOCKED",
		"DNS lookup of pypi.org was not blocked under egress=none")
	assert.Contains(t, res.Stdout, "IP_BLOCKED",
		"raw-IP egress to 1.1.1.1:443 was not blocked under egress=none. "+
			"If OpenSandbox is using [egress] mode=\"dns\", switch to "+
			"\"dns+nft\" — dns-only filtering does not block raw-IP traffic.")
}

// TestRunner_Integration_EgressPackageManagersAllowsPyPI verifies that
// pypi.org resolves under egress="package-managers". DNS resolution alone
// proves the allow rule applied; we deliberately do not run a full
// `pip install` to keep the test off the public-internet critical path.
func TestRunner_Integration_EgressPackageManagersAllowsPyPI(t *testing.T) {
	runner := integrationRunner(t)

	res, err := runner.RunScript(context.Background(), ScriptRequest{
		Command: "getent hosts pypi.org",
		Image:   "python",
		Egress:  EgressPackageManagers,
	})
	require.NoError(t, err)
	assert.Equal(t, 0, res.ExitCode,
		"getent hosts pypi.org under egress=package-managers (stdout=%q)", res.Stdout)
}

// TestRunner_Integration_PersistentLifecycle exercises the full M2 happy
// path: create a persistent sandbox, exec twice, upload a file, exec
// against it, verify /out files on host, download a file, destroy, and
// confirm post-destroy operations error.
func TestRunner_Integration_PersistentLifecycle(t *testing.T) {
	runner := integrationRunner(t)
	ctx := context.Background()

	created, err := runner.Create(ctx, CreateRequest{Image: "python", Egress: EgressNone})
	require.NoError(t, err)
	sandboxID := created.SandboxID
	t.Logf("sandbox_id=%s output_dir=%s", sandboxID, created.OutputPath)
	t.Cleanup(func() {
		_ = runner.Destroy(context.Background(), DestroyRequest{SandboxID: sandboxID})
	})

	mustExec(t, runner, ctx, sandboxID, "echo step1 > /out/a")

	uploadSrc := filepath.Join(runner.cfg.AllowedPaths[0], "in.txt")
	require.NoError(t, os.WriteFile(uploadSrc, []byte("hello-from-host\n"), 0o600))
	require.NoError(t, runner.Upload(ctx, UploadRequest{
		SandboxID: sandboxID, HostSrc: uploadSrc, SandboxDst: "/tmp/in.txt",
	}))

	step2 := mustExec(t, runner, ctx, sandboxID, "cat /tmp/in.txt > /out/b && cat /out/b")
	assert.Contains(t, step2.Stdout, "hello-from-host")

	assertHostOutFiles(t, created.OutputPath, []string{"a", "b"})

	dl, err := runner.Download(ctx, DownloadRequest{SandboxID: sandboxID, SandboxSrc: "/etc/hostname"})
	require.NoError(t, err)
	contents, err := os.ReadFile(dl.HostPath)
	require.NoError(t, err)
	assert.NotEmpty(t, contents, "downloaded /etc/hostname is empty (path=%s)", dl.HostPath)

	require.NoError(t, runner.Destroy(ctx, DestroyRequest{SandboxID: sandboxID}))

	// After destroy, further operations should error — either at attach or
	// at command time. Either is acceptable: the sandbox is gone.
	_, err = runner.Exec(ctx, ExecRequest{SandboxID: sandboxID, Command: "true"})
	assert.Error(t, err, "Exec after Destroy succeeded; expected error")
}

func mustExec(t *testing.T, r *Runner, ctx context.Context, sandboxID SandboxID, cmd string) ExecResult {
	t.Helper()
	res, err := r.Exec(ctx, ExecRequest{SandboxID: sandboxID, Command: cmd})
	require.NoError(t, err, "Exec %q", cmd)
	require.Equal(t, 0, res.ExitCode, "Exec %q stdout=%q", cmd, res.Stdout)
	return res
}

// containerExists reports whether a container named name exists in any
// state, via the same docker-compat CLI the sidecar package uses.
func containerExists(t *testing.T, name string) bool {
	t.Helper()
	out, err := exec.Command("docker", "ps", "-a", "--filter", "name="+name, "--format", "{{.Names}}").Output()
	require.NoError(t, err)
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == name {
			return true
		}
	}
	return false
}

// egressSidecarExists reports whether OpenSandbox's egress sidecar for the
// given sandbox still exists.
func egressSidecarExists(t *testing.T, sandboxID string) bool {
	t.Helper()
	out, err := exec.Command("docker", "ps", "-a", "--filter",
		"label="+sidecar.EgressSidecarLabel+"="+sandboxID, "--format", "{{.ID}}").Output()
	require.NoError(t, err)
	return strings.TrimSpace(string(out)) != ""
}

// TestRunner_Integration_DestroyCleansUpSidecars guards the teardown leak:
// Destroy must remove the demesne sidecar (which shares the egress
// sidecar's namespaces) so OpenSandbox can then remove the egress sidecar.
// Before the fix both leaked, accumulating until new creates failed.
func TestRunner_Integration_DestroyCleansUpSidecars(t *testing.T) {
	runner := integrationRunner(t)
	ctx := context.Background()

	created, err := runner.Create(ctx, CreateRequest{Image: "python", Egress: EgressNone})
	require.NoError(t, err)
	sid := string(created.SandboxID)
	sidecarName := "demesne-sidecar-" + sid
	require.True(t, containerExists(t, sidecarName), "demesne sidecar should run after create")

	require.NoError(t, runner.Destroy(ctx, DestroyRequest{SandboxID: created.SandboxID}))

	// Our sidecar is removed synchronously by Destroy.
	assert.False(t, containerExists(t, sidecarName), "demesne sidecar leaked after destroy")
	// With our dependent gone, OpenSandbox can remove the egress sidecar.
	assert.Eventually(t, func() bool { return !egressSidecarExists(t, sid) },
		20*time.Second, 500*time.Millisecond, "egress sidecar leaked after destroy")
}

func assertHostOutFiles(t *testing.T, outputDir string, names []string) {
	t.Helper()
	for _, name := range names {
		assert.FileExists(t, filepath.Join(outputDir, name))
	}
}

// TestRunner_Integration_AutoRenew confirms that Exec calls Renew, pushing
// ExpiresAt forward. We don't wait the full TTL — a small delta after a
// short sleep is enough signal.
func TestRunner_Integration_AutoRenew(t *testing.T) {
	runner := integrationRunner(t)
	ctx := context.Background()

	created, err := runner.Create(ctx, CreateRequest{Image: "python", Egress: EgressNone})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = runner.Destroy(context.Background(), DestroyRequest{SandboxID: created.SandboxID})
	})

	sb, err := runner.attach(ctx, created.SandboxID)
	require.NoError(t, err)
	before, err := sb.GetInfo(ctx)
	require.NoError(t, err)
	if before.ExpiresAt == nil {
		t.Skip("sandbox created without an ExpiresAt; nothing to renew")
	}
	beforeExpiry := *before.ExpiresAt

	time.Sleep(2 * time.Second)

	_, err = runner.Exec(ctx, ExecRequest{SandboxID: created.SandboxID, Command: "true"})
	require.NoError(t, err)

	sb2, err := runner.attach(ctx, created.SandboxID)
	require.NoError(t, err)
	after, err := sb2.GetInfo(ctx)
	require.NoError(t, err)
	require.NotNil(t, after.ExpiresAt, "ExpiresAt missing after renew")
	assert.True(t, after.ExpiresAt.After(beforeExpiry),
		"ExpiresAt did not advance: before=%s after=%s", beforeExpiry, *after.ExpiresAt)
}

// TestRunner_Integration_Agent exercises the full sandbox_agent path:
// the demesne sidecar image is built (or reused) and started in the
// OpenSandbox egress sidecar's network namespace, the claude-code image
// is built (or reused), and an OAuth-authenticated request to
// api.anthropic.com flows through the per-sandbox proxy.
//
// Slow: the first run on a host builds two images (multi-minute) and
// every run does a real Anthropic API round-trip.
func TestRunner_Integration_Agent(t *testing.T) {
	oauthToken := os.Getenv("DEMESNE_CLAUDE_CODE_OAUTH_TOKEN")
	require.NotEmpty(t, oauthToken,
		"DEMESNE_CLAUDE_CODE_OAUTH_TOKEN is required for agent integration tests")

	runner := agentIntegrationRunner(t, oauthToken)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	res, err := runner.Agent(ctx, AgentRequest{
		Agent:  "claude-code",
		Prompt: "Reply with the single word PONG and nothing else.",
		Model:  "haiku",
		Egress: EgressNone,
	})
	require.NoError(t, err)
	assert.Equal(t, 0, res.ExitCode, "stdout=%q", res.Stdout)
	assert.Contains(t, res.Stdout, "PONG")
	for _, p := range []string{res.OutputPath, res.WorkspacePath} {
		assert.DirExists(t, p)
	}
	claudemd := filepath.Join(filepath.Dir(res.OutputPath), "config", "CLAUDE.md")
	body, err := os.ReadFile(claudemd) //nolint:gosec // path under t.TempDir()
	require.NoError(t, err)
	assert.Contains(t, string(body), "## Task")
}

func agentIntegrationRunner(t *testing.T, oauthToken string) *Runner {
	t.Helper()
	domain := os.Getenv("OPEN_SANDBOX_DOMAIN")
	apiKey := os.Getenv("OPEN_SANDBOX_API_KEY")
	require.NotEmpty(t, domain, "OPEN_SANDBOX_DOMAIN is required for integration tests")
	require.NotEmpty(t, apiKey, "OPEN_SANDBOX_API_KEY is required for integration tests")
	return NewRunner(Config{
		AllowedPaths:         []string{t.TempDir()},
		OutputRoot:           t.TempDir(),
		OpenSandboxDomain:    domain,
		OpenSandboxProtocol:  envOr("OPEN_SANDBOX_PROTOCOL", "http"),
		OpenSandboxAPIKey:    apiKey,
		ClaudeCodeOAuthToken: oauthToken,
	})
}

// TestRunner_Integration_Research exercises the sandbox_research path:
// a Claude Code instance running in a sandbox with open egress (so it
// can curl the live web). Asserts that:
//   - the agent can reach a public HTTPS endpoint outside Anthropic,
//   - the proxy populated usage.json with a non-zero cost.
//
// Slow: real Anthropic API round-trip plus an outbound HTTPS fetch.
func TestRunner_Integration_Research(t *testing.T) {
	oauthToken := os.Getenv("DEMESNE_CLAUDE_CODE_OAUTH_TOKEN")
	require.NotEmpty(t, oauthToken,
		"DEMESNE_CLAUDE_CODE_OAUTH_TOKEN is required for research integration tests")

	runner := agentIntegrationRunner(t, oauthToken)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	res, err := runner.Research(ctx, ResearchRequest{
		Agent:  "claude-code",
		Prompt: "Use the Bash tool to run `curl -sSf https://example.com/` " +
			"and reply with just the text inside the page's <title> tag.",
		Model: "haiku",
	})
	require.NoError(t, err)
	assert.Equal(t, 0, res.ExitCode, "stdout=%q", res.Stdout)
	assert.Contains(t, res.Stdout, "Example Domain",
		"expected the agent to fetch example.com and report its <title>")
	assert.Greater(t, res.CostUSD, 0.0, "research run should report non-zero cost")
	assert.FileExists(t, filepath.Join(res.OutputPath, "usage.json"))
}

// TestRunner_Integration_AgentWithMCP proves the host-MCP-proxy path
// end-to-end: a real aggregator (pointed at the host ~/.claude.json)
// exposes the workflowy server's read-only tools, the runner wires
// them into a fresh agent sandbox, and the agent reaches them through
// the per-sandbox MCP tunnel. The sidecar detects the egress
// sidecar's network gateway to reach the aggregator.
//
// Requires a host workflowy MCP server in ~/.claude.json (the default
// allowlist exposes its read tools).
func TestRunner_Integration_AgentWithMCP(t *testing.T) {
	oauthToken := os.Getenv("DEMESNE_CLAUDE_CODE_OAUTH_TOKEN")
	require.NotEmpty(t, oauthToken,
		"DEMESNE_CLAUDE_CODE_OAUTH_TOKEN is required for MCP integration tests")

	home, err := os.UserHomeDir()
	require.NoError(t, err)
	hostConfig := filepath.Join(home, ".claude.json")
	require.FileExists(t, hostConfig, "host MCP config is required for this test")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	agg, err := mcpproxy.NewAggregator(mcpproxy.Config{
		HostMCPConfigPath: hostConfig,
		SocketPath:        filepath.Join(t.TempDir(), "aggregator.sock"),
	})
	require.NoError(t, err)
	require.NoError(t, agg.Start(ctx))
	t.Cleanup(func() {
		shutCtx, shutCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutCancel()
		_ = agg.Shutdown(shutCtx)
	})

	cat := agg.Catalogue()
	require.Contains(t, cat, "workflowy",
		"host must have a workflowy MCP server configured for this test")

	runner := agentIntegrationRunner(t, oauthToken)
	runner.SetMCPWiring(agg.Servers(), agg.SocketPath(), cat)

	res, err := runner.Agent(ctx, AgentRequest{
		Agent:  "claude-code",
		Prompt: "Call the workflowy `search_nodes` tool with the query \"demesne\". " +
			"Then reply with exactly DONE and nothing else.",
		Model:  "haiku",
		Egress: EgressNone,
	})
	require.NoError(t, err)
	assert.Equal(t, 0, res.ExitCode, "stdout=%q", res.Stdout)

	// The generated config (now in the read-only config dir, not the
	// shared workspace) must reflect the wired-in server.
	mcpCfg, err := os.ReadFile( //nolint:gosec // path under t.TempDir()
		filepath.Join(filepath.Dir(res.WorkspacePath), "config", ".demesne-mcp.json"))
	require.NoError(t, err)
	assert.Contains(t, string(mcpCfg), "workflowy")
	assert.Contains(t, string(mcpCfg), "http://127.0.0.1:")
}

// TestRunner_Integration_ChildSandbox proves the M6 in-sandbox demesne
// server end-to-end: an agent calls the demesne `sandbox_script` tool
// to spawn a named child, the child's output lands at
// /out/child/<name> (visible to the parent on the host), and the
// parent's results.json rolls up.
//
// Points discovery at an empty MCP config so the only exposed server
// is demesne (deterministic), making the wiring independent of the
// host's ~/.claude.json.
func TestRunner_Integration_ChildSandbox(t *testing.T) {
	oauthToken := os.Getenv("DEMESNE_CLAUDE_CODE_OAUTH_TOKEN")
	require.NotEmpty(t, oauthToken,
		"DEMESNE_CLAUDE_CODE_OAUTH_TOKEN is required for child-sandbox integration tests")

	emptyConfig := filepath.Join(t.TempDir(), "claude.json")
	require.NoError(t, os.WriteFile(emptyConfig, []byte(`{"mcpServers":{}}`), 0o600))

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	runner := agentIntegrationRunner(t, oauthToken)
	name, tools, handler := runner.ChildMCPServer()
	agg, err := mcpproxy.NewAggregator(mcpproxy.Config{
		HostMCPConfigPath: emptyConfig,
		SocketPath:        filepath.Join(t.TempDir(), "aggregator.sock"),
		ExtraServers:      []mcpproxy.ExtraServer{{Name: name, Tools: tools, Handler: handler}},
	})
	require.NoError(t, err)
	require.NoError(t, agg.Start(ctx))
	t.Cleanup(func() {
		shutCtx, shutCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutCancel()
		_ = agg.Shutdown(shutCtx)
	})
	require.Equal(t, []string{"demesne"}, agg.Servers())
	runner.SetMCPWiring(agg.Servers(), agg.SocketPath(), agg.Catalogue())

	res, err := runner.Agent(ctx, AgentRequest{
		Agent:  "claude-code",
		Prompt: "Use the demesne MCP server's `sandbox_script` tool to spawn a child " +
			"named \"probe\" that runs the command: echo hello-from-child > /out/result.txt. " +
			"Then reply with exactly DONE and nothing else.",
		Model:  "sonnet",
		Egress: EgressNone,
	})
	require.NoError(t, err)
	assert.Equal(t, 0, res.ExitCode, "stdout=%q", res.Stdout)

	// The child's output is nested under the parent's /out.
	childFile := filepath.Join(res.OutputPath, "child", "probe", "result.txt")
	body, err := os.ReadFile(childFile) //nolint:gosec // path under t.TempDir()
	require.NoError(t, err, "child output should exist at /out/child/probe")
	assert.Contains(t, string(body), "hello-from-child")

	// The parent's results.json exists and rolls up (>= own usage).
	results := filepath.Join(res.OutputPath, "results.json")
	assert.FileExists(t, results)
	assert.GreaterOrEqual(t, res.TotalUsageUSD, res.CostUSD)
}
