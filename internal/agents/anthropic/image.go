package anthropic

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

// imageRepo is the local image name; the tag is a sha256 prefix of the
// embedded Dockerfile so a Dockerfile change forces a rebuild.
const imageRepo = "demesne-claude-code"

var (
	imageBuildMu sync.Mutex
	imageTagOnce string
)

// ensureImage builds the claude-code image if it isn't already present in
// the local Docker daemon. Safe for concurrent first-callers — a sync.Mutex
// serialises the build; subsequent calls fall through to a fast cache check.
func ensureImage(ctx context.Context) (string, error) {
	tag := imageTag()
	ref := imageRepo + ":" + tag

	if imagePresent(ctx, ref) {
		return ref, nil
	}

	imageBuildMu.Lock()
	defer imageBuildMu.Unlock()
	if imagePresent(ctx, ref) {
		return ref, nil
	}

	dir, err := os.MkdirTemp("", "demesne-anthropic-build-*")
	if err != nil {
		return "", fmt.Errorf("create build context: %w", err)
	}
	defer func() { _ = os.RemoveAll(dir) }()
	if err := os.WriteFile(filepath.Join(dir, "Dockerfile"), dockerfileBytes, 0o600); err != nil {
		return "", fmt.Errorf("write Dockerfile: %w", err)
	}

	// ref derived from embed hash, dir from MkdirTemp.
	build := exec.CommandContext(ctx, "docker", "build", "-t", ref, dir) //nolint:gosec
	output, err := build.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("docker build %s: %w\n%s", ref, err, output)
	}
	return ref, nil
}

func imageTag() string {
	if imageTagOnce != "" {
		return imageTagOnce
	}
	sum := sha256.Sum256(dockerfileBytes)
	imageTagOnce = hex.EncodeToString(sum[:])[:12]
	return imageTagOnce
}

func imagePresent(ctx context.Context, ref string) bool {
	cmd := exec.CommandContext(ctx, "docker", "image", "inspect", ref) //nolint:gosec // ref derived from embed hash
	return cmd.Run() == nil
}
