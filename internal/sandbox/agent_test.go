package sandbox

import (
	"context"
	"testing"

	"github.com/jbeshir/demesne/internal/agents"
	proxyopenai "github.com/jbeshir/demesne/internal/proxies/openai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEgressOrDefault(t *testing.T) {
	assert.Equal(t, EgressNone, egressOrDefault("", EgressNone))
	assert.Equal(t, EgressOpen, egressOrDefault("", EgressOpen))
	assert.Equal(t, EgressPackageManagers, egressOrDefault(EgressPackageManagers, EgressNone))
}

// vendorStubAgent is a minimal agents.Agent whose only meaningful method
// is ProxyVendor; the rest return zero values. It lets buildProxyConfig
// be tested in isolation from any concrete provider.
type vendorStubAgent struct{ vendor agents.ProxyVendor }

func (vendorStubAgent) Name() string                                       { return "stub" }
func (vendorStubAgent) EnsureImage(context.Context) (string, error)        { return "", nil }
func (vendorStubAgent) GenerateContext(agents.ContextParams) string        { return "" }
func (vendorStubAgent) WriteAgentConfig(string, agents.AgentConfig) error  { return nil }
func (vendorStubAgent) ContextFileName() string                            { return "" }
func (vendorStubAgent) ResultText([]byte) string                           { return "" }
func (vendorStubAgent) ResolveModel(string) (agents.ModelName, error)      { return "", nil }
func (vendorStubAgent) Command(string, agents.ModelName) []string          { return nil }
func (vendorStubAgent) EnvVars(string, agents.ModelName) map[string]string { return nil }
func (s vendorStubAgent) ProxyVendor() agents.ProxyVendor                  { return s.vendor }

// TestBuildProxyConfig verifies the runner routes each agent vendor to its
// matching sidecar credential proxy with the correct upstream credential,
// and that the agent-facing token is the per-sandbox fake token (never the
// real upstream credential). The unknown-vendor arm yields no proxy.
func TestBuildProxyConfig(t *testing.T) {
	codexAuth := proxyopenai.TokenSet{
		AccessToken:  "access-x",
		RefreshToken: "refresh-x",
		IDToken:      "idtok",
		AccountID:    "acct-x",
	}
	r := &Runner{cfg: Config{
		ClaudeCodeOAuthToken: "claude-upstream",
	}}
	const fakeToken = "demesne-agent-fake" //nolint:gosec // test fixture, not a real credential
	const results = "/host/results"

	t.Run("anthropic", func(t *testing.T) {
		cfg := r.buildProxyConfig(vendorStubAgent{vendor: agents.ProxyAnthropic}, fakeToken, results, proxyopenai.TokenSet{})
		require.NotNil(t, cfg.Anthropic)
		assert.Nil(t, cfg.Codex)
		assert.Equal(t, fakeToken, cfg.Anthropic.AgentToken)
		assert.Equal(t, "claude-upstream", cfg.Anthropic.UpstreamToken)
		assert.Equal(t, results, cfg.Anthropic.ResultsHost)
	})

	t.Run("openai", func(t *testing.T) {
		cfg := r.buildProxyConfig(vendorStubAgent{vendor: agents.ProxyOpenAI}, fakeToken, results, codexAuth)
		require.NotNil(t, cfg.Codex)
		assert.Nil(t, cfg.Anthropic)
		assert.Equal(t, fakeToken, cfg.Codex.AgentToken)
		assert.Equal(t, codexAuth, cfg.Codex.Tokens)
		assert.Equal(t, results, cfg.Codex.ResultsHost)
	})

	t.Run("unknown vendor yields no proxy", func(t *testing.T) {
		cfg := r.buildProxyConfig(vendorStubAgent{vendor: "nope"}, fakeToken, results, proxyopenai.TokenSet{})
		assert.Nil(t, cfg.Anthropic)
		assert.Nil(t, cfg.Codex)
	})
}
