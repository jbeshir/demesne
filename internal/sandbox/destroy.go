package sandbox

import (
	"context"
	"fmt"
	"log"

	"github.com/jbeshir/demesne/internal/sidecar"
)

// Destroy kills an existing sandbox. The host output directory is left
// behind so the caller can still read files produced before destroy.
func (r *Runner) Destroy(ctx context.Context, req DestroyRequest) error {
	sb, err := r.attach(ctx, req.SandboxID)
	if err != nil {
		return err
	}
	// Remove our sidecar before killing the sandbox. The sidecar shares the
	// OpenSandbox egress sidecar's network and PID namespace, so podman
	// refuses to remove the egress sidecar (which OpenSandbox does on Kill)
	// while ours still exists — leaking both. Best-effort: a cleanup failure
	// must not block the kill.
	if err := sidecar.Remove(ctx, string(req.SandboxID)); err != nil {
		log.Printf("sandbox_destroy: sidecar cleanup for %s: %v", req.SandboxID, err)
	}
	if err := sb.Kill(ctx); err != nil {
		return fmt.Errorf("kill sandbox %s: %w", req.SandboxID, err)
	}
	return nil
}
