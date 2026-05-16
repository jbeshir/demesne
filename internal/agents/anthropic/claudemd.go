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
// "Environment" section, then "Task" with the prompt.
func generateContext(preamble, prompt string, inputs []agents.InputInfo) string {
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
	b.WriteString("- Outbound network access is restricted to the Anthropic API " +
		"backing this CLI; nothing else is reachable.\n")

	b.WriteString("\n## Task\n\n")
	b.WriteString(strings.TrimSpace(prompt))
	b.WriteString("\n")
	return b.String()
}
