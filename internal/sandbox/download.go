package sandbox

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Download copies a file out of an existing sandbox into the host
// OutputRoot. The destination is fixed by convention — caller does not
// choose a host path — so there is no new authorisation surface beyond
// the existing OutputRoot.
//
// The per-sandbox host dir name (where /out lives) is recorded as the
// demesne.job metadata at create time; we fetch it via GetInfo so the
// downloads land alongside /out.
func (r *Runner) Download(ctx context.Context, req DownloadRequest) (DownloadResult, error) {
	sb, err := r.attach(ctx, req.SandboxID)
	if err != nil {
		return DownloadResult{}, err
	}

	info, err := sb.GetInfo(ctx)
	if err != nil {
		return DownloadResult{}, fmt.Errorf("inspect sandbox %s: %w", req.SandboxID, err)
	}
	jobID := info.Metadata[metadataDemesneJob]
	if jobID == "" {
		return DownloadResult{}, fmt.Errorf(
			"sandbox %s is missing %s metadata; not a demesne-managed sandbox",
			req.SandboxID, metadataDemesneJob)
	}

	hostDir := filepath.Join(r.cfg.OutputRoot, jobID, "downloads")
	if err := os.MkdirAll(hostDir, 0o750); err != nil {
		return DownloadResult{}, fmt.Errorf("create downloads dir: %w", err)
	}

	rc, err := sb.DownloadFile(ctx, req.SandboxSrc, "")
	if err != nil {
		return DownloadResult{}, fmt.Errorf("download %s: %w", req.SandboxSrc, err)
	}
	defer func() { _ = rc.Close() }()

	hostPath := filepath.Join(hostDir, filepath.Base(req.SandboxSrc))
	f, err := createDownloadFile(hostPath)
	if err != nil {
		return DownloadResult{}, fmt.Errorf("create %s: %w", hostPath, err)
	}
	if _, err := io.Copy(f, rc); err != nil {
		_ = f.Close()
		return DownloadResult{}, fmt.Errorf("write %s: %w", hostPath, err)
	}
	if err := f.Close(); err != nil {
		return DownloadResult{}, fmt.Errorf("close %s: %w", hostPath, err)
	}
	return DownloadResult{HostPath: hostPath}, nil
}
