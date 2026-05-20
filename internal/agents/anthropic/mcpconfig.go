package anthropic

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jbeshir/demesne/internal/agents"
)

// mcpConfigBasename is the file WriteAgentConfig drops in the
// workspace. The runner mounts the workspace at /workspace, so the
// in-sandbox path is mcpConfigPath; the claude command references it
// via --mcp-config.
const mcpConfigBasename = ".demesne-mcp.json"

// mcpConfigPath is the in-sandbox absolute path of the MCP config
// file (workspace is the cwd and mount point).
const mcpConfigPath = "/workspace/" + mcpConfigBasename

// mcpConfigFile is the JSON shape Claude Code's --mcp-config reads:
// a map of server name → HTTP transport descriptor.
type mcpConfigFile struct {
	MCPServers map[string]mcpHTTPServer `json:"mcpServers"`
}

type mcpHTTPServer struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

// writeMCPConfig writes the Claude Code MCP config into workspaceDir.
// It is written unconditionally (an empty mcpServers map when no host
// servers are wired in) so the claude command can always pass
// --mcp-config --strict-mcp-config and fully control which MCP
// servers the agent sees.
func writeMCPConfig(workspaceDir string, servers []agents.MCPServerInfo) error {
	cfg := mcpConfigFile{MCPServers: make(map[string]mcpHTTPServer, len(servers))}
	for _, s := range servers {
		cfg.MCPServers[s.Name] = mcpHTTPServer{Type: "http", URL: s.URL}
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal mcp config: %w", err)
	}
	path := filepath.Join(workspaceDir, mcpConfigBasename)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
