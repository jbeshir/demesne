package sandbox

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jbeshir/demesne/internal/mcpproxy"
)

// Config holds the settings the sandbox runner needs. Most are
// environment-derived (see LoadConfigFromEnv); the MCP fields are
// populated by main after the host MCP aggregator starts.
type Config struct {
	// AllowedPaths is the colon-separated list of host paths under which
	// callers are permitted to mount files or directories.
	AllowedPaths []string

	// OutputRoot is the host directory under which per-job /out directories
	// are created.
	OutputRoot string

	// OpenSandboxDomain is the host:port (or full URL) of the OpenSandbox
	// lifecycle server.
	OpenSandboxDomain string

	// OpenSandboxProtocol is "http" or "https".
	OpenSandboxProtocol string

	// OpenSandboxAPIKey authenticates the lifecycle requests.
	OpenSandboxAPIKey string

	// ClaudeCodeOAuthToken is the long-lived token from `claude setup-token`,
	// injected into agent containers as CLAUDE_CODE_OAUTH_TOKEN. Required
	// only when sandbox_agent is invoked.
	ClaudeCodeOAuthToken string

	// CodexAuthFile is the path to the Codex auth.json (from
	// DEMESNE_CODEX_AUTH_FILE, default ~/.codex/auth.json). The runner
	// reads, refreshes, and persists it per launch so the rotating
	// single-use refresh token stays current and co-tenanted with the
	// host's own codex process. Empty when neither the env var is set nor
	// the home directory is resolvable.
	CodexAuthFile string

	// MCPServers are the host MCP aggregator's exposed server names
	// (sorted). Empty when no host MCP servers are available. Populated
	// by main after the aggregator starts, not from env.
	MCPServers []string

	// MCPSocketPath is the host filesystem path of the aggregator's
	// unix socket. The runner bind-mounts it into each sandbox sidecar,
	// where the MCP tunnel forwards to it.
	MCPSocketPath string

	// MCPToolCatalogue maps each exposed server to its allowlisted tool
	// list, used to populate the agent's CLAUDE.md and MCP config.
	// Populated alongside MCPServers.
	MCPToolCatalogue mcpproxy.ToolCatalogue

	// Owner is the per-instance identity computed at startup and written
	// into each sandbox's metadata as demesne.owner. Format:
	// "boot_id_pid_starttime" — unique across reboots and PID reuse.
	// Populated by main via ComputeOwner(); not env-derived.
	Owner string
}

// resolveCodexAuthFile returns the path to the Codex auth.json from
// DEMESNE_CODEX_AUTH_FILE, falling back to ~/.codex/auth.json, or ""
// when the home directory cannot be determined. It does not read or
// parse the file.
func resolveCodexAuthFile() string {
	if v := os.Getenv("DEMESNE_CODEX_AUTH_FILE"); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home + "/.codex/auth.json"
}

// defaultOutputRoot returns the runtime default for DEMESNE_OUTPUT_ROOT:
// ~/.demesne/out under the user's home, falling back to /tmp/demesne/out
// only when the home directory cannot be resolved (e.g. no HOME set).
// A home-based default keeps the output root private to the running user
// instead of dropping it in world-readable /tmp.
func defaultOutputRoot() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "/tmp/demesne/out"
	}
	return filepath.Join(home, ".demesne", "out")
}

// LoadConfigFromEnv reads required configuration from the process environment.
// DEMESNE_ALLOWED_PATHS and OPEN_SANDBOX_DOMAIN / OPEN_SANDBOX_API_KEY are mandatory; MCPServers
// and Owner are not env-derived and must be set after construction.
func LoadConfigFromEnv() (Config, error) {
	cfg := Config{
		OutputRoot:           envOr("DEMESNE_OUTPUT_ROOT", defaultOutputRoot()),
		OpenSandboxDomain:    os.Getenv("OPEN_SANDBOX_DOMAIN"),
		OpenSandboxProtocol:  envOr("OPEN_SANDBOX_PROTOCOL", "http"),
		OpenSandboxAPIKey:    os.Getenv("OPEN_SANDBOX_API_KEY"),
		ClaudeCodeOAuthToken: os.Getenv("DEMESNE_CLAUDE_CODE_OAUTH_TOKEN"),
	}

	cfg.CodexAuthFile = resolveCodexAuthFile()

	rawAllowed := os.Getenv("DEMESNE_ALLOWED_PATHS")
	if rawAllowed == "" {
		return Config{}, errors.New(
			"DEMESNE_ALLOWED_PATHS is required " +
				"(colon-separated list of host paths permitted as mount sources)",
		)
	}
	for _, p := range strings.Split(rawAllowed, ":") {
		p = strings.TrimSpace(p)
		if p != "" {
			cfg.AllowedPaths = append(cfg.AllowedPaths, p)
		}
	}
	if len(cfg.AllowedPaths) == 0 {
		return Config{}, errors.New("DEMESNE_ALLOWED_PATHS must contain at least one path")
	}

	// Always include the output root in the effective allowlist so /out
	// mounts and nested /in/previous-jobs/<name> mounts (which live under
	// OutputRoot) are mountable without the user having to enumerate the
	// path. Configured paths come first; the output root is appended last.
	cleanedOutputRoot := filepath.Clean(cfg.OutputRoot)
	alreadyPresent := false
	for _, p := range cfg.AllowedPaths {
		if filepath.Clean(p) == cleanedOutputRoot {
			alreadyPresent = true
			break
		}
	}
	if !alreadyPresent {
		cfg.AllowedPaths = append(cfg.AllowedPaths, cfg.OutputRoot)
	}

	if cfg.OpenSandboxDomain == "" {
		return Config{}, errors.New("OPEN_SANDBOX_DOMAIN is required (e.g. localhost:8080)")
	}
	if cfg.OpenSandboxAPIKey == "" {
		return Config{}, errors.New("OPEN_SANDBOX_API_KEY is required")
	}

	if err := os.MkdirAll(cfg.OutputRoot, 0o750); err != nil {
		return Config{}, fmt.Errorf("create output root %s: %w", cfg.OutputRoot, err)
	}

	return cfg, nil
}

func envOr(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}
