package agentcommon

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"sync"
)

// ImageBuilder lazily builds and caches a provider's container image. The
// tag is a sha256 prefix of the Dockerfile plus any resolved build args, so
// a change to either forces a rebuild. Safe for concurrent first-callers.
// Each provider holds its own *ImageBuilder, so the sync state never crosses
// vendors.
type ImageBuilder struct {
	Repo       string // local image name, e.g. "demesne-claude-code"
	TmpPrefix  string // os.MkdirTemp pattern, e.g. "demesne-anthropic-build-*"
	Dockerfile []byte // embedded Dockerfile contents

	// BuildArgs, if set, is called at Ensure time to resolve --build-arg
	// values for the build (e.g. a host-detected tool version). The
	// resolved args are passed to `docker build` and folded into the image
	// tag, so a value change (e.g. the host upgrading a tool) forces an
	// automatic rebuild without any code change. Nil means no build args —
	// the tag is then a hash of the Dockerfile alone, unchanged from a
	// builder that never used build args.
	BuildArgs func(ctx context.Context) (map[string]string, error)

	mu sync.Mutex
}

// Ensure builds the provider image if it isn't already present in the local
// Docker daemon. Safe for concurrent first-callers — a sync.Mutex serialises
// the build; subsequent calls fall through to a fast cache check.
func (b *ImageBuilder) Ensure(ctx context.Context) (string, error) {
	args, err := b.resolveBuildArgs(ctx)
	if err != nil {
		return "", err
	}
	tag := imageTag(b.Dockerfile, args)
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

	// ref derived from embed hash + resolved args, dir from MkdirTemp.
	buildArgs := []string{"build"}
	for _, k := range sortedKeys(args) {
		buildArgs = append(buildArgs, "--build-arg", k+"="+args[k])
	}
	buildArgs = append(buildArgs, "-t", ref, dir)
	build := dockerCommand(ctx, buildArgs...)
	output, err := build.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("docker build %s: %w\n%s", ref, err, output)
	}
	return ref, nil
}

func (b *ImageBuilder) resolveBuildArgs(ctx context.Context) (map[string]string, error) {
	if b.BuildArgs == nil {
		return nil, nil
	}
	return b.BuildArgs(ctx)
}

// imageTag is a 12-hex-char sha256 prefix over the Dockerfile and the
// resolved build args. With no args it equals sha256(Dockerfile), so
// builders that don't use build args keep their previous tag.
func imageTag(dockerfile []byte, args map[string]string) string {
	h := sha256.New()
	h.Write(dockerfile)
	for _, k := range sortedKeys(args) {
		h.Write([]byte(k))
		h.Write([]byte{0})
		h.Write([]byte(args[k]))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))[:12]
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func (b *ImageBuilder) imagePresent(ctx context.Context, ref string) bool {
	cmd := dockerCommand(ctx, "image", "inspect", ref)
	return cmd.Run() == nil
}

// dockerCommand wraps exec.CommandContext("docker", ...) so the gosec
// G204 suppression for agent-image build/inspect lives here. argv only;
// constant subcommands; ref is derived from a sha256 of the embedded
// Dockerfile plus resolved build args (no shell, no user input).
func dockerCommand(ctx context.Context, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, "docker", args...) //nolint:gosec // argv-only; subcommand const, ref from embed hash
}
