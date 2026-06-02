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
// "Environment" section, then optionally "Available host tools", then
// "Orchestrating child agents", then "Task" with the prompt.
func generateContext(p agents.ContextParams) string {
	var b strings.Builder
	agentcommon.WritePreamble(&b, p.Preamble)
	agentcommon.WriteEnvironment(&b, p, agentcommon.EgressSentence(p.Egress, "the OpenAI API"))
	agentcommon.WriteHostTools(&b, p.MCPServers)
	agentcommon.WriteOrchestration(&b)
	agentcommon.WriteTask(&b, p.Prompt)
	return b.String()
}
