package sandbox

import (
	"context"
	"errors"
	"testing"
	"time"

	opensandbox "github.com/alibaba/OpenSandbox/sdks/sandbox/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Shared constants for test fixtures.
const (
	testJobID = JobID("test-job")
	testImage = ImageURI("anaconda")
	testTTL   = oneShotSandboxTTLSeconds
	testTool  = ToolSandboxScript
)

// withCreateSandboxSeams saves the three retry seams and restores them on
// t.Cleanup. It zeroes createSandboxBackoffs so tests run without sleeping.
// Callers assign createSandboxFn and dockerRemoveFn after calling this.
func withCreateSandboxSeams(t *testing.T) {
	t.Helper()
	prevFn, prevRemove, prevBackoffs := createSandboxFn, dockerRemoveFn, createSandboxBackoffs
	t.Cleanup(func() {
		createSandboxFn = prevFn
		dockerRemoveFn = prevRemove
		createSandboxBackoffs = prevBackoffs
	})
	createSandboxBackoffs = []time.Duration{0, 0}
}

// callResult pairs a sandbox pointer with an error for sequenced fakes.
type callResult struct {
	sb  *opensandbox.Sandbox
	err error
}

// transientErr makes an error containing both transient substrings and,
// optionally, a parseable container URL. Pass empty id to omit the URL.
func transientErr(id string) error {
	msg := createSandboxTransientCode + ": " + createSandboxTransientMessage
	if id != "" {
		msg += " 500 Server Error for http+docker://localhost/v1.41/containers/" +
			id + "/archive?path=%2F"
	}
	return errors.New(msg)
}

// launchTestSandbox calls launchSandbox with the shared test constants.
func launchTestSandbox(ctx context.Context, r *Runner) (*opensandbox.Sandbox, error) {
	return r.launchSandbox(ctx, testImage, nil, nil, testTTL, testJobID, testTool)
}

func TestLaunchSandbox_HappyPath(t *testing.T) {
	withCreateSandboxSeams(t)
	calls := 0
	removes := 0
	results := []callResult{{(*opensandbox.Sandbox)(nil), nil}}
	createSandboxFn = func(
		_ context.Context,
		_ opensandbox.ConnectionConfig,
		_ opensandbox.SandboxCreateOptions,
	) (*opensandbox.Sandbox, error) {
		r := results[calls]
		calls++
		return r.sb, r.err
	}
	dockerRemoveFn = func(_ context.Context, _ string) error { removes++; return nil }

	_, err := launchTestSandbox(context.Background(), NewRunner(Config{}))
	require.NoError(t, err)
	assert.Equal(t, 1, calls)
	assert.Equal(t, 0, removes)
}

func TestLaunchSandbox_TransientThenSuccess(t *testing.T) {
	withCreateSandboxSeams(t)
	calls := 0
	var removedIDs []string
	results := []callResult{
		{nil, transientErr("abc123def456")},
		{(*opensandbox.Sandbox)(nil), nil},
	}
	createSandboxFn = func(
		_ context.Context,
		_ opensandbox.ConnectionConfig,
		_ opensandbox.SandboxCreateOptions,
	) (*opensandbox.Sandbox, error) {
		r := results[calls]
		calls++
		return r.sb, r.err
	}
	dockerRemoveFn = func(_ context.Context, id string) error {
		removedIDs = append(removedIDs, id)
		return nil
	}

	_, err := launchTestSandbox(context.Background(), NewRunner(Config{}))
	require.NoError(t, err)
	assert.Equal(t, 2, calls)
	require.Len(t, removedIDs, 1)
	assert.Equal(t, "abc123def456", removedIDs[0])
}

func TestLaunchSandbox_PersistentTransient(t *testing.T) {
	withCreateSandboxSeams(t)
	calls := 0
	removes := 0
	results := []callResult{
		{nil, transientErr("aa11bb22cc33dd44")},
		{nil, transientErr("ee55ff667788aabb")},
		{nil, transientErr("ccdd0011eeff2233")},
	}
	createSandboxFn = func(
		_ context.Context,
		_ opensandbox.ConnectionConfig,
		_ opensandbox.SandboxCreateOptions,
	) (*opensandbox.Sandbox, error) {
		r := results[calls]
		calls++
		return r.sb, r.err
	}
	dockerRemoveFn = func(_ context.Context, _ string) error { removes++; return nil }

	_, err := launchTestSandbox(context.Background(), NewRunner(Config{}))
	require.Error(t, err)
	assert.Equal(t, 3, calls)
	assert.Equal(t, 2, removes)
	assert.Contains(t, err.Error(), "after 3 attempts")
	assert.Contains(t, err.Error(), createSandboxTransientCode)
}

func TestLaunchSandbox_NonTransient_NoRetry(t *testing.T) {
	withCreateSandboxSeams(t)
	calls := 0
	removes := 0
	createSandboxFn = func(
		_ context.Context,
		_ opensandbox.ConnectionConfig,
		_ opensandbox.SandboxCreateOptions,
	) (*opensandbox.Sandbox, error) {
		calls++
		return nil, errors.New("image not found")
	}
	dockerRemoveFn = func(_ context.Context, _ string) error { removes++; return nil }

	_, err := launchTestSandbox(context.Background(), NewRunner(Config{}))
	require.Error(t, err)
	assert.Equal(t, 1, calls)
	assert.Equal(t, 0, removes)
	assert.Contains(t, err.Error(), "image not found")
	assert.NotContains(t, err.Error(), "after")
}

func TestLaunchSandbox_TransientNoIDInError(t *testing.T) {
	withCreateSandboxSeams(t)
	calls := 0
	removes := 0
	results := []callResult{
		{nil, transientErr("")},
		{(*opensandbox.Sandbox)(nil), nil},
	}
	createSandboxFn = func(
		_ context.Context,
		_ opensandbox.ConnectionConfig,
		_ opensandbox.SandboxCreateOptions,
	) (*opensandbox.Sandbox, error) {
		r := results[calls]
		calls++
		return r.sb, r.err
	}
	dockerRemoveFn = func(_ context.Context, _ string) error { removes++; return nil }

	_, err := launchTestSandbox(context.Background(), NewRunner(Config{}))
	require.NoError(t, err)
	assert.Equal(t, 2, calls)
	assert.Equal(t, 0, removes)
}

func TestLaunchSandbox_CtxCancelledDuringBackoff(t *testing.T) {
	prevFn, prevRemove, prevBackoffs := createSandboxFn, dockerRemoveFn, createSandboxBackoffs
	t.Cleanup(func() {
		createSandboxFn = prevFn
		dockerRemoveFn = prevRemove
		createSandboxBackoffs = prevBackoffs
	})

	calls := 0
	createSandboxFn = func(
		_ context.Context,
		_ opensandbox.ConnectionConfig,
		_ opensandbox.SandboxCreateOptions,
	) (*opensandbox.Sandbox, error) {
		calls++
		return nil, transientErr("")
	}
	dockerRemoveFn = func(_ context.Context, _ string) error { return nil }
	createSandboxBackoffs = []time.Duration{200 * time.Millisecond, 200 * time.Millisecond}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		_, err := launchTestSandbox(ctx, NewRunner(Config{}))
		done <- err
	}()

	// Let the first call complete instantly, then cancel during the 200ms backoff.
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		require.Error(t, err)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("launchSandbox did not return promptly after context cancellation")
	}
	assert.Less(t, calls, 3, "expected fewer than 3 sandbox calls")
}

