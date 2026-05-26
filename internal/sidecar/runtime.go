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

// ProxyConfig carries the per-sandbox configuration the agent-vendor
// proxy needs at sidecar startup. The agent-facing token is validated
// by the proxy on every inbound request; the upstream token is what
// the proxy sends to the real vendor API. Keeping both off-host
// avoids storing them in sidecar image layers or container args
// (visible via docker inspect) — they're passed via docker run -e
// instead.
//
// ResultsHost is the host path bind-mounted to SidecarResultsDir
// inside the sidecar; the vendor proxy writes its usage.json there.
//
// MCPUpstreams, when non-empty, configure the sidecar's MCP tunnel:
// one loopback listener per upstream, all forwarding over the
// bind-mounted aggregator unix socket (MCPSocketHost). A unix socket
// rather than a host TCP hop is what makes this work under rootless
// podman, where the sandbox network namespace can't reach a
// host-process port. Empty MCPUpstreams means no MCP tunnel.
type ProxyConfig struct {
	AgentToken    string
	UpstreamToken string
	ResultsHost   string
	MCPUpstreams  []proxymcp.Binding
	MCPSocketHost string
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
	// The Go module proxy runs in every sidecar with no config. The
	// Anthropic proxy is agent-mode only: its three values arrive
	// together or not at all.
	agentMode := cfg.AgentToken != ""
	if agentMode && (cfg.UpstreamToken == "" || cfg.ResultsHost == "") {
		return nil, errors.New("sidecar.Start: UpstreamToken and ResultsHost are required when AgentToken is set")
	}
	egressID, err := findEgressSidecar(ctx, sandboxID)
	if err != nil {
		return nil, err
	}
	name := "demesne-sidecar-" + sandboxID
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
	// Anthropic proxy env + results mount only for agent runs; the
	// sidecar binary skips that proxy when these are absent.
	if agentMode {
		args = append(args,
			"-v", cfg.ResultsHost+":"+SidecarResultsDir,
			"-e", proxyanthropic.AuthTokenEnv+"="+cfg.AgentToken,
			"-e", proxyanthropic.UpstreamTokenEnv+"="+cfg.UpstreamToken,
			"-e", proxyanthropic.UsagePathEnv+"="+SidecarUsageFile,
		)
	}
	// The MCP tunnel is optional. It reaches the host aggregator over a
	// bind-mounted unix socket — a filesystem hop that works under
	// rootless podman, where the sandbox network namespace can't reach
	// a host-process TCP port (and --add-host is rejected in
	// --network=container: mode anyway).
	if len(cfg.MCPUpstreams) > 0 {
		if cfg.MCPSocketHost == "" {
			return nil, errors.New("sidecar.Start: MCPSocketHost is required when MCPUpstreams are set")
		}
		socketDir := filepath.Dir(cfg.MCPSocketHost)
		inSidecarSocket := SidecarMCPDir + "/" + filepath.Base(cfg.MCPSocketHost)
		raw, err := json.Marshal(cfg.MCPUpstreams)
		if err != nil {
			return nil, fmt.Errorf("sidecar.Start: marshal MCP bindings: %w", err)
		}
		args = append(args,
			"-v", socketDir+":"+SidecarMCPDir,
			"-e", proxymcp.SocketPathEnv+"="+inSidecarSocket,
			"-e", proxymcp.BindingsEnv+"="+string(raw),
		)
	}
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
