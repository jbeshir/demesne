package agentcommon

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

// ImageBuilder lazily builds and caches a provider's container image.
// The tag is a sha256 prefix of Dockerfile so a Dockerfile change forces
// a rebuild. Safe for concurrent first-callers. Each provider holds its
// own *ImageBuilder, so the sync state never crosses vendors.
type ImageBuilder struct {
	Repo       string // local image name, e.g. "demesne-claude-code"
	TmpPrefix  string // os.MkdirTemp pattern, e.g. "demesne-anthropic-build-*"
	Dockerfile []byte // embedded Dockerfile contents
	mu         sync.Mutex
	once       sync.Once
	tag        string
}

// Ensure builds the provider image if it isn't already present in the local
// Docker daemon. Safe for concurrent first-callers — a sync.Mutex serialises
// the build; subsequent calls fall through to a fast cache check.
func (b *ImageBuilder) Ensure(ctx context.Context) (string, error) {
	tag := b.imageTag()
	ref := b.Repo + ":" + tag

	if b.imagePresent(ctx, ref) {
		return ref, nil
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	if b.imagePresent(ctx, ref) {
		return ref, nil
	}

	dir, err := os.MkdirTemp("", b.TmpPrefix)
	if err != nil {
		return "", fmt.Errorf("create build context: %w", err)
	}
	defer func() { _ = os.RemoveAll(dir) }()
	if err := os.WriteFile(filepath.Join(dir, "Dockerfile"), b.Dockerfile, 0o600); err != nil {
		return "", fmt.Errorf("write Dockerfile: %w", err)
	}

	// ref derived from embed hash, dir from MkdirTemp.
	build := dockerCommand(ctx, "build", "-t", ref, dir)
	output, err := build.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("docker build %s: %w\n%s", ref, err, output)
	}
	return ref, nil
}

func (b *ImageBuilder) imageTag() string {
	b.once.Do(func() {
		sum := sha256.Sum256(b.Dockerfile)
		b.tag = hex.EncodeToString(sum[:])[:12]
	})
	return b.tag
}

func (b *ImageBuilder) imagePresent(ctx context.Context, ref string) bool {
	cmd := dockerCommand(ctx, "image", "inspect", ref)
	return cmd.Run() == nil
}

// dockerCommand wraps exec.CommandContext("docker", ...) so the gosec
// G204 suppression for agent-image build/inspect lives here. argv only;
// constant subcommands; ref is derived from a sha256 of the embedded
// Dockerfile (no shell, no user input).
func dockerCommand(ctx context.Context, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, "docker", args...) //nolint:gosec // argv-only; subcommand const, ref from embed hash
}
