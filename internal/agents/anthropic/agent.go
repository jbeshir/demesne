package anthropic

import (
	"context"

	"github.com/jbeshir/demesne/internal/agents"
	proxyanthropic "github.com/jbeshir/demesne/internal/proxies/anthropic"
)

// AgentName is the caller-facing identifier for the Claude Code agent.
const AgentName = "claude-code"

// Environment variable names injected into the agent container. These
// are what Claude Code reads to reach the in-sidecar proxy and to
// authenticate. Defined as constants so a name change is compile-checked.
const (
	envOAuthToken = "CLAUDE_CODE_OAUTH_TOKEN" //nolint:gosec // env var name, not a credential value
	envBaseURL    = "ANTHROPIC_BASE_URL"
	envModel      = "ANTHROPIC_MODEL"
	envIsSandbox  = "IS_SANDBOX"
)

// claudeCodeAgent implements agents.Agent for the Claude Code CLI.
type claudeCodeAgent struct{}

// New returns the Claude Code agent provider.
func New() agents.Agent {
	return claudeCodeAgent{}
}

func init() {
	agents.Register(AgentName, New())
}

func (claudeCodeAgent) Name() string { return AgentName }

func (claudeCodeAgent) EnsureImage(ctx context.Context) (string, error) {
	return ensureImage(ctx)
}

func (claudeCodeAgent) GenerateContext(p agents.ContextParams) string {
	return generateContext(p)
}

func (claudeCodeAgent) WriteAgentConfig(configDir string, cfg agents.AgentConfig) error {
	if err := writeMCPConfig(configDir, cfg.MCPServers); err != nil {
		return err
	}
	return writeRetryScript(configDir)
}

func (claudeCodeAgent) ContextFileName() string { return "CLAUDE.md" }

func (claudeCodeAgent) ResolveModel(name string) (ModelName, error) {
	return ResolveModel(name)
}

func (claudeCodeAgent) Command(prompt string, model ModelName) []string {
	return []string{
		// sh retryScriptPath claude: the wrapper relaunches claude on quota
		// exhaustion; everything after "claude" is the argv it runs (and
		// re-runs with --resume on a rate-limit reset). $1=claude.
		"sh", retryScriptPath, "claude",
		"-p", prompt,
		"--model", string(model),
		// stream-json emits the full NDJSON event stream (messages, tool
		// calls, the final result) on stdout. The runner redirects that
		// to /out so the structured transcript streams to the host live;
		// ResultText parses the final answer back out. --verbose is
		// mandatory: `--print --output-format=stream-json requires
		// --verbose`.
		"--output-format", "stream-json",
		"--verbose",
		"--dangerously-skip-permissions",
		// WriteAgentConfig always writes this file (empty mcpServers
		// when no host servers are wired in); --strict-mcp-config makes
		// it the sole source of MCP servers, so the sandbox never picks
		// up unrelated config.
		"--mcp-config", mcpConfigPath,
		"--strict-mcp-config",
	}
}

func (claudeCodeAgent) EnvVars(oauthToken string, model ModelName) map[string]string {
	return map[string]string{
		envOAuthToken: oauthToken,
		envBaseURL:    proxyanthropic.ListenURL(),
		envModel:      string(model),
		envIsSandbox:  "1",
	}
}
