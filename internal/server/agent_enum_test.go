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
	modelGPT56Sol   = "gpt-5.6-sol"
	modelGPT56Terra = "gpt-5.6-terra"
	modelGPT56Luna  = "gpt-5.6-luna"
	modelGPT55      = "gpt-5.5"
	modelGPT54Mini  = "gpt-5.4-mini"
)

// toolEnumStrings reads a registered tool's property enum from the mcp
// server's tool registry. nil enum + ok=false means the enum option was
// omitted entirely (the "no agent configured" branch).
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

// toolPropDescription returns the description of one property on a registered tool.
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

// toolDescription returns the tool-level description for a registered tool.
func toolDescription(t *testing.T, s *Server, toolName string) string {
	t.Helper()
	tools := s.mcpServer.ListTools()
	st, ok := tools[toolName]
	require.True(t, ok, "tool %q not registered", toolName)
	return st.Tool.Description
}

// TestModelEnumReflectsAvailability covers the three meaningful
// credential combos (both / codex-only / neither) for both
// sandbox_agent and sandbox_research, asserting the registered tool's
// `model` enum and description match what AvailableAgents reports.
func TestModelEnumReflectsAvailability(t *testing.T) {
	tests := []struct {
		name              string
		available         []sandbox.AgentOption
		wantModelEnum     []string
		wantModelOmit     bool
		modelDescCheck    []string
		modelDescNotCheck []string
	}{
		{
			name: "both configured codex-first",
			available: []sandbox.AgentOption{
				{Name: agentNameCodex, Models: []string{modelGPT56Sol, modelGPT56Terra, modelGPT56Luna, modelGPT55, modelGPT54Mini}},
				{Name: agentNameClaudeCode, Models: []string{"sonnet", "opus", "fable", "haiku"}},
			},
			wantModelEnum:  []string{modelGPT56Sol, modelGPT56Terra, modelGPT56Luna, modelGPT55, modelGPT54Mini, "sonnet", "opus", "fable", "haiku"},
			modelDescCheck: []string{"claude-code uses", "gpt-5.6-sol", "gpt-5.6-terra", "gpt-5.6-luna", "gpt-5.5", "gpt-5.4-mini", "inferred"},
		},
		{
			name:              "only codex configured single-value enum",
			available:         []sandbox.AgentOption{{Name: agentNameCodex, Models: []string{modelGPT56Sol, modelGPT56Terra, modelGPT56Luna, modelGPT55, modelGPT54Mini}}},
			wantModelEnum:     []string{modelGPT56Sol, modelGPT56Terra, modelGPT56Luna, modelGPT55, modelGPT54Mini},
			modelDescCheck:    []string{"gpt-5.6-sol", "gpt-5.6-terra", "gpt-5.6-luna", "gpt-5.5", "gpt-5.4-mini"},
			modelDescNotCheck: []string{"claude-code uses"},
		},
		{
			name:           "neither configured omits enum",
			available:      []sandbox.AgentOption{},
			wantModelOmit:  true,
			modelDescCheck: []string{"No agent credentials are configured"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewServer(&fakeRunner{available: tt.available})
			for _, toolName := range []string{sandbox.ToolSandboxAgent, sandbox.ToolSandboxResearch} {
				modelEnum, hasModelEnum := toolEnumStrings(t, s, toolName, paramModel)
				if tt.wantModelOmit {
					assert.False(t, hasModelEnum, "%s: model enum should be omitted", toolName)
				} else {
					assert.True(t, hasModelEnum, "%s: model enum should be present", toolName)
					assert.Equal(t, tt.wantModelEnum, modelEnum, "%s: model enum", toolName)
				}
				modelDesc := toolPropDescription(t, s, toolName, paramModel)
				for _, s := range tt.modelDescCheck {
					assert.Contains(t, modelDesc, s, "%s: model description missing %q", toolName, s)
				}
				for _, s := range tt.modelDescNotCheck {
					assert.NotContains(t, modelDesc, s, "%s: model description should omit %q", toolName, s)
				}
			}
		})
	}
}

// mountToolsWithFilesDirs is the set of tools whose `files`/`directories`
// param descriptions are populated from AllowedMountPaths.
var mountToolsWithFilesDirs = []string{
	sandbox.ToolSandboxScript,
	sandbox.ToolSandboxCreate,
	sandbox.ToolSandboxAgent,
}

// TestMountPathDescriptionsReflectAllowlist asserts that the
// `files`/`directories` param descriptions on the three mount-accepting
// tools, plus `sandbox_upload`'s `src` and tool-level description, are
// populated from the Runner's AllowedMountPaths: configured roots are
// listed verbatim, and an empty allowlist names DEMESNE_ALLOWED_PATHS
// in its no-host-inputs wording. Mirrors TestModelEnumReflectsAvailability.
func TestMountPathDescriptionsReflectAllowlist(t *testing.T) {
	const pathFoo = "/srv/foo"
	const pathBar = "/srv/bar"
	const emptyMarker = "No host inputs can be mounted"
	const envVarMarker = "DEMESNE_ALLOWED_PATHS"

	tests := []struct {
		name         string
		allowedPaths []string
		descContains []string
		descOmits    []string
	}{
		{
			name:         "paths configured listed verbatim",
			allowedPaths: []string{pathFoo, pathBar},
			descContains: []string{"`" + pathFoo + "`", "`" + pathBar + "`", "configured mount roots"},
			descOmits:    []string{emptyMarker},
		},
		{
			name:         "no paths configured names env var",
			allowedPaths: nil,
			descContains: []string{emptyMarker, envVarMarker},
			descOmits:    []string{"configured mount roots"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewServer(&fakeRunner{allowedPaths: tt.allowedPaths})
			for _, toolName := range mountToolsWithFilesDirs {
				for _, prop := range []string{paramFiles, paramDirectories} {
					desc := toolPropDescription(t, s, toolName, prop)
					for _, want := range tt.descContains {
						assert.Contains(t, desc, want,
							"%s.%s: description missing %q", toolName, prop, want)
					}
					for _, omit := range tt.descOmits {
						assert.NotContains(t, desc, omit,
							"%s.%s: description should omit %q", toolName, prop, omit)
					}
				}
			}
			srcDesc := toolPropDescription(t, s, sandbox.ToolSandboxUpload, paramSrc)
			toolDesc := toolDescription(t, s, sandbox.ToolSandboxUpload)
			for _, want := range tt.descContains {
				assert.Contains(t, srcDesc, want, "upload src description missing %q", want)
				assert.Contains(t, toolDesc, want, "upload tool description missing %q", want)
			}
			for _, omit := range tt.descOmits {
				assert.NotContains(t, srcDesc, omit, "upload src description should omit %q", omit)
				assert.NotContains(t, toolDesc, omit, "upload tool description should omit %q", omit)
			}
		})
	}
}
