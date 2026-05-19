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

func (claudeCodeAgent) GenerateContext(preamble, prompt, egress string, inputs []agents.InputInfo) string {
	return generateContext(preamble, prompt, egress, inputs)
}

func (claudeCodeAgent) ContextFileName() string { return "CLAUDE.md" }

func (claudeCodeAgent) ResolveModel(name string) (string, error) {
	return ResolveModel(name)
}

func (claudeCodeAgent) Command(prompt, model string) []string {
	return []string{
		"claude",
		"-p", prompt,
		"--model", model,
		"--output-format", "text",
		"--dangerously-skip-permissions",
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
