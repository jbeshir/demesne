//go:build integration

package sandbox

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
