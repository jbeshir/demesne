package sandbox

import (
	"errors"
	"fmt"
	"os"
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
}

// LoadConfigFromEnv reads required configuration from environment variables.
func LoadConfigFromEnv() (Config, error) {
	cfg := Config{
		OutputRoot:           envOr("DEMESNE_OUTPUT_ROOT", "/tmp/demesne/out"),
		OpenSandboxDomain:    os.Getenv("OPEN_SANDBOX_DOMAIN"),
		OpenSandboxProtocol:  envOr("OPEN_SANDBOX_PROTOCOL", "http"),
		OpenSandboxAPIKey:    os.Getenv("OPEN_SANDBOX_API_KEY"),
		ClaudeCodeOAuthToken: os.Getenv("DEMESNE_CLAUDE_CODE_OAUTH_TOKEN"),
	}

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