func TestIsCreateSandboxTransient(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "both substrings present",
			err:  errors.New(createSandboxTransientCode + " " + createSandboxTransientMessage),
			want: true,
		},
		{
			name: "only code present",
			err:  errors.New(createSandboxTransientCode),
			want: false,
		},
		{
			name: "only message present",
			err:  errors.New(createSandboxTransientMessage),
			want: false,
		},
		{
			name: "neither present",
			err:  errors.New("some unrelated error"),
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "both present with surrounding text and reversed order",
			err: errors.New("prefix " + createSandboxTransientMessage +
				" middle " + createSandboxTransientCode + " suffix"),
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isCreateSandboxTransient(tt.err))
		})
	}
}

func TestContainerIDFromError(t *testing.T) {
	const fullID = "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2"
	const urlPrefix = "500 Server Error for http+docker://localhost/v1.41/containers/"

	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "real URL pattern with hex ID",
			err:  errors.New(urlPrefix + "abc123def456/archive?path=%2F"),
			want: "abc123def456",
		},
		{
			name: "error with no URL",
			err:  errors.New("some other error without a container URL"),
			want: "",
		},
		{
			name: "URL with uppercase ID rejected",
			err:  errors.New(urlPrefix + "ABC123DEF456/archive?path=%2F"),
			want: "",
		},
		{
			name: "URL with too-short ID rejected",
			err:  errors.New(urlPrefix + "abc1234/archive?path=%2F"),
			want: "",
		},
		{
			name: "URL with 64-char hex ID accepted",
			err:  errors.New(urlPrefix + fullID + "/archive?path=%2F"),
			want: fullID,
		},
		{
			name: "nil error",
			err:  nil,
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, containerIDFromError(tt.err))
		})
	}
}
