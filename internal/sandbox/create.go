package sandbox

import "context"

// Create provisions a persistent sandbox and returns its handle. The caller
// is responsible for eventually calling Destroy — no defer Kill here.
func (r *Runner) Create(ctx context.Context, req CreateRequest) (CreateResult, error) {
	sb, outputHost, _, err := r.prepareSandbox(ctx, sandboxPrepOptions{
		Image:       req.Image,
		Egress:      req.Egress,
		Files:       req.Files,
		Directories: req.Directories,
		Tool:        "sandbox_create",
	})
	if err != nil {
		return CreateResult{}, err
	}
	return CreateResult{
		SandboxID:  sb.ID(),
		OutputPath: outputHost,
	}, nil
}
