package sandbox

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	opensandbox "github.com/alibaba/OpenSandbox/sdks/sandbox/go"
)

// Upload copies a host file into an existing sandbox. The host path is
// validated against AllowedPaths (same allowlist as mount sources); the
// sandbox destination path is passed through to OpenSandbox.
func (r *Runner) Upload(ctx context.Context, req UploadRequest) error {
	resolved, err := ValidateMountPath(req.HostSrc, r.cfg.AllowedPaths)
	if err != nil {
		return err
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return fmt.Errorf("stat %s: %w", req.HostSrc, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", req.HostSrc)
	}

	sb, err := r.attach(ctx, req.SandboxID)
	if err != nil {
		return err
	}

	f, err := openValidatedHostFile(resolved)
	if err != nil {
		return fmt.Errorf("open %s: %w", req.HostSrc, err)
	}
	defer func() { _ = f.Close() }()

	if err := sb.UploadFile(ctx, f, opensandbox.UploadFileOptions{
		FileName: filepath.Base(resolved),
		Metadata: opensandbox.FileMetadata{Path: req.SandboxDst},
	}); err != nil {
		return fmt.Errorf("upload %s -> %s: %w", req.HostSrc, req.SandboxDst, err)
	}
	return nil
}
