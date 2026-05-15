package sandbox

import (
	"context"
	"fmt"
)

// Destroy kills an existing sandbox. The host output directory is left
// behind so the caller can still read files produced before destroy.
func (r *Runner) Destroy(ctx context.Context, req DestroyRequest) error {
	sb, err := r.attach(ctx, req.SandboxID)
	if err != nil {
		return err
	}
	if err := sb.Kill(ctx); err != nil {
		return fmt.Errorf("kill sandbox %s: %w", req.SandboxID, err)
	}
	return nil
}
