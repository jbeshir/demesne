package sandbox

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestRunner(tmpDir string) (*Runner, string) {
	r := &Runner{registry: newChildRegistry()}
	r.registry.Register(JobID("job-x"), &spawnContext{workspaceHost: tmpDir})
	return r, "job-x"
}

func TestRunner_DeliveryDir_CreatesDir(t *testing.T) {
	tmpDir := t.TempDir()
	r, jobID := newTestRunner(tmpDir)

	hostDir, sandboxDir, err := r.DeliveryDir(jobID)

	require.NoError(t, err)
	assert.Equal(t, filepath.Join(tmpDir, "generated"), hostDir)
	assert.Equal(t, "/workspace/generated", sandboxDir)
	fi, statErr := os.Stat(hostDir)
	require.NoError(t, statErr)
	assert.True(t, fi.IsDir())
}

func TestRunner_DeliveryDir_UnknownParentReturnsErr(t *testing.T) {
	r := &Runner{registry: newChildRegistry()}

	_, _, err := r.DeliveryDir("nope")

	assert.ErrorIs(t, err, ErrUnknownParent)
}

func TestRunner_DeliveryDir_NoWorkspaceReturnsErr(t *testing.T) {
	r := &Runner{registry: newChildRegistry()}
	r.registry.Register(JobID("job-empty"), &spawnContext{workspaceHost: ""})

	_, _, err := r.DeliveryDir("job-empty")

	assert.ErrorIs(t, err, ErrNoWorkspace)
}

func TestRunner_Deliver_CopiesFileAndReturnsSandboxPath(t *testing.T) {
	tmpDir := t.TempDir()
	r, jobID := newTestRunner(tmpDir)

	srcDir := t.TempDir()
	srcPath := filepath.Join(srcDir, "hello.txt")
	require.NoError(t, os.WriteFile(srcPath, []byte("hello world"), 0o600))

	mapping, err := r.Deliver(jobID, []string{srcPath})

	require.NoError(t, err)
	assert.Equal(t, "/workspace/generated/hello.txt", mapping[srcPath])

	dstPath := filepath.Join(tmpDir, "generated", "hello.txt")
	dstContent, readErr := os.ReadFile(dstPath) //nolint:gosec // test fixture path under t.TempDir()
	require.NoError(t, readErr)
	srcContent, _ := os.ReadFile(srcPath) //nolint:gosec // test fixture path under t.TempDir()
	assert.Equal(t, srcContent, dstContent)
}

func TestRunner_Deliver_SkipsCopyWhenAlreadyInDeliveryDir(t *testing.T) {
	tmpDir := t.TempDir()
	r, jobID := newTestRunner(tmpDir)

	genDir := filepath.Join(tmpDir, "generated")
	require.NoError(t, os.MkdirAll(genDir, 0o750))
	existingPath := filepath.Join(genDir, "already.txt")
	require.NoError(t, os.WriteFile(existingPath, []byte("pre-existing"), 0o600))

	mapping, err := r.Deliver(jobID, []string{existingPath})

	require.NoError(t, err)
	assert.Equal(t, "/workspace/generated/already.txt", mapping[existingPath])

	entries, readErr := os.ReadDir(genDir)
	require.NoError(t, readErr)
	assert.Len(t, entries, 1)
}

func TestRunner_Deliver_BasenameCollisionGetsUniqueSuffix(t *testing.T) {
	tmpDir := t.TempDir()
	r, jobID := newTestRunner(tmpDir)

	dir1 := t.TempDir()
	dir2 := t.TempDir()
	src1 := filepath.Join(dir1, "img.png")
	src2 := filepath.Join(dir2, "img.png")
	require.NoError(t, os.WriteFile(src1, []byte("png1"), 0o600))
	require.NoError(t, os.WriteFile(src2, []byte("png2"), 0o600))

	mapping, err := r.Deliver(jobID, []string{src1, src2})

	require.NoError(t, err)

	genDir := filepath.Join(tmpDir, "generated")
	entries, readErr := os.ReadDir(genDir)
	require.NoError(t, readErr)
	assert.Len(t, entries, 2)

	assert.Equal(t, "/workspace/generated/img.png", mapping[src1])
	assert.Equal(t, "/workspace/generated/img-1.png", mapping[src2])

	_, err1 := os.Stat(filepath.Join(genDir, "img.png"))
	_, err2 := os.Stat(filepath.Join(genDir, "img-1.png"))
	assert.NoError(t, err1)
	assert.NoError(t, err2)
}
