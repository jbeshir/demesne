package sandbox

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/jbeshir/demesne/internal/mcpproxy"
	proxyopenai "github.com/jbeshir/demesne/internal/proxies/openai"
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

	// CodexAuth is the ChatGPT OAuth token set read from the Codex auth file
	// (default ~/.codex/auth.json, overridden by DEMESNE_CODEX_AUTH_FILE).
	// It is passed to the per-sandbox sidecar proxy, which holds and refreshes
	// it autonomously. Required only when sandbox_agent/sandbox_research is
	// invoked with agent="codex"; an absent auth file leaves this zero.
	CodexAuth proxyopenai.TokenSet

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

// loadCodexAuth resolves and parses the Codex auth file (DEMESNE_CODEX_AUTH_FILE
// or ~/.codex/auth.json). An absent file returns a zero TokenSet without error;
// a present but malformed file returns an error.
func loadCodexAuth() (proxyopenai.TokenSet, error) {
	authFile := os.Getenv("DEMESNE_CODEX_AUTH_FILE")
	if authFile == "" {
		if home, err := os.UserHomeDir(); err == nil {
			authFile = home + "/.codex/auth.json"
		}
	}
	if authFile == "" {
		return proxyopenai.TokenSet{}, nil
	}
	data, err := os.ReadFile(authFile) //nolint:gosec // path from env or user home
	if errors.Is(err, fs.ErrNotExist) {
		// An absent file is not an error — Codex is simply unusable for this run.
		return proxyopenai.TokenSet{}, nil
	}
	if err != nil {
		return proxyopenai.TokenSet{}, fmt.Errorf("read Codex auth file %s: %w", authFile, err)
	}
	ts, parseErr := proxyopenai.ParseAuthJSON(data)
	if parseErr != nil {
		return proxyopenai.TokenSet{}, fmt.Errorf("parse Codex auth file %s: %w", authFile, parseErr)
	}
	return ts, nil
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

	codexAuth, err := loadCodexAuth()
	if err != nil {
		return Config{}, err
	}
	cfg.CodexAuth = codexAuth

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
