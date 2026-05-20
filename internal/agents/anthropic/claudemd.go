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
// "Environment" section, then optionally "Available host tools", then
// "Task" with the prompt. The egress argument controls the wording of
// the outbound-network sentence (one of "none", "package-managers",
// "open"; empty is treated as "none"). When egress is "open" we also
// add a long-running-research framing note so the model knows to flush
// incremental notes to /out. mcpServers, when non-empty, are listed
// under their native tool names so the model knows what host tools it
// can call.
func generateContext(
	preamble, prompt, egress string,
	inputs []agents.InputInfo,
	mcpServers []agents.MCPServerInfo,
) string {
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
		b.WriteString("- **This is a long-running research task.** Flush " +
			"partial findings to `/out` as you go so progress survives " +
			"interruption.\n")
	}

	writeHostTools(&b, mcpServers)

	b.WriteString("\n## Task\n\n")
	b.WriteString(strings.TrimSpace(prompt))
	b.WriteString("\n")
	return b.String()
}

// writeHostTools appends the "Available host tools" section listing
// each server's allowlisted tools under their native names. No-op
// when no MCP servers are wired in.
func writeHostTools(b *strings.Builder, mcpServers []agents.MCPServerInfo) {
	if len(mcpServers) == 0 {
		return
	}
	b.WriteString("\n## Available host tools\n\n")
	b.WriteString("These read-only MCP servers from the host are wired into this " +
		"run. Call their tools directly by name:\n\n")
	for _, s := range mcpServers {
		fmt.Fprintf(b, "- **%s**:\n", s.Name)
		for _, t := range s.Tools {
			if t.Description != "" {
				fmt.Fprintf(b, "    - `%s` — %s\n", t.Name, t.Description)
			} else {
				fmt.Fprintf(b, "    - `%s`\n", t.Name)
			}
		}
	}
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
