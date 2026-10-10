package anthropic

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jbeshir/demesne/internal/agents"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommand_InvokesWrapper(t *testing.T) {
	got := claudeCodeAgent{}.Command("my prompt", "opus")

	assert.Equal(t, "node", got[0])
	assert.Equal(t, retryScriptPath, got[1])
	assert.Equal(t, "claude", got[2])

	assert.Equal(t, []string{
		"claude",
		"-p", "my prompt",
		"--model", "opus",
		"--output-format", "stream-json",
		"--verbose",
		"--dangerously-skip-permissions",
		"--mcp-config", mcpConfigPath,
		"--strict-mcp-config",
	}, got[2:])

	assert.Equal(t, agents.AgentConfigDir+"/claude-retry.mjs", retryScriptPath)
}

func TestWriteAgentConfig_WritesScriptAndMCP(t *testing.T) {
	dir := t.TempDir()

	err := claudeCodeAgent{}.WriteAgentConfig(dir, agents.AgentConfig{})
	require.NoError(t, err)

	mcpPath := filepath.Join(dir, ".demesne-mcp.json")
	_, err = os.Stat(mcpPath)
	require.NoError(t, err, ".demesne-mcp.json should exist")

	scriptPath := filepath.Join(dir, "claude-retry.mjs")
	data, err := os.ReadFile(scriptPath) //nolint:gosec // path under t.TempDir()
	require.NoError(t, err, "claude-retry.mjs should exist")
	assert.Equal(t, retryScriptBytes, data)

	info, err := os.Stat(scriptPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestRetryScriptEmbedded(t *testing.T) {
	assert.NotEmpty(t, retryScriptBytes)
	assert.Contains(t, string(retryScriptBytes), "claude-retry: ")
}
