package sandbox

import (
	"context"
	"strings"
	"testing"

	opensandbox "github.com/alibaba/OpenSandbox/sdks/sandbox/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrepareSandbox_ZeroTimeout(t *testing.T) {
	outRoot := t.TempDir()
	r := NewRunner(Config{
		AllowedPaths: []string{outRoot},
		OutputRoot:   outRoot,
	})

	_, _, _, err := r.prepareSandbox(context.Background(), sandboxPrepOptions{
		Image:          DefaultImage,
		Egress:         EgressNone,
		TimeoutSeconds: 0,
		Tool:           ToolSandboxScript,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "TimeoutSeconds must be set")
}

// TestPrepareSandbox_HappyPath verifies that a successful prepareSandbox call
// creates the per-job output directory under OutputRoot and returns a non-empty
// job ID. It exercises the code path up to launchSandbox using the same test
// seam as runner_create_retry_test.go; the full SDK path is not tested here.
func TestPrepareSandbox_HappyPath(t *testing.T) {
	withCreateSandboxSeams(t)
	outRoot := t.TempDir()
	r := NewRunner(Config{
		AllowedPaths: []string{outRoot},
		OutputRoot:   outRoot,
	})
	createSandboxFn = func(
		_ context.Context,
		_ opensandbox.ConnectionConfig,
		_ opensandbox.SandboxCreateOptions,
	) (*opensandbox.Sandbox, error) {
		return (*opensandbox.Sandbox)(nil), nil
	}

	_, outputHost, jobID, err := r.prepareSandbox(context.Background(), sandboxPrepOptions{
		Image:          DefaultImage,
		Egress:         EgressNone,
		TimeoutSeconds: testTTL,
		Tool:           ToolSandboxScript,
	})
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(outputHost, outRoot),
		"outputHost %q should be under outRoot %q", outputHost, outRoot)
	assert.DirExists(t, outputHost)
	assert.NotEmpty(t, string(jobID))
}
