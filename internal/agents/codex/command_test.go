package codex

import (
	"testing"

	"github.com/jbeshir/demesne/internal/agents"
	"github.com/stretchr/testify/assert"
)

func TestCommand(t *testing.T) {
	got := codexAgent{}.Command("do it", ModelGPT56Sol)
	assert.Equal(t, []string{"sh", wrapperScriptPath, "gpt-5.6-sol", "do it"}, got)
	assert.Equal(t, agents.AgentConfigDir+"/codex-retry.sh", wrapperScriptPath)
}

func TestEnvVars_SetsAgentKeyAndSandbox(t *testing.T) {
	got := codexAgent{}.EnvVars("tok", ModelGPT56Sol)
	assert.Equal(t, "tok", got[envAgentKey])
	assert.Equal(t, "1", got[envIsSandbox])
}

func TestEnvVars_NoRealKeyOrProxyURL(t *testing.T) {
	// The agent must never receive the real OpenAI API key or the upstream URL.
	got := codexAgent{}.EnvVars("tok", ModelGPT56Sol)
	for k, v := range got {
		assert.NotContains(t, k, "OPENAI_API_KEY", "real key name must not appear in env var name")
		assert.NotContains(t, v, "api.openai.com", "upstream URL must not appear in env var value")
	}
}

func TestContextFileName(t *testing.T) {
	assert.Equal(t, "AGENTS.md", codexAgent{}.ContextFileName())
}

func TestName(t *testing.T) {
	assert.Equal(t, "codex", codexAgent{}.Name())
}

func TestProxyVendor(t *testing.T) {
	assert.Equal(t, agents.ProxyOpenAI, codexAgent{}.ProxyVendor())
}
