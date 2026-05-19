package anthropic

import (
	"fmt"
	"strings"

	"github.com/jbeshir/demesne/internal/agents"
)

// generateContext composes the CLAUDE.md content the agent reads from
// /in/CLAUDE.md (symlinked to ./CLAUDE.md in cwd).
//
// Layout: caller-supplied preamble (verbatim), then an auto-generated
// "Environment" section, then "Task" with the prompt. The egress
// argument controls the wording of the outbound-network sentence
// (one of "none", "package-managers", "open"; empty is treated as
// "none"). When egress is "open" we also add a long-running-research
// framing note so the model knows to flush incremental notes to /out.
func generateContext(preamble, prompt, egress string, inputs []agents.InputInfo) string {
	var b strings.Builder
	if preamble != "" {
		b.WriteString(strings.TrimSpace(preamble))
		b.WriteString("\n\n")
	}

	b.WriteString("## Environment\n\n")
	b.WriteString("You are running inside a demesne-managed sandbox.\n\n")
	b.WriteString("- `IS_SANDBOX=1` is set; long-running side effects and prompts for " +
		"user input have no recipient.\n")
	b.WriteString("- `/workspace` is your working directory and the only writable " +
		"scratch area. Copy any input you need to mutate into `/workspace` " +
		"first; do not try to modify `/in`.\n")
	b.WriteString("- `/out` is writable but **output only** — write your final " +
		"artefacts (results, reports, generated files) here. The caller reads " +
		"this back from the host after you exit.\n")
	if len(inputs) > 0 {
		b.WriteString("- Read-only inputs under `/in/`:\n")
		for _, in := range inputs {
			kind := "file"
			if in.IsDir {
				kind = "dir"
			}
			size := ""
			if !in.IsDir && in.Size >= 0 {
				size = fmt.Sprintf(" (%d bytes)", in.Size)
			}
			fmt.Fprintf(&b, "    - `/in/%s` — %s%s\n", in.Basename, kind, size)
		}
	} else {
		b.WriteString("- No caller-supplied inputs were mounted under `/in/`.\n")
	}
	b.WriteString("- " + egressDescription(egress) + "\n")
	if egress == "open" {
		b.WriteString("- **This is a long-running research task.** Cumulative " +
			"Anthropic spend is capped; if the cap is reached the proxy returns " +
			"402 and you will exit. Flush partial findings to `/out` as you go " +
			"so progress survives interruption.\n")
	}

	b.WriteString("\n## Task\n\n")
	b.WriteString(strings.TrimSpace(prompt))
	b.WriteString("\n")
	return b.String()
}

// egressDescription returns the human-readable sentence describing
// what the sandbox can reach over the network, given the egress mode
// string. Unknown values fall back to the strictest case so we never
// promise more reachability than the policy allows.
func egressDescription(egress string) string {
	switch egress {
	case "open":
		return "Outbound network access is unrestricted — you can reach any " +
			"HTTPS endpoint on the open internet."
	case "package-managers":
		return "Outbound network access is restricted to the Anthropic API " +
			"backing this CLI and the standard npm/PyPI/conda package registries."
	default:
		return "Outbound network access is restricted to the Anthropic API " +
			"backing this CLI; nothing else is reachable."
	}
}
