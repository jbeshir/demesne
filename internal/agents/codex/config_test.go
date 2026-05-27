package codex

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jbeshir/demesne/internal/agents"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteCodexConfig_BasicFields(t *testing.T) {
	dir := t.TempDir()
	err := writeCodexConfig(dir, nil)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(dir, codexConfigBasename)) //nolint:gosec // path under t.TempDir()
	require.NoError(t, err)
	content := string(data)

	assert.Contains(t, content, `model_provider = "demesne"`)
	assert.Contains(t, content, `base_url = "http://127.0.0.1:8086/v1"`)
	assert.Contains(t, content, `wire_api = "responses"`)
	assert.Contains(t, content, `env_key = "DEMESNE_OPENAI_AGENT_KEY"`)
	assert.Contains(t, content, `approval_policy = "never"`)
	assert.Contains(t, content, `sandbox_mode = "danger-full-access"`)
	assert.Contains(t, content, `supports_websockets = false`)
}

func TestWriteCodexConfig_FilePermissions(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, writeCodexConfig(dir, nil))

	info, err := os.Stat(filepath.Join(dir, codexConfigBasename))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestWriteCodexConfig_NoServersOmitsMCPSection(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, writeCodexConfig(dir, nil))

	data, err := os.ReadFile(filepath.Join(dir, codexConfigBasename)) //nolint:gosec // path under t.TempDir()
	require.NoError(t, err)
	assert.NotContains(t, string(data), "[mcp_servers")
}

func TestWriteCodexConfig_WithServer(t *testing.T) {
	dir := t.TempDir()
	servers := []agents.MCPServerInfo{
		{Name: "testserver", URL: "http://127.0.0.1:9000/mcp"},
	}
	require.NoError(t, writeCodexConfig(dir, servers))

	data, err := os.ReadFile(filepath.Join(dir, codexConfigBasename)) //nolint:gosec // path under t.TempDir()
	require.NoError(t, err)
	content := string(data)

	assert.Contains(t, content, "[mcp_servers.testserver]")
	assert.Contains(t, content, `url = "http://127.0.0.1:9000/mcp"`)
}
