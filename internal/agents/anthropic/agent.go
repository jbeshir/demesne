package anthropic

import (
	"context"

	"github.com/jbeshir/demesne/internal/agents"
	proxyanthropic "github.com/jbeshir/demesne/internal/proxies/anthropic"
)

// AgentName is the caller-facing identifier for the Claude Code agent.
const AgentName = "claude-code"

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

func (claudeCodeAgent) GenerateContext(
	preamble, prompt, egress string,
	inputs []agents.InputInfo,
	mcpServers []agents.MCPServerInfo,
	previousJobs []string,
) string {
	return generateContext(preamble, prompt, egress, inputs, mcpServers, previousJobs)
}

func (claudeCodeAgent) WriteAgentConfig(configDir string, cfg agents.AgentConfig) error {
	if err := writeMCPConfig(configDir, cfg.MCPServers); err != nil {
		return err
	}
	return writeRetryScript(configDir)
}

func (claudeCodeAgent) ContextFileName() string { return "CLAUDE.md" }

func (claudeCodeAgent) ResolveModel(name string) (string, error) {
	return ResolveModel(name)
}

func (claudeCodeAgent) Command(prompt, model string) []string {
	return []string{
		// sh retryScriptPath claude: the wrapper relaunches claude on quota
		// exhaustion; everything after "claude" is the argv it runs (and
		// re-runs with --resume on a rate-limit reset). $1=claude.
		"sh", retryScriptPath, "claude",
		"-p", prompt,
		"--model", model,
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

func (claudeCodeAgent) EnvVars(oauthToken, model string) map[string]string {
	return map[string]string{
		"CLAUDE_CODE_OAUTH_TOKEN": oauthToken,
		"ANTHROPIC_BASE_URL":      proxyanthropic.ListenURL(),
		"ANTHROPIC_MODEL":         model,
		"IS_SANDBOX":              "1",
	}
}
