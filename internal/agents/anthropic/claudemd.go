package anthropic

import (
	"fmt"
	"strings"

	"github.com/jbeshir/demesne/internal/agents"
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
func generateContext(
	preamble, prompt string,
	mode egress.Mode,
	inputs []agents.InputInfo,
	mcpServers []agents.MCPServerInfo,
	previousJobs []string,
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
	b.WriteString("- Your working directory is a private subdirectory of `/workspace`. " +
		"`/workspace` itself is shared writable scratch — if you spawn child " +
		"agents (see below) they share the same `/workspace`, so coordinate via " +
		"absolute `/workspace/...` paths. Copy any input you need to mutate into " +
		"`/workspace` first; do not try to modify `/in`.\n")
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
	if len(previousJobs) > 0 {
		b.WriteString("- Completed sibling jobs' outputs are mounted read-only under " +
			"`/in/previous-jobs/<name>` — read earlier siblings' results there.\n")
	}
	b.WriteString("- " + egressDescription(mode) + "\n")
	if mode == egress.Open {
		b.WriteString("- **This is a long-running research task.** Flush " +
			"partial findings to `/out` as you go so progress survives " +
			"interruption.\n")
	}

	writeHostTools(&b, mcpServers)
	writeOrchestration(&b)

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

// writeOrchestration appends guidance for agents that spawn child
// sandboxes via the demesne MCP server. Every agent run has that
// server wired in, so this is always emitted.
func writeOrchestration(b *strings.Builder) {
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
