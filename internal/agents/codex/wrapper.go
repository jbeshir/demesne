package codex

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jbeshir/demesne/internal/agents"
)

// wrapperScriptBasename is the file WriteAgentConfig drops in the run's
// config dir. The runner mounts that dir read-only at
// agents.AgentConfigDir, so the in-sandbox path is wrapperScriptPath;
// the provider's Command runs codex through it via `sh <wrapperScriptPath>`.
const wrapperScriptBasename = "codex-exec.sh"

// wrapperScriptPath is the in-sandbox absolute path of the wrapper under
// the read-only config-dir mount.
const wrapperScriptPath = agents.AgentConfigDir + "/" + wrapperScriptBasename

// writeWrapperScript writes the embedded exec wrapper into configDir.
// The provider's Command invokes it via `sh <wrapperScriptPath>`, so no
// exec bit is needed; 0o600 matches writeCodexConfig and the read-only
// config-dir mount the in-sandbox user reads it from.
//
// The wrapper hardcodes /in/.agent/config.toml (== codexConfigPath)
// when copying into CODEX_HOME — keep the two constants in sync.
func writeWrapperScript(configDir string) error {
	path := filepath.Join(configDir, wrapperScriptBasename)
	if err := os.WriteFile(path, wrapperScriptBytes, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
