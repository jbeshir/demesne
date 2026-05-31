package sidecar

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const fakeDocker = `#!/bin/sh
printf '1\n' >> "$SCENARIO_DIR/calls"
calls=$(wc -l < "$SCENARIO_DIR/calls" 2>/dev/null || echo 0)

case "$DEMESNE_TEST_SCENARIO" in
  happy)
    exit 0
    ;;
  transient_then_ok)
    if [ "$calls" -le 1 ]; then
      printf 'Error: rootless netns: kill network process: permission denied\n' >&2
      exit 1
    fi
    exit 0
    ;;
  no_such_container)
    printf 'Error: no such container: demesne-sidecar-xxx\n' >&2
    exit 1
    ;;
  transient_persistent)
    printf 'Error: permission denied\n' >&2
    exit 1
    ;;
  fatal)
    printf 'Error: invalid reference format\n' >&2
    exit 1
    ;;
  *)
    printf 'unknown scenario: %s\n' "$DEMESNE_TEST_SCENARIO" >&2
    exit 2
    ;;
esac
`

func requireSh(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh unavailable")
	}
}

func writeFakeDocker(t *testing.T) string {
	t.Helper()
	binDir := t.TempDir()
	p := filepath.Join(binDir, "docker")
	require.NoError(t, os.WriteFile(p, []byte(fakeDocker), 0o755)) //nolint:gosec
	return p
}

func readCallCount(t *testing.T, scenarioDir string) int {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(scenarioDir, "calls")) //nolint:gosec // scenarioDir is t.TempDir() in tests
	if err != nil {
		return 0
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	count := 0
	for _, l := range lines {
		if l != "" {
			count++
		}
	}
	return count
}

func overrideExec(t *testing.T, fakeDockerPath, scenario, scenarioDir string) {
	t.Helper()
	prev := execCommand
	t.Cleanup(func() { execCommand = prev })
	execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		cmd := exec.CommandContext(ctx, fakeDockerPath, args...) //nolint:gosec // fakeDockerPath is t.TempDir() in tests
		cmd.Env = append(os.Environ(),
			"DEMESNE_TEST_SCENARIO="+scenario,
			"SCENARIO_DIR="+scenarioDir,
		)
		return cmd
	}
}

func TestRemove_Happy(t *testing.T) {
	requireSh(t)
	fakeDockerPath := writeFakeDocker(t)
	scenarioDir := t.TempDir()
	overrideExec(t, fakeDockerPath, "happy", scenarioDir)

	err := Remove(context.Background(), "abc123")
	require.NoError(t, err)
	assert.Equal(t, 1, readCallCount(t, scenarioDir))
}

func TestRemove_TransientThenOK(t *testing.T) {
	requireSh(t)
	fakeDockerPath := writeFakeDocker(t)
	scenarioDir := t.TempDir()
	overrideExec(t, fakeDockerPath, "transient_then_ok", scenarioDir)

	err := Remove(context.Background(), "abc123")
	require.NoError(t, err)
	assert.Equal(t, 2, readCallCount(t, scenarioDir))
}

func TestRemove_NoSuchContainerIdempotent(t *testing.T) {
	requireSh(t)
	fakeDockerPath := writeFakeDocker(t)
	scenarioDir := t.TempDir()
	overrideExec(t, fakeDockerPath, "no_such_container", scenarioDir)

	err := Remove(context.Background(), "abc123")
	require.NoError(t, err)
	assert.Equal(t, 1, readCallCount(t, scenarioDir))
}

func TestRemove_TransientPersistent(t *testing.T) {
	requireSh(t)
	fakeDockerPath := writeFakeDocker(t)
	scenarioDir := t.TempDir()
	overrideExec(t, fakeDockerPath, "transient_persistent", scenarioDir)

	err := Remove(context.Background(), "abc123")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "permission denied")
	assert.Equal(t, 3, readCallCount(t, scenarioDir))
}

func TestRemove_FatalNonRetryable(t *testing.T) {
	requireSh(t)
	fakeDockerPath := writeFakeDocker(t)
	scenarioDir := t.TempDir()
	overrideExec(t, fakeDockerPath, "fatal", scenarioDir)

	err := Remove(context.Background(), "abc123")
	require.Error(t, err)
	assert.Equal(t, 1, readCallCount(t, scenarioDir))
}

