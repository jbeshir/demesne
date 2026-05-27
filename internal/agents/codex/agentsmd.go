package codex

import (
	"strings"

	"github.com/jbeshir/demesne/internal/agents"
	"github.com/jbeshir/demesne/internal/agents/agentcommon"
	"github.com/jbeshir/demesne/internal/egress"
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
	agentcommon.WriteEnvironment(&b, p, egressSentence(p.Egress))
	agentcommon.WriteHostTools(&b, p.MCPServers)
	agentcommon.WriteOrchestration(&b)
	agentcommon.WriteTask(&b, p.Prompt)
	return b.String()
}

// egressSentence returns the human-readable sentence describing
// what the sandbox can reach over the network, given the egress mode.
// Unknown values fall back to the strictest case so we never
// promise more reachability than the policy allows.
func egressSentence(mode egress.Mode) string {
	switch mode {
	case egress.Open:
		return "Outbound network access is unrestricted — you can reach any " +
			"HTTPS endpoint on the open internet."
	case egress.PackageManagers:
		return "Outbound network access is restricted to the OpenAI API " +
			"backing this CLI and the standard npm/PyPI/conda package registries."
	case egress.None:
		return "Outbound network access is restricted to the OpenAI API " +
			"backing this CLI; nothing else is reachable."
	default:
		// Unknown values fall back to the strictest case so we never
		// promise more reachability than the policy allows.
		return "Outbound network access is restricted to the OpenAI API " +
			"backing this CLI; nothing else is reachable."
	}
}
