package codex

import (
	"strings"

	"github.com/jbeshir/demesne/internal/agents"
	"github.com/jbeshir/demesne/internal/agents/agentcommon"
)

// generateContext composes the AGENTS.md content the agent reads from
// /in/AGENTS.md (symlinked to ./AGENTS.md in cwd).
//
// Layout: caller-supplied preamble (verbatim), then an auto-generated
// "Environment" section, then Codex-specific engineering guidance, then
// optionally "Available host tools", then "Orchestrating child agents", then
// "Task" with the prompt.
func generateContext(p agents.ContextParams) string {
	var b strings.Builder
	agentcommon.WritePreamble(&b, p.Preamble)
	agentcommon.WriteEnvironment(&b, p, agentcommon.EgressSentence(p.Egress, "the OpenAI API"))
	writeEngineeringGuidance(&b)
	agentcommon.WriteHostTools(&b, p.MCPServers)
	agentcommon.WriteFileGenNote(&b, p.MCPServers)
	agentcommon.WriteOrchestration(&b)
	agentcommon.WriteDefinitionOfDone(&b, p.OutputContract)
	agentcommon.WriteTask(&b, p.Prompt)
	return b.String()
}

// writeEngineeringGuidance appends guidance intentionally scoped to Codex.
func writeEngineeringGuidance(b *strings.Builder) {
	b.WriteString("\n## Engineering changes\n\n")
	b.WriteString("For internal packages, prefer clean, explicit migrations and update all callers. " +
		"Unless the user explicitly requires compatibility, do not preserve backward compatibility " +
		"with variadic optional parameters, nil/default shims, alternate constructors, or deprecated wrappers.\n")
}
