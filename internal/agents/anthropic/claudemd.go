package anthropic

import (
	"github.com/jbeshir/demesne/internal/agents"
	"github.com/jbeshir/demesne/internal/agents/agentcommon"
	"github.com/jbeshir/demesne/internal/egress"
)

// generateContext composes the CLAUDE.md content the agent reads from
// /in/CLAUDE.md (symlinked to ./CLAUDE.md in cwd).
//
// Layout: caller-supplied preamble (verbatim), then an auto-generated
// "Environment" section, then optionally "Available host tools", then
// "Task" with the prompt. The mode argument controls the wording of
// the outbound-network sentence (one of "none", "package-managers",
// "open"; empty is treated as "none"). When mode is egress.Open we also
// add a long-running-research framing note so the model knows to flush
// incremental notes to /out. mcpServers, when non-empty, are listed
// under their native tool names so the model knows what host tools it
// can call.
func generateContext(p agents.ContextParams) string {
	return agentcommon.GenerateContext(p, egressDescription)
}

// egressDescription returns the human-readable sentence describing
// what the sandbox can reach over the network, given the egress mode.
// Unknown values fall back to the strictest case so we never
// promise more reachability than the policy allows.
func egressDescription(mode egress.Mode) string {
	switch mode {
	case egress.Open:
		return "Outbound network access is unrestricted — you can reach any " +
			"HTTPS endpoint on the open internet."
	case egress.PackageManagers:
		return "Outbound network access is restricted to the Anthropic API " +
			"backing this CLI and the standard npm/PyPI/conda package registries."
	case egress.None:
		return "Outbound network access is restricted to the Anthropic API " +
			"backing this CLI; nothing else is reachable."
	default:
		// Unknown values fall back to the strictest case so we never
		// promise more reachability than the policy allows.
		return "Outbound network access is restricted to the Anthropic API " +
			"backing this CLI; nothing else is reachable."
	}
}
