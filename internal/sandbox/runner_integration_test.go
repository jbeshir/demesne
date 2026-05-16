//go:build integration

package sandbox

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	// Side-effect: register the claude-code agent and the anthropic proxy.
	_ "github.com/jbeshir/demesne/internal/agents/anthropic"
	_ "github.com/jbeshir/demesne/internal/proxies/anthropic"
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

func mustExec(t *testing.T, r *Runner, ctx context.Context, sandboxID, cmd string) ExecResult {
	t.Helper()
	res, err := r.Exec(ctx, ExecRequest{SandboxID: sandboxID, Command: cmd})
	require.NoError(t, err, "Exec %q", cmd)
	require.Equal(t, 0, res.ExitCode, "Exec %q stdout=%q", cmd, res.Stdout)
	return res
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
	claudemd := filepath.Join(filepath.Dir(res.OutputPath), "claudemd", "CLAUDE.md")
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
