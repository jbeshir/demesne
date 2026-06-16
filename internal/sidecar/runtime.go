package sidecar

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	proxyanthropic "github.com/jbeshir/demesne/internal/proxies/anthropic"
	proxymcp "github.com/jbeshir/demesne/internal/proxies/mcp"
	proxyopenai "github.com/jbeshir/demesne/internal/proxies/openai"
)

// SidecarResultsDir is the path the proxy sees inside the sidecar
// container for the host-bind-mounted results directory. The
// vendor proxy writes usage.json under here after every
// request; the host runner reads it back from the matching
// ResultsHost path after the sandbox exits.
const SidecarResultsDir = "/sidecar-results"

// SidecarMCPDir is the in-sidecar mount point for the directory
// holding the host MCP aggregator's unix socket. The MCP tunnel
// forwards over the socket at SidecarMCPDir/<socket-basename>.
const SidecarMCPDir = "/demesne-mcp"

// SidecarUsageFile is the absolute in-container path of the
// vendor-proxy usage snapshot it maintains.
const SidecarUsageFile = SidecarResultsDir + "/usage.json"

// EgressSidecarLabel is OpenSandbox's label on its own per-sandbox egress
// sidecar container. We use it to find the container ID so we can join
// its network namespace.
const EgressSidecarLabel = "opensandbox.io/egress-sidecar-for"

const dockerCmd = "docker"

// execCommand is a seam to allow tests to swap the docker invocation.
var execCommand = exec.CommandContext

// dockerCommand builds an exec.Cmd for the docker CLI through the
// execCommand seam so tests can swap it. Callers pass argv with a
// constant subcommand + validated hex container IDs / embed-hash refs
// / UUIDs — none of the args ever transit a shell.
func dockerCommand(ctx context.Context, args ...string) *exec.Cmd {
	return execCommand(ctx, dockerCmd, args...)
}

// sidecarStartSettle is how long Start waits after `docker run -d` returns
// before confirming the sidecar is still running (see verifySidecarRunning).
// A package var so tests can zero it.
var sidecarStartSettle = 300 * time.Millisecond

var containerIDRe = regexp.MustCompile(`^[0-9a-f]{8,64}$`)

// validateContainerID returns an error if id is not a lowercase hex string
// of 8–64 characters, which is the only shape a Docker/Podman container ID
// can have.
func validateContainerID(id string) error {
	if !containerIDRe.MatchString(id) {
		return fmt.Errorf("unexpected container ID format: %q", id)
	}
	return nil
}

// AnthropicProxyConfig carries the auth values the vendor proxy
// needs. The agent-facing token is validated by the proxy on every
// inbound request; the upstream token is what the proxy sends to the
// real vendor API. Both are passed via docker run -e, which keeps them
// out of image layers and out of the untrusted agent sandbox — the host
// is trusted, so visibility via docker inspect (Config.Env) to host
// processes is acceptable. ResultsHost is the host path bind-mounted to
// SidecarResultsDir; the vendor proxy writes its usage.json there.
type AnthropicProxyConfig struct {
	AgentToken    string
	UpstreamToken string
	ResultsHost   string
}

// CodexProxyConfig carries the auth values the OpenAI/Codex credential
// proxy needs. AgentToken is the per-sandbox fake token the proxy
// validates on every inbound request; Tokens is the ChatGPT OAuth token
// set, already refreshed+persisted host-side before launch — the proxy
// forwards its access token as-is and never refreshes. Both are passed
// via docker run -e, which keeps them out of image layers and out of the
// untrusted agent sandbox; env vars are visible via docker inspect
// (Config.Env) to host processes, but the host is trusted — the sandbox
// edge is the only trust boundary here.
// ResultsHost is bind-mounted to SidecarResultsDir; the proxy writes its
// usage.json there.
type CodexProxyConfig struct {
	AgentToken  string
	Tokens      proxyopenai.TokenSet
	ResultsHost string
}

// MCPTunnelConfig configures the sidecar's MCP tunnel: one loopback
// listener per upstream, all forwarding over the bind-mounted aggregator
// unix socket (SocketHost). A unix socket rather than a host TCP hop is
// what makes this work under rootless podman, where the sandbox network
// namespace can't reach a host-process port.
type MCPTunnelConfig struct {
	Upstreams  []proxymcp.Binding
	SocketHost string
}

