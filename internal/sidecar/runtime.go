package sidecar

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	proxyanthropic "github.com/jbeshir/demesne/internal/proxies/anthropic"
)

// SidecarResultsDir is the path the proxy sees inside the sidecar
// container for the host-bind-mounted results directory. The
// agent-vendor proxy writes usage.json under here after every
// request; the host runner reads it back from the matching
// ResultsHost path after the sandbox exits.
const SidecarResultsDir = "/sidecar-results"

// SidecarUsageFile is the absolute in-container path of the
// agent-vendor usage snapshot the proxy maintains.
const SidecarUsageFile = SidecarResultsDir + "/usage.json"

// EgressSidecarLabel is OpenSandbox's label on its own per-sandbox egress
// sidecar container. We use it to find the container ID so we can join
// its network namespace.
const EgressSidecarLabel = "opensandbox.io/egress-sidecar-for"

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
type ProxyConfig struct {
	AgentToken    string
	UpstreamToken string
	ResultsHost   string
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
	if cfg.AgentToken == "" {
		return nil, errors.New("sidecar.Start: AgentToken is required")
	}
	if cfg.UpstreamToken == "" {
		return nil, errors.New("sidecar.Start: UpstreamToken is required")
	}
	if cfg.ResultsHost == "" {
		return nil, errors.New("sidecar.Start: ResultsHost is required")
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
		"-v", cfg.ResultsHost + ":" + SidecarResultsDir,
		"-e", proxyanthropic.AuthTokenEnv + "=" + cfg.AgentToken,
		"-e", proxyanthropic.UpstreamTokenEnv + "=" + cfg.UpstreamToken,
		"-e", proxyanthropic.UsagePathEnv + "=" + SidecarUsageFile,
		imageRef,
	}
	//nolint:gosec // args composed from validated input
	cmd := exec.CommandContext(ctx, "docker", args...)
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
	cmd := exec.CommandContext(ctx, "docker", "rm", "-f", h.ContainerID)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("docker rm -f %s: %w\n%s", h.ContainerID, err, out)
	}
	return nil
}

func findEgressSidecar(ctx context.Context, sandboxID string) (string, error) {
	filter := fmt.Sprintf("label=%s=%s", EgressSidecarLabel, sandboxID)
	//nolint:gosec // filter composed from a validated UUID
	cmd := exec.CommandContext(ctx, "docker", "ps", "--filter", filter, "--format", "{{.ID}}")
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
