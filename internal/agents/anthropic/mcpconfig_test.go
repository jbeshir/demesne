package anthropic

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/jbeshir/demesne/internal/agents"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteMCPConfig_HTTPServerParity(t *testing.T) {
	dir := t.TempDir()
	servers := []agents.MCPServerInfo{{Name: "demesne", URL: "http://127.0.0.1:9000/mcp"}, {Name: "fixture", URL: "http://127.0.0.1:9001/mcp"}}
	require.NoError(t, writeMCPConfig(dir, servers))
	data, err := os.ReadFile(filepath.Join(dir, mcpConfigBasename)) // #nosec G304 -- test-only path from t.TempDir()
	require.NoError(t, err)
	var got map[string]map[string]map[string]string
	require.NoError(t, json.Unmarshal(data, &got))
	require.Equal(t, map[string]map[string]map[string]string{"mcpServers": {"demesne": {"type": mcpServerTypeHTTP, "url": "http://127.0.0.1:9000/mcp"}, "fixture": {"type": mcpServerTypeHTTP, "url": "http://127.0.0.1:9001/mcp"}}}, got)
	// Claude's strict HTTP configuration has no equivalent verified timeout key.
	assert.NotContains(t, string(data), "timeout")
}
