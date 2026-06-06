package server

import (
	"testing"

	"github.com/jbeshir/demesne/internal/sandbox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Model alias literals used in the schema-shape tests; pulled out so
// goconst doesn't flag the repetition.
const (
	modelGPT55     = "gpt-5.5"
	modelGPT54Mini = "gpt-5.4-mini"
)

// agentEnum reads the registered tool's `agent`-property enum out of
// the mcp server's tool registry. nil enum + ok=false means the enum
// option was omitted entirely (the "no agent configured" branch).
func toolEnumStrings(t *testing.T, s *Server, toolName, propName string) ([]string, bool) {
	t.Helper()
	tools := s.mcpServer.ListTools()
	st, ok := tools[toolName]
	require.True(t, ok, "tool %q not registered", toolName)
	prop, ok := st.Tool.InputSchema.Properties[propName].(map[string]any)
	require.True(t, ok, "property %q on tool %q missing or wrong type", propName, toolName)
	raw, ok := prop["enum"]
	if !ok {
		return nil, false
	}
	values, ok := raw.([]string)
	require.True(t, ok, "enum on %q.%q has type %T, want []string", toolName, propName, raw)
	return values, true
}

// toolDescription returns the description of one property on a registered tool.
func toolPropDescription(t *testing.T, s *Server, toolName, propName string) string {
	t.Helper()
	tools := s.mcpServer.ListTools()
	st, ok := tools[toolName]
	require.True(t, ok, "tool %q not registered", toolName)
	prop, ok := st.Tool.InputSchema.Properties[propName].(map[string]any)
	require.True(t, ok)
	desc, _ := prop["description"].(string)
	return desc
}

// TestAgentEnumReflectsAvailability covers the three meaningful
// credential combos (both / codex-only / neither) for both
// sandbox_agent and sandbox_research, asserting the registered tool's
// `agent` and `model` enums match what AvailableAgents reports.
func TestAgentEnumReflectsAvailability(t *testing.T) {
	tests := []struct {
		name              string
		available         []sandbox.AgentOption
		wantAgentEnum     []string
		wantAgentOmit     bool
		wantModelEnum     []string
		wantModelOmit     bool
		descContains      []string
		descOmits         []string
		modelDescCheck    []string
		modelDescNotCheck []string
	}{
		{
			name: "both configured codex-first",
			available: []sandbox.AgentOption{
				{Name: agentNameCodex, Models: []string{modelGPT55, modelGPT54Mini}},
				{Name: agentNameClaudeCode, Models: []string{"opus", "sonnet", "haiku"}},
			},
			wantAgentEnum:  []string{agentNameCodex, agentNameClaudeCode},
			wantModelEnum:  []string{modelGPT55, modelGPT54Mini, "opus", "sonnet", "haiku"},
			descContains:   []string{"`codex`", "`claude-code`", "defaults to `codex`"},
			modelDescCheck: []string{"claude-code uses", "codex uses the gpt-5.x family"},
		},
		{
			name:              "only codex configured single-value enum",
			available:         []sandbox.AgentOption{{Name: agentNameCodex, Models: []string{modelGPT55, modelGPT54Mini}}},
			wantAgentEnum:     []string{agentNameCodex},
			wantModelEnum:     []string{modelGPT55, modelGPT54Mini},
			descContains:      []string{"`codex`", "only configured provider"},
			descOmits:         []string{"claude-code", "defaults to"},
			modelDescCheck:    []string{"codex uses the gpt-5.x family"},
			modelDescNotCheck: []string{"claude-code uses"},
		},
		{
			name:          "neither configured omits enum",
			available:     []sandbox.AgentOption{},
			wantAgentOmit: true,
			wantModelOmit: true,
			descContains:  []string{"No agent credentials are configured", "DEMESNE_CODEX_AUTH_FILE", "DEMESNE_CLAUDE_CODE_OAUTH_TOKEN"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewServer(&fakeRunner{available: tt.available})
			for _, toolName := range []string{sandbox.ToolSandboxAgent, sandbox.ToolSandboxResearch} {
				agentEnum, hasAgentEnum := toolEnumStrings(t, s, toolName, paramAgent)
				modelEnum, hasModelEnum := toolEnumStrings(t, s, toolName, paramModel)
				if tt.wantAgentOmit {
					assert.False(t, hasAgentEnum, "%s: agent enum should be omitted", toolName)
				} else {
					assert.True(t, hasAgentEnum, "%s: agent enum should be present", toolName)
					assert.Equal(t, tt.wantAgentEnum, agentEnum, "%s: agent enum", toolName)
				}
				if tt.wantModelOmit {
					assert.False(t, hasModelEnum, "%s: model enum should be omitted", toolName)
				} else {
					assert.True(t, hasModelEnum, "%s: model enum should be present", toolName)
					assert.Equal(t, tt.wantModelEnum, modelEnum, "%s: model enum", toolName)
				}
				agentDesc := toolPropDescription(t, s, toolName, paramAgent)
				for _, s := range tt.descContains {
					assert.Contains(t, agentDesc, s, "%s: agent description missing %q", toolName, s)
				}
				for _, s := range tt.descOmits {
					assert.NotContains(t, agentDesc, s, "%s: agent description should omit %q", toolName, s)
				}
				if len(tt.modelDescCheck) > 0 || len(tt.modelDescNotCheck) > 0 {
					modelDesc := toolPropDescription(t, s, toolName, paramModel)
					for _, s := range tt.modelDescCheck {
						assert.Contains(t, modelDesc, s, "%s: model description missing %q", toolName, s)
					}
					for _, s := range tt.modelDescNotCheck {
						assert.NotContains(t, modelDesc, s, "%s: model description should omit %q", toolName, s)
					}
				}
			}
		})
	}
}
