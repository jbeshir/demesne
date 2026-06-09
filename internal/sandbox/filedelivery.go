package sandbox

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ErrUnknownParent is returned when a Deliver/DeliveryDir call references
// a jobID with no registered spawn context.
var ErrUnknownParent = errors.New("file-gen delivery: parent sandbox not registered")

// ErrNoWorkspace is returned when the spawn context has no host workspace
// (e.g. research-style isolated children — no /workspace path is exposed).
var ErrNoWorkspace = errors.New("file-gen delivery: parent sandbox has no host workspace")

// sandboxGeneratedDir is the in-sandbox path file-gen output is exposed at.
const sandboxGeneratedDir = "/workspace/generated"

// DeliveryDir locates this parent sandbox's <workspaceHost>/generated dir
// and creates it if missing. Implements mcpproxy.FileDeliverer.
func (r *Runner) DeliveryDir(parentJobID string) (string, string, error) {
	c, ok := r.registry.Lookup(JobID(parentJobID))
	if !ok {
		return "", "", fmt.Errorf("%w: %q", ErrUnknownParent, parentJobID)
	}
	if c.workspaceHost == "" {
		return "", "", fmt.Errorf("%w: %q", ErrNoWorkspace, parentJobID)
	}
	hostDir := filepath.Join(c.workspaceHost, "generated")
	if err := os.MkdirAll(hostDir, 0o750); err != nil {
		return "", "", fmt.Errorf("create %s: %w", hostDir, err)
	}
	return hostDir, sandboxGeneratedDir, nil
}

// Deliver copies each host path into the delivery dir (skipping any path
// already inside it) and returns a host->sandbox path map. Implements
// mcpproxy.FileDeliverer.
//
// On basename collision the destination is suffixed -1, -2, … so each
// source maps to a unique sandbox path.
func (r *Runner) Deliver(parentJobID string, hostPaths []string) (map[string]string, error) {
	hostDir, _, err := r.DeliveryDir(parentJobID)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(hostPaths))
	used := map[string]struct{}{}
	for _, src := range hostPaths {
		src = filepath.Clean(src)
		if _, dup := out[src]; dup {
			continue
		}
		base := filepath.Base(src)
		if alreadyUnder(src, hostDir) {
			out[src] = filepath.Join(sandboxGeneratedDir, base)
			used[base] = struct{}{}
			continue
		}
		dstBase := uniqueBase(base, used)
		dst := filepath.Join(hostDir, dstBase)
		if err := copyFile(src, dst); err != nil {
			return nil, fmt.Errorf("copy %s -> %s: %w", src, dst, err)
		}
		used[dstBase] = struct{}{}
		out[src] = filepath.Join(sandboxGeneratedDir, dstBase)
	}
	return out, nil
}

// alreadyUnder reports whether src lies inside hostDir (after symlink-free
// path cleaning). Avoids re-copying paths the upstream already placed in
// the delivery dir.
func alreadyUnder(src, hostDir string) bool {
	rel, err := filepath.Rel(hostDir, src)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

// uniqueBase returns base, or base with a -N suffix inserted before the
// extension, picking the lowest N not already in `used`.
func uniqueBase(base string, used map[string]struct{}) string {
	if _, taken := used[base]; !taken {
		return base
	}
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	for i := 1; ; i++ {
		cand := fmt.Sprintf("%s-%d%s", stem, i, ext)
		if _, taken := used[cand]; !taken {
			return cand
		}
	}
}

// copyFile is a buffered file copy with deferred-close error capture.
func copyFile(src, dst string) (err error) {
	in, err := os.Open(src) //nolint:gosec // path supplied by trusted file-gen registry
	if err != nil {
		return err
	}
	defer func() {
		if cerr := in.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()
	out, err := os.Create(dst) //nolint:gosec // dst inside per-sandbox workspace
	if err != nil {
		return err
	}
	defer func() {
		if cerr := out.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()
	_, err = io.Copy(out, in)
	return err
}
