package sidecar

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	proxyanthropic "github.com/jbeshir/demesne/internal/proxies/anthropic"
	proxymcp "github.com/jbeshir/demesne/internal/proxies/mcp"
	proxyopenai "github.com/jbeshir/demesne/internal/proxies/openai"
)

// SidecarResultsDir is the path the proxy sees inside the sidecar
// container for the host-bind-mounted results directory. The
// agent-vendor proxy writes usage.json under here after every
// request; the host runner reads it back from the matching
// ResultsHost path after the sandbox exits.
const SidecarResultsDir = "/sidecar-results"

// SidecarMCPDir is the in-sidecar mount point for the directory
// holding the host MCP aggregator's unix socket. The MCP tunnel
// forwards over the socket at SidecarMCPDir/<socket-basename>.
const SidecarMCPDir = "/demesne-mcp"

// SidecarUsageFile is the absolute in-container path of the
// agent-vendor usage snapshot the proxy maintains.
const SidecarUsageFile = SidecarResultsDir + "/usage.json"

// EgressSidecarLabel is OpenSandbox's label on its own per-sandbox egress
// sidecar container. We use it to find the container ID so we can join
// its network namespace.
const EgressSidecarLabel = "opensandbox.io/egress-sidecar-for"

const dockerCmd = "docker"

// AnthropicProxyConfig carries the auth values the agent-vendor proxy
// needs. The agent-facing token is validated by the proxy on every
// inbound request; the upstream token is what the proxy sends to the
// real vendor API. Keeping both off-host avoids storing them in sidecar
// image layers or container args (visible via docker inspect) — they're
// passed via docker run -e instead. ResultsHost is the host path
// bind-mounted to SidecarResultsDir; the vendor proxy writes its
// usage.json there.
type AnthropicProxyConfig struct {
	AgentToken    string
	UpstreamToken string
	ResultsHost   string
}

// CodexProxyConfig carries the auth values the OpenAI/Codex credential
// proxy needs. AgentToken is the per-sandbox fake token the proxy
// validates on every inbound request; UpstreamKey is the real OpenAI API
// key the proxy substitutes before forwarding. Both are passed via
// docker run -e (kept off image layers / inspect-visible args).
// ResultsHost is bind-mounted to SidecarResultsDir; the proxy writes its
// usage.json there.
type CodexProxyConfig struct {
	AgentToken  string
	UpstreamKey string
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
// and host results path the agent-vendor proxy needs.
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
	//nolint:gosec // args composed from validated input
	cmd := exec.CommandContext(ctx, dockerCmd, args...)
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
	return &Handle{ContainerID: containerID}, nil
}

// proxyRunArgs builds the docker run -v/-e arguments for the per-sandbox
// proxies enabled in cfg: the agent-vendor credential proxy (Anthropic OR
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
		if cfg.Codex.UpstreamKey == "" || cfg.Codex.ResultsHost == "" {
			return nil, errors.New("sidecar.Start: UpstreamKey and ResultsHost are required when Codex AgentToken is set")
		}
		args = append(args,
			"-v", cfg.Codex.ResultsHost+":"+SidecarResultsDir,
			"-e", proxyopenai.AuthTokenEnv+"="+cfg.Codex.AgentToken,
			"-e", proxyopenai.UpstreamKeyEnv+"="+cfg.Codex.UpstreamKey,
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
	//nolint:gosec // h.ContainerID is a docker-returned ID
	cmd := exec.CommandContext(ctx, dockerCmd, "rm", "-f", h.ContainerID)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("docker rm -f %s: %w\n%s", h.ContainerID, err, out)
	}
	return nil
}

// Remove force-removes a sandbox's demesne sidecar by its deterministic
// name, for teardown paths that don't hold the Handle (sandbox_create →
// Destroy). It MUST run before OpenSandbox tears down the egress sidecar:
// ours shares the egress container's network and PID namespace, so podman
// refuses to remove the egress while ours still exists, leaking both
// containers (which accumulate until new sandbox creates fail). Idempotent
// — a missing container is not an error.
func Remove(ctx context.Context, sandboxID string) error {
	name := containerName(sandboxID)
	//nolint:gosec // name composed from a validated UUID
	cmd := exec.CommandContext(ctx, dockerCmd, "rm", "-f", name)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if strings.Contains(strings.ToLower(string(out)), "no such container") {
			return nil
		}
		return fmt.Errorf("docker rm -f %s: %w\n%s", name, err, out)
	}
	return nil
}

func findEgressSidecar(ctx context.Context, sandboxID string) (string, error) {
	filter := fmt.Sprintf("label=%s=%s", EgressSidecarLabel, sandboxID)
	//nolint:gosec // filter composed from a validated UUID
	cmd := exec.CommandContext(ctx, dockerCmd, "ps", "--filter", filter, "--format", "{{.ID}}")
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
	return id, nil
}
