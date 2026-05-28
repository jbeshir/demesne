package sandbox

import (
	"context"
	"testing"

	"github.com/jbeshir/demesne/internal/agents"
	proxyopenai "github.com/jbeshir/demesne/internal/proxies/openai"
	"github.com/jbeshir/demesne/internal/sidecar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEgressOrDefault(t *testing.T) {
	tests := []struct {
		name string
		mode EgressMode
		def  EgressMode
		want EgressMode
	}{
		{
			name: "empty uses none default",
			def:  EgressNone,
			want: EgressNone,
		},
		{
			name: "empty uses open default",
			def:  EgressOpen,
			want: EgressOpen,
		},
		{
			name: "explicit package managers wins",
			mode: EgressPackageManagers,
			def:  EgressNone,
			want: EgressPackageManagers,
		},
		{
			name: "explicit open wins",
			mode: EgressOpen,
			def:  EgressNone,
			want: EgressOpen,
		},
		{
			name: "explicit none wins",
			mode: EgressNone,
			def:  EgressOpen,
			want: EgressNone,
		},
		{
			name: "unknown explicit value wins",
			mode: EgressMode("custom-egress"),
			def:  EgressNone,
			want: EgressMode("custom-egress"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, egressOrDefault(tt.mode, tt.def))
		})
	}
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

	tests := []struct {
		name        string
		vendor      agents.ProxyVendor
		codexTokens proxyopenai.TokenSet
		check       func(*testing.T, string, proxyopenai.TokenSet, sidecar.ProxyConfig)
	}{
		{
			name:   "anthropic",
			vendor: agents.ProxyAnthropic,
			check: func(t *testing.T, resultsHost string, _ proxyopenai.TokenSet, cfg sidecar.ProxyConfig) {
				t.Helper()
				require.NotNil(t, cfg.Anthropic)
				assert.Nil(t, cfg.Codex)
				assert.Equal(t, fakeToken, cfg.Anthropic.AgentToken)
				assert.Equal(t, "claude-upstream", cfg.Anthropic.UpstreamToken)
				assert.Equal(t, resultsHost, cfg.Anthropic.ResultsHost)
			},
		},
		{
			name:        "openai",
			vendor:      agents.ProxyOpenAI,
			codexTokens: codexAuth,
			check: func(t *testing.T, resultsHost string, tokens proxyopenai.TokenSet, cfg sidecar.ProxyConfig) {
				t.Helper()
				require.NotNil(t, cfg.Codex)
				assert.Nil(t, cfg.Anthropic)
				assert.Equal(t, fakeToken, cfg.Codex.AgentToken)
				assert.Equal(t, tokens, cfg.Codex.Tokens)
				assert.Equal(t, resultsHost, cfg.Codex.ResultsHost)
			},
		},
		{
			name:   "unknown vendor yields no proxy",
			vendor: "nope",
			check: func(t *testing.T, _ string, _ proxyopenai.TokenSet, cfg sidecar.ProxyConfig) {
				t.Helper()
				assert.Nil(t, cfg.Anthropic)
				assert.Nil(t, cfg.Codex)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := r.buildProxyConfig(vendorStubAgent{vendor: tt.vendor}, fakeToken, results, tt.codexTokens)
			tt.check(t, results, tt.codexTokens, cfg)
		})
	}
}
