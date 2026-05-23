package anthropic

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jbeshir/demesne/internal/agents"
)

// retryScriptBasename is the file WriteAgentConfig drops in the run's
// config dir. The runner mounts that dir read-only at
// agents.AgentConfigDir, so the in-sandbox path is retryScriptPath; the
// provider's Command runs claude through it.
const retryScriptBasename = "claude-retry.sh"

// retryScriptPath is the in-sandbox absolute path of the wrapper under
// the read-only config-dir mount.
const retryScriptPath = agents.AgentConfigDir + "/" + retryScriptBasename

// writeRetryScript writes the embedded retry wrapper into configDir.
// The provider's Command invokes it via `sh <retryScriptPath>`, so no
// exec bit is needed; 0o600 matches writeMCPConfig and the read-only
// config-dir mount the in-sandbox user reads it from.
func writeRetryScript(configDir string) error {
	path := filepath.Join(configDir, retryScriptBasename)
	if err := os.WriteFile(path, retryScriptBytes, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
