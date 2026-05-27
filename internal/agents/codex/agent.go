package codex

import (
	"context"

	"github.com/jbeshir/demesne/internal/agents"
)

// AgentName is the caller-facing identifier for the Codex agent.
const AgentName = "codex"

// Environment variable names injected into the agent container. These
// are what Codex reads to authenticate against the in-sidecar proxy.
// Defined as constants so a name change is compile-checked.
const (
	envAgentKey  = "DEMESNE_OPENAI_AGENT_KEY"
	envIsSandbox = "IS_SANDBOX"
)

// codexAgent implements agents.Agent for the OpenAI Codex CLI.
type codexAgent struct{}

// New returns the Codex agent provider.
func New() agents.Agent {
	return codexAgent{}
}

func init() {
	agents.Register(AgentName, New())
}

func (codexAgent) Name() string { return AgentName }

func (codexAgent) EnsureImage(ctx context.Context) (string, error) {
	return ensureImage(ctx)
}

func (codexAgent) GenerateContext(p agents.ContextParams) string {
	return generateContext(p)
}

func (codexAgent) WriteAgentConfig(configDir string, cfg agents.AgentConfig) error {
	if err := writeCodexConfig(configDir, cfg.MCPServers); err != nil {
		return err
	}
	return writeWrapperScript(configDir)
}

func (codexAgent) ContextFileName() string { return "AGENTS.md" }

func (codexAgent) ResolveModel(name string) (ModelName, error) {
	return ResolveModel(name)
}

func (codexAgent) ProxyVendor() agents.ProxyVendor { return agents.ProxyOpenAI }

func (codexAgent) Command(prompt string, model ModelName) []string {
	return []string{"sh", wrapperScriptPath, string(model), prompt}
}

func (codexAgent) EnvVars(agentToken string, _ ModelName) map[string]string {
	// The real OpenAI key never appears here; it lives only in the sidecar
	// proxy. The proxy URL is baked into config.toml's base_url via
	// writeCodexConfig — not passed via env — to keep the provider-knows-its-
	// own-proxy pattern consistent. CODEX_HOME is set by the wrapper script
	// to "$PWD/.codex" to avoid collisions between sibling runs sharing
	// /workspace.
	return map[string]string{
		envAgentKey:  agentToken,
		envIsSandbox: "1",
	}
}
