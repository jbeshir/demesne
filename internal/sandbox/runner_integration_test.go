//go:build integration

package sandbox

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
	if domain == "" {
		t.Fatal("OPEN_SANDBOX_DOMAIN is required for integration tests")
	}
	if apiKey == "" {
		t.Fatal("OPEN_SANDBOX_API_KEY is required for integration tests")
	}
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
	if err != nil {
		t.Fatalf("RunScript: %v", err)
	}
	if res.ExitCode != 0 {
		t.Errorf("exit code = %d, want 0 (stdout=%q)", res.ExitCode, res.Stdout)
	}
	if !strings.Contains(res.Stdout, "hello") {
		t.Errorf("stdout = %q, want to contain 'hello'", res.Stdout)
	}

	outFile := filepath.Join(res.OutputPath, "x.txt")
	contents, err := os.ReadFile(outFile) //nolint:gosec // path is under t.TempDir()
	if err != nil {
		t.Fatalf("read %s: %v", outFile, err)
	}
	if !strings.Contains(string(contents), "hello") {
		t.Errorf("%s = %q, want to contain 'hello'", outFile, contents)
	}
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
	if err != nil {
		t.Fatalf("RunScript: %v", err)
	}
	if !strings.Contains(res.Stdout, "DNS_BLOCKED") {
		t.Errorf("DNS lookup of pypi.org was not blocked under egress=none (stdout=%q)", res.Stdout)
	}
	if !strings.Contains(res.Stdout, "IP_BLOCKED") {
		t.Errorf("raw-IP egress to 1.1.1.1:443 was not blocked under egress=none "+
			"(stdout=%q). If OpenSandbox is using [egress] mode=\"dns\", switch to "+
			"\"dns+nft\" — dns-only filtering does not block raw-IP traffic.", res.Stdout)
	}
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
	if err != nil {
		t.Fatalf("RunScript: %v", err)
	}
	if res.ExitCode != 0 {
		t.Errorf("getent hosts pypi.org exit=%d under egress=package-managers (stdout=%q)",
			res.ExitCode, res.Stdout)
	}
}

// TestRunner_Integration_PersistentLifecycle exercises the full M2 happy
// path: create a persistent sandbox, exec twice, upload a file, exec
// against it, verify /out files on host, download a file, destroy, and
// confirm post-destroy operations error.
func TestRunner_Integration_PersistentLifecycle(t *testing.T) {
	runner := integrationRunner(t)
	ctx := context.Background()

	created, err := runner.Create(ctx, CreateRequest{
		Image:  "python",
		Egress: EgressNone,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	sandboxID := created.SandboxID
	t.Logf("sandbox_id=%s output_dir=%s", sandboxID, created.OutputPath)

	// Ensure the sandbox is killed even if the test fails partway through.
	t.Cleanup(func() {
		_ = runner.Destroy(context.Background(), DestroyRequest{SandboxID: sandboxID})
	})

	step1, err := runner.Exec(ctx, ExecRequest{
		SandboxID: sandboxID,
		Command:   "echo step1 > /out/a",
	})
	if err != nil {
		t.Fatalf("Exec step1: %v", err)
	}
	if step1.ExitCode != 0 {
		t.Errorf("step1 exit=%d stdout=%q", step1.ExitCode, step1.Stdout)
	}

	// Upload a host file the sandbox can read.
	uploadSrc := filepath.Join(runner.cfg.AllowedPaths[0], "in.txt")
	if err := os.WriteFile(uploadSrc, []byte("hello-from-host\n"), 0o600); err != nil {
		t.Fatalf("write %s: %v", uploadSrc, err)
	}
	if err := runner.Upload(ctx, UploadRequest{
		SandboxID:  sandboxID,
		HostSrc:    uploadSrc,
		SandboxDst: "/tmp/in.txt",
	}); err != nil {
		t.Fatalf("Upload: %v", err)
	}

	step2, err := runner.Exec(ctx, ExecRequest{
		SandboxID: sandboxID,
		Command:   "cat /tmp/in.txt > /out/b && cat /out/b",
	})
	if err != nil {
		t.Fatalf("Exec step2: %v", err)
	}
	if step2.ExitCode != 0 {
		t.Errorf("step2 exit=%d stdout=%q", step2.ExitCode, step2.Stdout)
	}
	if !strings.Contains(step2.Stdout, "hello-from-host") {
		t.Errorf("step2 stdout = %q, want 'hello-from-host'", step2.Stdout)
	}

	// Both /out artifacts visible on host.
	for _, name := range []string{"a", "b"} {
		p := filepath.Join(created.OutputPath, name)
		if _, err := os.Stat(p); err != nil {
			t.Errorf("expected %s on host: %v", p, err)
		}
	}

	dl, err := runner.Download(ctx, DownloadRequest{
		SandboxID:  sandboxID,
		SandboxSrc: "/etc/hostname",
	})
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	contents, err := os.ReadFile(dl.HostPath) //nolint:gosec // path is under r.cfg.OutputRoot/<jobID>/downloads
	if err != nil {
		t.Fatalf("read %s: %v", dl.HostPath, err)
	}
	if len(contents) == 0 {
		t.Errorf("downloaded /etc/hostname is empty (path=%s)", dl.HostPath)
	}

	if err := runner.Destroy(ctx, DestroyRequest{SandboxID: sandboxID}); err != nil {
		t.Fatalf("Destroy: %v", err)
	}

	// After destroy, further operations should error — either at attach or
	// at command time. Either is acceptable: the sandbox is gone.
	if _, err := runner.Exec(ctx, ExecRequest{SandboxID: sandboxID, Command: "true"}); err == nil {
		t.Errorf("Exec after Destroy succeeded; expected error")
	}
}

// TestRunner_Integration_AutoRenew confirms that Exec calls Renew, pushing
// ExpiresAt forward. We don't wait the full TTL — a small delta after a
// short sleep is enough signal.
func TestRunner_Integration_AutoRenew(t *testing.T) {
	runner := integrationRunner(t)
	ctx := context.Background()

	created, err := runner.Create(ctx, CreateRequest{Image: "python", Egress: EgressNone})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Cleanup(func() {
		_ = runner.Destroy(context.Background(), DestroyRequest{SandboxID: created.SandboxID})
	})

	sb, err := runner.attach(ctx, created.SandboxID)
	if err != nil {
		t.Fatalf("attach: %v", err)
	}
	before, err := sb.GetInfo(ctx)
	if err != nil {
		t.Fatalf("GetInfo before: %v", err)
	}
	if before.ExpiresAt == nil {
		t.Skip("sandbox created without an ExpiresAt; nothing to renew")
	}
	beforeExpiry := *before.ExpiresAt

	time.Sleep(2 * time.Second)

	if _, err := runner.Exec(ctx, ExecRequest{SandboxID: created.SandboxID, Command: "true"}); err != nil {
		t.Fatalf("Exec: %v", err)
	}

	sb2, err := runner.attach(ctx, created.SandboxID)
	if err != nil {
		t.Fatalf("attach: %v", err)
	}
	after, err := sb2.GetInfo(ctx)
	if err != nil {
		t.Fatalf("GetInfo after: %v", err)
	}
	if after.ExpiresAt == nil {
		t.Fatalf("ExpiresAt missing after renew")
	}
	if !after.ExpiresAt.After(beforeExpiry) {
		t.Errorf("ExpiresAt did not advance: before=%s after=%s", beforeExpiry, *after.ExpiresAt)
	}
}