func TestStop_Happy(t *testing.T) {
	requireSh(t)
	fakeDockerPath := writeFakeDocker(t)
	scenarioDir := t.TempDir()
	overrideExec(t, fakeDockerPath, "happy", scenarioDir)

	h := &Handle{ContainerID: "abc123"}
	err := h.Stop(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, readCallCount(t, scenarioDir))
}

func TestStop_NilHandleAndEmptyID(t *testing.T) {
	requireSh(t)
	fakeDockerPath := writeFakeDocker(t)
	scenarioDir := t.TempDir()
	overrideExec(t, fakeDockerPath, "happy", scenarioDir)

	var nilHandle *Handle
	require.NoError(t, nilHandle.Stop(context.Background()))

	emptyHandle := &Handle{}
	require.NoError(t, emptyHandle.Stop(context.Background()))

	assert.Equal(t, 0, readCallCount(t, scenarioDir))
}

func TestStop_TransientThenOK(t *testing.T) {
	requireSh(t)
	fakeDockerPath := writeFakeDocker(t)
	scenarioDir := t.TempDir()
	overrideExec(t, fakeDockerPath, "transient_then_ok", scenarioDir)

	h := &Handle{ContainerID: "abc123"}
	err := h.Stop(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 2, readCallCount(t, scenarioDir))
}

func TestRemove_ContextCancelledMidBackoff(t *testing.T) {
	requireSh(t)
	fakeDockerPath := writeFakeDocker(t)
	scenarioDir := t.TempDir()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	prev := execCommand
	defer func() { execCommand = prev }()
	execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		cmd := exec.CommandContext(ctx, fakeDockerPath, args...) //nolint:gosec // fakeDockerPath is t.TempDir() in tests
		cmd.Env = append(os.Environ(),
			"DEMESNE_TEST_SCENARIO=transient_persistent",
			"SCENARIO_DIR="+scenarioDir,
		)
		return cmd
	}

	done := make(chan error, 1)
	go func() {
		done <- Remove(ctx, "abc123")
	}()

	// Let first call complete (it's instant), then cancel during the 50ms backoff.
	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		require.Error(t, err)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Remove did not return promptly after context cancellation")
	}

	count := readCallCount(t, scenarioDir)
	assert.Less(t, count, 3, "expected fewer than 3 calls; got "+strconv.Itoa(count))
}

// TestVerifySidecarRunning covers the post-start liveness check: a "running"
// container passes; an exited one or a vanished one (inspect errors because
// --rm already removed it — the wrong-architecture failure mode) returns an
// error naming the linux/amd64 requirement.
func TestVerifySidecarRunning(t *testing.T) {
	requireSh(t)
	prevSettle := sidecarStartSettle
	sidecarStartSettle = 0 // don't actually wait in tests
	t.Cleanup(func() { sidecarStartSettle = prevSettle })

	// inspectStub makes execCommand return a command emitting the given stdout
	// and exit code, regardless of args.
	inspectStub := func(stdout string, exit int) func(context.Context, string, ...string) *exec.Cmd {
		return func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
			script := "printf '%s' " + stdout + "; exit " + strconv.Itoa(exit)
			return exec.CommandContext(ctx, "sh", "-c", script) //nolint:gosec // fixed test script
		}
	}

	tests := []struct {
		name    string
		stub    func(context.Context, string, ...string) *exec.Cmd
		wantErr bool
	}{
		{name: "running", stub: inspectStub("running", 0), wantErr: false},
		{name: "exited", stub: inspectStub("exited", 0), wantErr: true},
		{name: "created", stub: inspectStub("created", 0), wantErr: true},
		// inspect of an already-removed container exits non-zero (--rm reaped it).
		{name: "gone", stub: inspectStub("", 1), wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prev := execCommand
			t.Cleanup(func() { execCommand = prev })
			execCommand = tt.stub

			err := verifySidecarRunning(context.Background(), "cid123")
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "linux/amd64",
					"failure must name the architecture requirement")
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestVerifySidecarRunning_ContextCancelled confirms the settle wait honours
// context cancellation rather than blocking.
func TestVerifySidecarRunning_ContextCancelled(t *testing.T) {
	prevSettle := sidecarStartSettle
	sidecarStartSettle = time.Hour // long enough that only cancellation returns
	t.Cleanup(func() { sidecarStartSettle = prevSettle })

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := verifySidecarRunning(ctx, "cid123")
	require.ErrorIs(t, err, context.Canceled)
}