// ProxyConfig carries the per-sandbox proxy configuration for sidecar
// startup. Exactly one of Anthropic or Codex is set for an agent run;
// Anthropic == nil means the Anthropic proxy is off; Codex == nil means
// the OpenAI/Codex proxy is off. MCP == nil means the MCP tunnel is off.
type ProxyConfig struct {
	Anthropic *AnthropicProxyConfig
	// Codex == nil means the OpenAI/Codex proxy is off.
	Codex *CodexProxyConfig
	MCP   *MCPTunnelConfig
}

// Handle is a running demesne sidecar container attached to an
// OpenSandbox sandbox.
type Handle struct {
	ContainerID string
}

// Start finds the OpenSandbox egress sidecar for the given sandbox ID,
// then starts a demesne sidecar container sharing its network
// namespace. Each registered proxy listens on its own fixed loopback
// port inside the sidecar; the namespace is isolated, so the ports are
// purely internal and have no host-side surface.
//
// imageRef is the image returned by EnsureImage. sandboxID is the
// OpenSandbox-issued UUID. cfg carries the per-sandbox auth values
// and host results path the vendor proxy needs.
func Start(ctx context.Context, sandboxID, imageRef string, cfg ProxyConfig) (*Handle, error) {
	egressID, err := findEgressSidecar(ctx, sandboxID)
	if err != nil {
		return nil, err
	}
	name := containerName(sandboxID)
	args := []string{
		"run", "-d", "--rm",
		"--name", name,
		"--network", "container:" + egressID,
		// Share the egress sidecar's PID namespace so that when
		// OpenSandbox kills the egress sidecar (sandbox destroyed or
		// TTL expired), our sidecar's processes get reaped too. With
		// --rm above, the container is then removed automatically. No
		// host-side reaper needed.
		"--pid", "container:" + egressID,
		// NET_ADMIN lets proxies set SO_MARK on their outbound sockets,
		// which is how they bypass the egress sidecar's daddr filter
		// without their upstream hosts being in the allowlist.
		"--cap-add", "NET_ADMIN",
	}
	proxyArgs, err := proxyRunArgs(cfg)
	if err != nil {
		return nil, err
	}
	args = append(args, proxyArgs...)
	args = append(args, imageRef)
	cmd := dockerCommand(ctx, args...)
	// Output (stdout only) — CombinedOutput would mix in Podman's
	// "Emulate Docker CLI..." stderr banner, corrupting the container
	// ID and silently breaking later docker rm -f calls.
	out, err := cmd.Output()
	if err != nil {
		var stderr []byte
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			stderr = ee.Stderr
		}
		return nil, fmt.Errorf("docker run demesne sidecar: %w\nstdout: %s\nstderr: %s", err, out, stderr)
	}
	containerID := strings.TrimSpace(string(out))
	if err := verifySidecarRunning(ctx, containerID); err != nil {
		// The container started but didn't stay running; best-effort remove
		// it in case --rm hasn't reaped it yet (idempotent if already gone).
		_ = dockerRemoveWithRetry(context.WithoutCancel(ctx), containerID)
		return nil, err
	}
	return &Handle{ContainerID: containerID}, nil
}

// verifySidecarRunning confirms the sidecar container is still running a
// short moment after `docker run -d` reported success. It catches the
// silent-failure case where the runtime accepts the container but its init
// can't exec the entrypoint: the embedded sidecar binary is linux/amd64, so
// on a container runtime that can't run linux/amd64 (e.g. an arm64 host with
// no amd64 emulation) the init exits immediately with "exec format error"
// and --rm removes the container — yet `docker run -d` still exits 0 with a
// container ID. Without this check Start would return a Handle for a dead
// sidecar and the failure would surface only later as a confusing
// proxy-unreachable error inside the agent run.
func verifySidecarRunning(ctx context.Context, containerID string) error {
	t := time.NewTimer(sidecarStartSettle)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
	}
	cmd := execCommand(ctx, dockerCmd, "inspect", "-f", "{{.State.Status}}", containerID)
	// Output (stdout only) — CombinedOutput would mix in Podman's
	// "Emulate Docker CLI..." stderr banner, so the status would never
	// compare equal to "running" and a live sidecar would be reported dead.
	out, err := cmd.Output()
	if err == nil && strings.TrimSpace(string(out)) == "running" {
		return nil
	}
	// status is "exited"/"created"/"dead", or — when --rm already removed the
	// immediately-exited container — inspect errors with "no such container".
	status := strings.TrimSpace(string(out))
	if err != nil {
		status = "not running (container already removed)"
	}
	return fmt.Errorf(
		"demesne sidecar did not stay running after start (status: %s). The embedded "+
			"sidecar binary is linux/amd64; the most likely cause is a container runtime "+
			"that cannot execute linux/amd64 (e.g. an arm64 host without amd64 emulation). "+
			"See the Requirements section of the README: the runtime must run linux/amd64 "+
			"natively, via a Docker/Podman Machine VM, Rosetta, or qemu-user-static. It can "+
			"also indicate an unexpected sidecar startup crash",
		status)
}

