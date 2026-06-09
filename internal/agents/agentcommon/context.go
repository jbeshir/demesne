package agentcommon

import (
	"fmt"
	"strings"

	"github.com/jbeshir/demesne/internal/agents"
	"github.com/jbeshir/demesne/internal/mcpproxy"
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
	b.WriteString("- `IS_SANDBOX=1` is set. Do not start persistent daemons or " +
		"servers, and do not prompt for human input — neither has a recipient.\n")
	b.WriteString("- Your working directory is a private subdirectory of `/workspace`. " +
		"`/workspace` itself is shared writable scratch — if you spawn child " +
		"agents (see below) they share the same `/workspace`, so coordinate via " +
		"absolute `/workspace/...` paths. Copy any input you need to mutate into " +
		"`/workspace` first; do not try to modify `/in`.\n")
	b.WriteString("- `/out` is writable but **output only** — write your final " +
		"artefacts (results, reports, generated files) here. The caller reads " +
		"this back from the host after you exit. Your final response is captured " +
		"as the `stdout` field of the parent's tool result, so write large " +
		"artefacts to `/out` and return a short summary.\n")
	b.WriteString("- Each command you run inside the sandbox has its stdout and stderr captured to files under /out " +
		"(e.g. stdout.log and stderr.log for shell scripts; transcript.jsonl + stderr.log for agent runs). " +
		"The same streams are returned as separate stdout and stderr fields in the tool result; " +
		"stderr is tail-bounded to ~16 KiB.\n")
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
			"`/in/previous-jobs/<name>` — enumerate with `ls /in/previous-jobs/` " +
			"and read earlier siblings' results there.\n")
	}
	b.WriteString("- " + egressSentence + "\n")
	b.WriteString("- Flush partial findings to `/out` as you go so progress " +
		"survives interruption (especially for long runs).\n")
}

// WriteHostTools appends the "Available host tools" section listing each server's tools.
// No-op when no MCP servers are wired in.
func WriteHostTools(b *strings.Builder, mcpServers []agents.MCPServerInfo) {
	if len(mcpServers) == 0 {
		return
	}
	b.WriteString("\n## Available host tools\n\n")
	b.WriteString("These MCP servers from the host are wired into this run; " +
		"call their tools directly by name. Some may touch live external " +
		"accounts — treat the operator's allowlist as authoritative for what " +
		"is safe to call:\n\n")
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

// WriteFileGenNote appends a short note about file-generating MCP tools.
// Skipped when no wired server is a file-gen server.
func WriteFileGenNote(b *strings.Builder, mcpServers []agents.MCPServerInfo) {
	for _, s := range mcpServers {
		if mcpproxy.IsFileGenServer(s.Name) {
			b.WriteString("\nFile-generating MCP tools (image generation, mermaid diagrams) " +
				"deliver their output into `/workspace/generated/` inside this sandbox. " +
				"Copy any file you want returned to the caller into `/out/` (which is output-only).\n")
			return
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
	b.WriteString("Delivering results is your job, not a child's. Files a child " +
		"writes to its `/out/child/<name>` directory do NOT appear in your `/out`, " +
		"so to hand a child-produced or `/workspace` artefact back to your caller, " +
		"copy it into your own `/out` with plain `cp` (no toolchain needed). Do not " +
		"try to delegate this copy step to a `sandbox_script` child: that child would " +
		"write only to its own `/out/child/<name>` subtree, replicating the problem.\n\n")
	b.WriteString("- **Verify with an external signal, not self-critique.** For " +
		"code, spawn a `sandbox_script` child (image `node`, `python`, `go`, or " +
		"`anaconda`) to compile/test/lint against the shared `/workspace`. For " +
		"prose or research, spawn a fresh `sandbox_agent` judge child given the " +
		"artefact + criteria; it can read the worker's `/out/child/<name>/" +
		"transcript.jsonl` for reasoning trace. Go modules resolve automatically " +
		"via `GOPROXY`; for npm/PyPI/conda set `egress: package-managers`.\n")
	b.WriteString("- **Preserve a baseline for review.** If you copy a repo from " +
		"`/in` into `/workspace` to edit it, copy it whole (including `.git`) so " +
		"review phases can `git diff` your changes against the original.\n")
	b.WriteString("- **Plan and enforce the handoff.** Before implementing in phases, " +
		"decide what each phase produces, where, and in what format (appropriate to " +
		"your task) and follow that contract strictly across every phase. Do not " +
		"spawn a child for what your own tools can complete.\n")
	b.WriteString("- **Match effort to task; manage context across phases.** Prefer " +
		"`sandbox_script` for deterministic checks; pick `haiku` for lookup, " +
		"`sonnet` for general agentic work, `opus` for complex synthesis; try a " +
		"single well-prompted agent before fanning out. For long work, checkpoint " +
		"plan and findings to `/workspace` and spawn a fresh child referencing the " +
		"checkpoint rather than letting one context grow unbounded.\n")
	b.WriteString("- **Read failed children's stderr.** Every child sandbox's stderr is " +
		"surfaced as the `stderr` field in the tool result (tail-bounded), and the " +
		"complete log is at the child's `/out/stderr.log`. On a non-zero `exit_code`, " +
		"read the stderr before retrying — that's where the failure cause lives.\n")
}

// WriteDefinitionOfDone appends a "## Definition of done" section describing the expected output.
// No-op when the contract is empty.
func WriteDefinitionOfDone(b *strings.Builder, c agents.OutputContract) {
	if c.IsEmpty() {
		return
	}
	b.WriteString("\n## Definition of done\n\n")
	if c.Path != "" {
		fmt.Fprintf(b, "- **Output path:** `%s`\n", c.Path)
	}
	if c.Format != "" {
		fmt.Fprintf(b, "- **Format:** %s\n", c.Format)
	}
	if len(c.SuccessCriteria) > 0 {
		b.WriteString("- **Success criteria:**\n")
		for _, s := range c.SuccessCriteria {
			fmt.Fprintf(b, "    - %s\n", s)
		}
	}
}

// WriteTask appends the "## Task" section with the caller-supplied prompt.
func WriteTask(b *strings.Builder, prompt string) {
	b.WriteString("\n## Task\n\n")
	b.WriteString(strings.TrimSpace(prompt))
	b.WriteString("\n")
}
