package sidecar

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

// ImageRepo is the local image name for the demesne sidecar. The tag is
// a sha256 prefix of the embedded Dockerfile + binary so any change
// forces a rebuild on the next agent invocation.
const ImageRepo = "demesne-sidecar"

var (
	imageBuildMu sync.Mutex
	imageTagOnce string
)

// EnsureImage builds the sidecar image if it isn't present in the local
// Docker daemon and returns the fully-qualified ref (e.g.
// "demesne-sidecar:abcdef123456"). Safe for concurrent first-callers —
// a sync.Mutex serialises the build.
func EnsureImage(ctx context.Context) (string, error) {
	if len(sidecarBinary) == 0 {
		return "", errors.New("sidecar binary is empty: run `make sidecar-binary` before building demesne-mcp")
	}
	tag := imageTag()
	ref := ImageRepo + ":" + tag

	if imagePresent(ctx, ref) {
		return ref, nil
	}

	imageBuildMu.Lock()
	defer imageBuildMu.Unlock()
	if imagePresent(ctx, ref) {
		return ref, nil
	}

	dir, err := os.MkdirTemp("", "demesne-sidecar-build-*")
	if err != nil {
		return "", fmt.Errorf("create build context: %w", err)
	}
	defer func() { _ = os.RemoveAll(dir) }()
	if err := os.WriteFile(filepath.Join(dir, "Dockerfile"), dockerfileBytes, 0o600); err != nil {
		return "", fmt.Errorf("write Dockerfile: %w", err)
	}
	binPath := filepath.Join(dir, "demesne-sidecar")
	if err := os.WriteFile(binPath, sidecarBinary, 0o600); err != nil {
		return "", fmt.Errorf("write sidecar binary: %w", err)
	}
	// Make the binary executable so the Docker COPY preserves the mode.
	if err := os.Chmod(binPath, 0o755); err != nil { //nolint:gosec // build artefact, intentional exec bit
		return "", fmt.Errorf("chmod sidecar binary: %w", err)
	}

	build := exec.CommandContext(ctx, dockerCmd, "build", "-t", ref, dir) //nolint:gosec // ref derived from embed hash
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
	h := sha256.New()
	h.Write(dockerfileBytes)
	h.Write(sidecarBinary)
	imageTagOnce = hex.EncodeToString(h.Sum(nil))[:12]
	return imageTagOnce
}

func imagePresent(ctx context.Context, ref string) bool {
	cmd := exec.CommandContext(ctx, dockerCmd, "image", "inspect", ref) //nolint:gosec // ref derived from embed hash
	return cmd.Run() == nil
}
