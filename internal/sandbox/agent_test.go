package sandbox

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jbeshir/demesne/internal/agents"
	// Side-effect imports register codex + claude-code so
	// AvailableAgents()/availableAgentNames tests can resolve real
	// Models lists via the registry rather than hard-coding them.
	_ "github.com/jbeshir/demesne/internal/agents/anthropic"
	_ "github.com/jbeshir/demesne/internal/agents/codex"
	proxyopenai "github.com/jbeshir/demesne/internal/proxies/openai"
	"github.com/jbeshir/demesne/internal/sidecar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Shared case names for the availability tests — pulled out so both
// TestAvailableAgentNames and TestAvailableAgents can name the same
// credential combos without tripping goconst.
const (
	caseCodexOnly  = "only codex configured"
	caseClaudeOnly = "only claude configured"
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

// TestAvailableAgentNames covers all four credential combinations
// for the codex-first availability helper that both resolveDefaultAgent
// and AvailableAgents share. The neither-set case returns an empty
// slice (NOT a fallback to codex — that's resolveDefaultAgent's job).
func TestAvailableAgentNames(t *testing.T) {
	dir := t.TempDir()
	codexAuth := filepath.Join(dir, "auth.json")
	require.NoError(t, os.WriteFile(codexAuth, []byte("{}"), 0o600))
	missingAuth := filepath.Join(dir, "does-not-exist.json")
	const claudeTok = "tok"

	tests := []struct {
		name        string
		codexFile   string
		claudeToken string
		want        []string
	}{
		{"both configured codex-first", codexAuth, claudeTok, []string{agentNameCodex, agentNameClaudeCode}},
		{caseCodexOnly, codexAuth, "", []string{agentNameCodex}},
		{caseClaudeOnly, missingAuth, claudeTok, []string{agentNameClaudeCode}},
		{"neither configured returns empty", missingAuth, "", nil},
		{"empty paths and no token returns empty", "", "", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{CodexAuthFile: tt.codexFile, ClaudeCodeOAuthToken: tt.claudeToken}
			assert.Equal(t, tt.want, availableAgentNames(cfg))
		})
	}
}

// TestAvailableAgents covers all four credential combinations for the
// Runner method the server consumes to build the `agent` / `model`
// enums. Models come from each provider's registered Models() so the
// test stays in sync with the catalog without hard-coding aliases.
func TestAvailableAgents(t *testing.T) {
	dir := t.TempDir()
	codexAuth := filepath.Join(dir, "auth.json")
	require.NoError(t, os.WriteFile(codexAuth, []byte("{}"), 0o600))
	missingAuth := filepath.Join(dir, "does-not-exist.json")
	const claudeTok = "tok"

	codexAgent, err := agents.Lookup(agentNameCodex)
	require.NoError(t, err)
	claudeAgent, err := agents.Lookup(agentNameClaudeCode)
	require.NoError(t, err)
	modelStrings := func(a agents.Agent) []string {
		ms := a.Models()
		out := make([]string, len(ms))
		for i, m := range ms {
			out[i] = string(m)
		}
		return out
	}
	codexModels := modelStrings(codexAgent)
	claudeModels := modelStrings(claudeAgent)

	tests := []struct {
		name        string
		codexFile   string
		claudeToken string
		want        []AgentOption
	}{
		{
			name:        "both configured codex-first",
			codexFile:   codexAuth,
			claudeToken: claudeTok,
			want: []AgentOption{
				{Name: agentNameCodex, Models: codexModels},
				{Name: agentNameClaudeCode, Models: claudeModels},
			},
		},
		{
			name:      caseCodexOnly,
			codexFile: codexAuth,
			want:      []AgentOption{{Name: agentNameCodex, Models: codexModels}},
		},
		{
			name:        caseClaudeOnly,
			codexFile:   missingAuth,
			claudeToken: claudeTok,
			want:        []AgentOption{{Name: agentNameClaudeCode, Models: claudeModels}},
		},
		{
			name:      "neither configured returns empty",
			codexFile: missingAuth,
			want:      []AgentOption{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Runner{cfg: Config{CodexAuthFile: tt.codexFile, ClaudeCodeOAuthToken: tt.claudeToken}}
			assert.Equal(t, tt.want, r.AvailableAgents())
		})
	}
}

// TestResolveDefaultAgent verifies the credential-aware default-agent
// resolution: prefer codex when its auth file exists, fall back to
// claude-code only when claude-code is configured and codex is not,
// and prefer codex when neither (or both) are configured so the
// missing-auth error names the Codex setup path.
func TestResolveDefaultAgent(t *testing.T) {
	dir := t.TempDir()
	codexAuth := filepath.Join(dir, "auth.json")
	require.NoError(t, os.WriteFile(codexAuth, []byte("{}"), 0o600))
	missingAuth := filepath.Join(dir, "does-not-exist.json")
	const claudeTok = "tok"

	tests := []struct {
		name        string
		codexFile   string
		claudeToken string
		want        string
	}{
		{"both configured prefers codex", codexAuth, claudeTok, agentNameCodex},
		{"only claude configured", missingAuth, claudeTok, agentNameClaudeCode},
		{"only codex configured", codexAuth, "", agentNameCodex},
		{"neither configured falls through to codex", missingAuth, "", agentNameCodex},
		{"empty codex path with claude token", "", claudeTok, agentNameClaudeCode},
		{"empty codex path with no claude token", "", "", agentNameCodex},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{CodexAuthFile: tt.codexFile, ClaudeCodeOAuthToken: tt.claudeToken}
			assert.Equal(t, tt.want, resolveDefaultAgent(cfg))
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
func (vendorStubAgent) Models() []agents.ModelName                         { return nil }
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

func TestReadAgentStderr_Missing(t *testing.T) {
	dir := t.TempDir()
	result := readAgentStderr(dir)
	assert.Nil(t, result)
}

func TestReadAgentStderr_Present(t *testing.T) {
	dir := t.TempDir()
	want := []byte("some stderr output\n")
	require.NoError(t, os.WriteFile(dir+"/"+agentStderrBasename, want, 0o600))
	got := readAgentStderr(dir)
	assert.Equal(t, want, got)
}
