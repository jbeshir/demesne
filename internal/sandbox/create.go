package sandbox

import (
	"context"
	"fmt"
)

// Create provisions a persistent sandbox and returns its handle. The caller
// is responsible for eventually calling Destroy — no defer Kill here.
func (r *Runner) Create(ctx context.Context, req CreateRequest) (CreateResult, error) {
	return r.create(ctx, req, nil)
}

// create backs Create and the in-sandbox child sandbox_create. When
// child is set, the sandbox inherits the parent's /in and /workspace
// and its /out is /out/child/<name>.
func (r *Runner) create(ctx context.Context, req CreateRequest, child *childSpawn) (CreateResult, error) {
	sb, outputHost, _, err := r.prepareSandbox(ctx, sandboxPrepOptions{
		Image:          req.Image,
		Egress:         req.Egress,
		Files:          req.Files,
		Directories:    req.Directories,
		Tool:           "sandbox_create",
		TimeoutSeconds: persistentSandboxTTLSeconds,
		Child:          child,
	})
	if err != nil {
		return CreateResult{}, err
	}
	// Persistent sandboxes keep their sidecar (the Go module proxy) for
	// the life of the sandbox; it's reaped via the shared egress-sidecar
	// PID namespace when the sandbox is destroyed. No host-side handle to
	// track. If it can't start, tear down the orphan sandbox.
	if _, err := r.startGoproxySidecar(ctx, sb.ID()); err != nil {
		killSandbox(ctx, sb)
		return CreateResult{}, fmt.Errorf("start sidecar: %w", err)
	}
	return CreateResult{
		SandboxID:  sb.ID(),
		OutputPath: outputHost,
	}, nil
}
