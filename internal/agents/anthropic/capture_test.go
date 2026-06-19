package anthropic

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPostRunCapture_IsNonempty is a lightweight assertion that the
// returned snippet is the expected constant (non-empty, contains key tokens).
func TestPostRunCapture_IsNonempty(t *testing.T) {
	snip := claudeCodeAgent{}.PostRunCapture()
	assert.NotEmpty(t, snip)
	assert.Contains(t, snip, ".claude/projects")
	assert.Contains(t, snip, ".demesne-attrib")
}

// TestPostRunCapture_CopiesProjectsTree runs the snippet in a temp dir
// with a fake $HOME/.claude/projects tree and asserts the files are
// copied into .demesne-attrib.
func TestPostRunCapture_CopiesProjectsTree(t *testing.T) {
	requireSh(t)

	// Build a fake $HOME with a .claude/projects tree
	fakeHome := t.TempDir()
	projDir := filepath.Join(fakeHome, ".claude", "projects", "my-session")
	require.NoError(t, os.MkdirAll(projDir, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(projDir, "session.jsonl"), []byte(`{"type":"user"}`+"\n"), 0o600))

	// Run the snippet from a fresh cwd (simulating the agent's private subdir)
	cwd := t.TempDir()
	cmd := exec.CommandContext(context.Background(), "sh", "-c", claudeCodeCaptureSnippet)
	cmd.Dir = cwd
	cmd.Env = append(os.Environ(), "HOME="+fakeHome)

	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "snippet must exit 0; output: %s", out)

	// The snippet copies $HOME/.claude/projects into .demesne-attrib
	copied := filepath.Join(cwd, ".demesne-attrib", "my-session", "session.jsonl")
	data, err := os.ReadFile(copied) //nolint:gosec // known test path
	require.NoError(t, err, ".demesne-attrib should contain the copied session file")
	assert.Contains(t, string(data), `"type":"user"`)
}

// TestPostRunCapture_NoopWhenAbsent verifies the snippet is a no-op (exit 0,
// no .demesne-attrib created) when $HOME/.claude/projects does not exist.
func TestPostRunCapture_NoopWhenAbsent(t *testing.T) {
	requireSh(t)

	fakeHome := t.TempDir() // empty — no .claude/projects dir
	cwd := t.TempDir()

	cmd := exec.CommandContext(context.Background(), "sh", "-c", claudeCodeCaptureSnippet)
	cmd.Dir = cwd
	cmd.Env = append(os.Environ(), "HOME="+fakeHome)

	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "snippet must exit 0 when dir absent; output: %s", out)

	_, statErr := os.Stat(filepath.Join(cwd, ".demesne-attrib"))
	assert.True(t, os.IsNotExist(statErr), ".demesne-attrib must not be created when projects dir is absent")
}