// proxyRunArgs builds the docker run -v/-e arguments for the per-sandbox
// proxies enabled in cfg: the vendor proxy (Anthropic OR
// Codex — never both, since they share the SidecarResultsDir mount) and
// the optional MCP tunnel. The Go module proxy needs no config, so it's
// not represented here. Validation mirrors each proxy's "all-or-nothing"
// contract.
func proxyRunArgs(cfg ProxyConfig) ([]string, error) {
	var args []string

	// Anthropic proxy env + results mount only for claude-code agent runs;
	// the sidecar binary skips that proxy when these are absent.
	if cfg.Anthropic != nil {
		if cfg.Anthropic.UpstreamToken == "" || cfg.Anthropic.ResultsHost == "" {
			return nil, errors.New("sidecar.Start: UpstreamToken and ResultsHost are required when AgentToken is set")
		}
		args = append(args,
			"-v", cfg.Anthropic.ResultsHost+":"+SidecarResultsDir,
			"-e", proxyanthropic.AuthTokenEnv+"="+cfg.Anthropic.AgentToken,
			"-e", proxyanthropic.UpstreamTokenEnv+"="+cfg.Anthropic.UpstreamToken,
			"-e", proxyanthropic.UsagePathEnv+"="+SidecarUsageFile,
		)
	}
	// OpenAI/Codex proxy env + results mount only for Codex agent runs.
	// Anthropic and Codex use the same SidecarResultsDir mount point, so
	// they are mutually exclusive — only one of cfg.Anthropic/cfg.Codex is
	// ever set for a given run.
	if cfg.Codex != nil {
		// The sidecar proxy forwards the access token as-is (refresh is
		// host-side), so only the access token + results mount are required.
		if cfg.Codex.Tokens.AccessToken == "" || cfg.Codex.ResultsHost == "" {
			return nil, errors.New(
				"sidecar.Start: Tokens.AccessToken and ResultsHost are required when Codex AgentToken is set")
		}
		// Token set serialized to pass to the sidecar via env, same as the anthropic upstream token.
		tokensJSON, err := json.Marshal(cfg.Codex.Tokens) //nolint:gosec // tokens passed to sidecar by design
		if err != nil {
			return nil, fmt.Errorf("sidecar.Start: marshal Codex tokens: %w", err)
		}
		args = append(args,
			"-v", cfg.Codex.ResultsHost+":"+SidecarResultsDir,
			"-e", proxyopenai.AuthTokenEnv+"="+cfg.Codex.AgentToken,
			"-e", proxyopenai.UpstreamTokensEnv+"="+string(tokensJSON),
			"-e", proxyopenai.UsagePathEnv+"="+SidecarUsageFile,
		)
	}
	// The MCP tunnel is optional. It reaches the host aggregator over a
	// bind-mounted unix socket — a filesystem hop that works under
	// rootless podman, where the sandbox network namespace can't reach
	// a host-process TCP port (and --add-host is rejected in
	// --network=container: mode anyway).
	if cfg.MCP != nil {
		if cfg.MCP.SocketHost == "" {
			return nil, errors.New("sidecar.Start: MCPSocketHost is required when MCPUpstreams are set")
		}
		socketDir := filepath.Dir(cfg.MCP.SocketHost)
		inSidecarSocket := SidecarMCPDir + "/" + filepath.Base(cfg.MCP.SocketHost)
		raw, err := json.Marshal(cfg.MCP.Upstreams)
		if err != nil {
			return nil, fmt.Errorf("sidecar.Start: marshal MCP bindings: %w", err)
		}
		args = append(args,
			"-v", socketDir+":"+SidecarMCPDir,
			"-e", proxymcp.SocketPathEnv+"="+inSidecarSocket,
			"-e", proxymcp.BindingsEnv+"="+string(raw),
		)
	}
	return args, nil
}

