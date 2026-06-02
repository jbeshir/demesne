package anthropic

import (
	"strings"

	"github.com/jbeshir/demesne/internal/agents"
	"github.com/jbeshir/demesne/internal/agents/agentcommon"
)

// generateContext composes the CLAUDE.md content the agent reads from
// /in/CLAUDE.md (symlinked to ./CLAUDE.md in cwd).
//
// Layout: caller-supplied preamble (verbatim), then an auto-generated
// "Environment" section, then optionally "Available host tools", then
// "Orchestrating child agents", then "Task" with the prompt.
func generateContext(p agents.ContextParams) string {
	var b strings.Builder
	agentcommon.WritePreamble(&b, p.Preamble)
	agentcommon.WriteEnvironment(&b, p, agentcommon.EgressSentence(p.Egress, "the Anthropic API"))
	agentcommon.WriteHostTools(&b, p.MCPServers)
	agentcommon.WriteOrchestration(&b)
	agentcommon.WriteTask(&b, p.Prompt)
	return b.String()
}
