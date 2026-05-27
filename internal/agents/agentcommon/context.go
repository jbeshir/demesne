package agentcommon

import (
	"fmt"
	"strings"

	"github.com/jbeshir/demesne/internal/agents"
	"github.com/jbeshir/demesne/internal/egress"
)

// WritePreamble appends the caller-supplied preamble section; no-op when empty.
func WritePreamble(b *strings.Builder, preamble string) {
	if preamble != "" {
		b.WriteString(strings.TrimSpace(preamble))
		b.WriteString("\n\n")
	}
}

// WriteEnvironment appends the full "## Environment" section with sandbox context.
func WriteEnvironment(b *strings.Builder, p agents.ContextParams, egressSentence string) {
	b.WriteString("## Environment\n\n")
	b.WriteString("You are running inside a demesne-managed sandbox.\n\n")
	b.WriteString("- `IS_SANDBOX=1` is set; long-running side effects and prompts for " +
		"user input have no recipient.\n")
	b.WriteString("- Your working directory is a private subdirectory of `/workspace`. " +
		"`/workspace` itself is shared writable scratch — if you spawn child " +
		"agents (see below) they share the same `/workspace`, so coordinate via " +
		"absolute `/workspace/...` paths. Copy any input you need to mutate into " +
		"`/workspace` first; do not try to modify `/in`.\n")
	b.WriteString("- `/out` is writable but **output only** — write your final " +
		"artefacts (results, reports, generated files) here. The caller reads " +
		"this back from the host after you exit.\n")
	if len(p.Inputs) > 0 {
		b.WriteString("- Read-only inputs under `/in/`:\n")
		for _, in := range p.Inputs {
			kind := "file"
			if in.IsDir {
				kind = "dir"
			}
			size := ""
			if !in.IsDir && in.Size >= 0 {
				size = fmt.Sprintf(" (%d bytes)", in.Size)
			}
			fmt.Fprintf(b, "    - `/in/%s` — %s%s\n", in.Basename, kind, size)
		}
	} else {
		b.WriteString("- No caller-supplied inputs were mounted under `/in/`.\n")
	}
	if len(p.PreviousJobs) > 0 {
		b.WriteString("- Completed sibling jobs' outputs are mounted read-only under " +
			"`/in/previous-jobs/<name>` — read earlier siblings' results there.\n")
	}
	b.WriteString("- " + egressSentence + "\n")
	if p.Egress == egress.Open {
		b.WriteString("- **This is a long-running research task.** Flush " +
			"partial findings to `/out` as you go so progress survives " +
			"interruption.\n")
	}
}

// WriteHostTools appends the "Available host tools" section listing each server's tools.
// No-op when no MCP servers are wired in.
func WriteHostTools(b *strings.Builder, mcpServers []agents.MCPServerInfo) {
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

// WriteOrchestration appends guidance for agents that spawn child sandboxes.
func WriteOrchestration(b *strings.Builder) {
	b.WriteString("\n## Orchestrating child agents\n\n")
	b.WriteString("You can spawn child sandboxes via the `demesne` MCP server " +
		"(`sandbox_agent`, `sandbox_research`, `sandbox_script`, and " +
		"`sandbox_create`/`sandbox_exec`/`sandbox_destroy`). Children inherit your " +
		"`/in` and share your `/workspace`; each child's output is at " +
		"`/out/child/<name>`, and completed siblings' outputs are mounted read-only " +
		"at `/in/previous-jobs/<name>`. " +
		"(Exception: `sandbox_research` children get a fresh private workspace " +
		"with no `/in` mounts — they do NOT inherit `/in` or share `/workspace`.)\n\n")
	b.WriteString("- **Delivering results is your job, not a child's.** " +
		"`/out/child/<name>` is that child's own output dir — files a child writes " +
		"there do NOT appear in your `/out`. To hand a child-produced or `/workspace` " +
		"artefact back to the caller, copy it into your own `/out` yourself with plain " +
		"`cp` (that needs no toolchain). Never delegate the copy-into-`/out` step to a " +
		"`sandbox_script` child — it would just write to its own `/out/child/<name>`.\n")
	b.WriteString("- **Validate with real builds/tests.** To compile, test, or lint " +
		"code, spawn a `sandbox_script` child — or a persistent " +
		"`sandbox_create`+`sandbox_exec` sandbox for repeated runs — with the " +
		"appropriate image (`node`, `python`, `go`, or `anaconda`), run it against " +
		"the shared `/workspace`, read the result, and iterate. Go modules resolve " +
		"automatically via `GOPROXY` (no egress change needed); for npm/PyPI/conda " +
		"set `egress: package-managers`.\n")
	b.WriteString("- **Preserve a baseline for review.** If you copy a repo from " +
		"`/in` into `/workspace` to edit it, copy it whole — including `.git` — so " +
		"review phases can `git diff` your changes against the original.\n")
	b.WriteString("- **Plan and enforce the handoff.** Before implementing in phases, " +
		"decide what each phase produces, where, and in what format — appropriate to " +
		"your task — and follow that contract strictly across every phase.\n")
}

// WriteTask appends the "## Task" section with the caller-supplied prompt.
func WriteTask(b *strings.Builder, prompt string) {
	b.WriteString("\n## Task\n\n")
	b.WriteString(strings.TrimSpace(prompt))
	b.WriteString("\n")
}