// dockerRemoveWithRetry runs `docker rm -f <target>` via execCommand, treats
// "no such container" as success (idempotent), and retries on transient
// docker/podman errors with a small bounded backoff. Toolkit-style: passive,
// no goroutines, no env reads; caller passes the already-resolved target.
func dockerRemoveWithRetry(ctx context.Context, target string) error {
	const attempts = 3
	backoff := []time.Duration{50 * time.Millisecond, 200 * time.Millisecond}
	var lastErr error
	for i := range attempts {
		cmd := execCommand(ctx, dockerCmd, "rm", "-f", target)
		out, err := cmd.CombinedOutput()
		if err == nil {
			return nil
		}
		lower := strings.ToLower(string(out))
		if strings.Contains(lower, "no such container") {
			return nil
		}
		lastErr = fmt.Errorf("docker rm -f %s: %w\n%s", target, err, out)
		if !isTransientDockerErr(lower) {
			return lastErr
		}
		if i < len(backoff) {
			t := time.NewTimer(backoff[i])
			select {
			case <-ctx.Done():
				t.Stop()
				return lastErr
			case <-t.C:
			}
		}
	}
	return lastErr
}

// isTransientDockerErr reports whether the docker/podman stderr indicates
// a retryable failure. Pattern matches are case-insensitive — caller
// lowercases. Conservative: false positives cost only an extra idempotent
// `docker rm -f` call.
func isTransientDockerErr(lowercaseOutput string) bool {
	patterns := []string{
		"permission denied", // observed: "rootless netns: kill network process: permission denied"
		"temporarily unavailable",
		"resource temporarily unavailable",
		"device or resource busy",
		"try again",
		"connection refused", // podman socket transient
		"i/o timeout",
	}
	for _, p := range patterns {
		if strings.Contains(lowercaseOutput, p) {
			return true
		}
	}
	return false
}

// containerName is the deterministic name of a sandbox's demesne sidecar.
// The persistent (sandbox_create) path discards the Handle, so teardown
// removes the sidecar by this name instead — see Remove.
func containerName(sandboxID string) string {
	return "demesne-sidecar-" + sandboxID
}

// Stop force-removes the sidecar container. Errors are returned so the
// caller can log them; cleanup never blocks the agent run.
func (h *Handle) Stop(ctx context.Context) error {
	if h == nil || h.ContainerID == "" {
		return nil
	}
	return dockerRemoveWithRetry(ctx, h.ContainerID)
}

// Remove force-removes a sandbox's demesne sidecar by its deterministic
// name, for teardown paths that don't hold the Handle (sandbox_create →
// Destroy). It MUST run before OpenSandbox tears down the egress sidecar:
// ours shares the egress container's network and PID namespace, so podman
// refuses to remove the egress while ours still exists, leaking both
// containers (which accumulate until new sandbox creates fail). Idempotent
// — a missing container is not an error.
func Remove(ctx context.Context, sandboxID string) error {
	return dockerRemoveWithRetry(ctx, containerName(sandboxID))
}

func findEgressSidecar(ctx context.Context, sandboxID string) (string, error) {
	filter := fmt.Sprintf("label=%s=%s", EgressSidecarLabel, sandboxID)
	cmd := dockerCommand(ctx, "ps", "--filter", filter, "--format", "{{.ID}}")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("locate egress sidecar for %s: %w", sandboxID, err)
	}
	id := strings.TrimSpace(string(out))
	if id == "" {
		return "", fmt.Errorf("no egress sidecar found for sandbox %s — egress must be enabled in OpenSandbox", sandboxID)
	}
	// Multi-line output means multiple containers matched; take the first
	// non-empty line.
	if i := strings.IndexAny(id, "\n\r"); i >= 0 {
		id = id[:i]
	}
	if err := validateContainerID(id); err != nil {
		return "", fmt.Errorf("locate egress sidecar for %s: %w", sandboxID, err)
	}
	return id, nil
}
