# Using demesne tools from inside an AI agent

When an AI agent is running inside a `sandbox_agent` or `sandbox_research` sandbox, demesne re-exposes its own tools to that agent as an in-process `demesne` MCP server, wired through the same per-sandbox sidecar tunnel as the host MCP servers (see `internal/sandbox/childserver.go` and `internal/sandbox/agent.go`). The block below is a ready-to-paste addition to any agent's system prompt — including top-level orchestrators that will spawn `sandbox_agent` or `sandbox_research` children. Every behavioural claim in the block is grounded in `internal/sandbox/agent.go`, `internal/sandbox/children.go`, `internal/sandbox/childserver.go`, and the [key-concepts](../explanation/key-concepts.md) and [trust-boundary](../explanation/trust-boundary.md) docs.

## System prompt block

```
## demesne sandbox tools

You have access to a set of demesne tools for running shell commands, scripts, and AI agents in isolated containers.

### Choosing the right tool

- **sandbox_script** — single-shot: spin up a fresh container, run one shell command, tear it down. Use for quick scripts, build steps, or test runs where you don't need state between commands.
- **sandbox_create + sandbox_exec (+ sandbox_destroy)** — persistent session: create one container, run multiple commands against it (state accumulates), destroy when done. Use when a sequence of commands must share a filesystem (e.g. pip install, then run a script that uses the installed package).
- **sandbox_agent** — run an AI coding agent (`claude-code` by default, or `codex` — experimental) inside a fresh sandbox against a prompt. Use when the task needs the agent's own reasoning, tool use, and multi-step execution. The agent inherits the parent's read-only /in mounts and shares /workspace. Egress is restricted to the vendor proxy (plus optional package-managers); open egress is refused here.
- **sandbox_research** — like sandbox_agent but with unrestricted outbound internet (open egress) and NO input mounts. Use for open-ended research that needs to fetch from the web. The combination of read-only inputs + open egress is intentionally unavailable: use sandbox_agent for inputs, sandbox_research for open egress — never both.

### Egress modes

- **none** (default for sandbox_agent children) — only the vendor proxy is reachable; no external hosts.
- **package-managers** (default for sandbox_script / sandbox_create children) — npm, PyPI, conda registries are reachable in addition to any sidecar bypasses.
- **open** — unrestricted internet access; only available through sandbox_research (any other tool rejects it).

### Child sandboxes

You can spawn child sandboxes from inside a sandbox_agent or sandbox_research run. The available child tools are: sandbox_script, sandbox_agent, sandbox_research, sandbox_create, sandbox_exec, sandbox_destroy (upload/download are not available to children).

- sandbox_agent children inherit the parent's read-only /in mounts and share /workspace. Their /out is /out/child/<name>.
- sandbox_research children get a FRESH PRIVATE workspace with NO /in mounts (isolated); their /out is still /out/child/<name>.
- Grandchildren nest further: /out/child/<name>/child/<grandchild-name>.

### The `name` parameter

Every child-spawning call (sandbox_script, sandbox_agent, sandbox_research, sandbox_create) requires a `name` parameter. Rules:
- Lowercase letters, digits, and interior hyphens only (no underscores, dots, or uppercase).
- Interior hyphens only — the name may not start or end with a hyphen.
- Maximum 40 characters.
- Must be unique within the current sandbox (reusing a name is an error).

### Delivering results: your job, not a child's

A child's /out/child/<name> is that child's own output directory. Files written there by the child do NOT automatically appear in your /out. To hand a child-produced artefact back to your caller, copy it into your own /out after the child finishes:

  sandbox_script name="copy-results" command="cp /out/child/analyzer/report.txt /out/report.txt"

Or equivalently with any shell cp inside a sandbox_script child. Never delegate this copy to the child that produced the file — it would land in its own /out/child/<name> again.

### Completed sibling outputs

Completed siblings' outputs are mounted read-only under /in/previous-jobs/<name> so a later sibling can read what an earlier sibling produced without going through /out.

### Host MCP tools

The host MCP servers (e.g. workflowy, alignment, anki) appear in your tool list under their native tool names. Only tools on the built-in read-only allowlist (or the operator's override) are available; calls to non-allowlisted tools will fail. There is no auth between you, the sidecar tunnel, and the aggregator — the sandbox edge is the trust boundary.
```

See [Spawn nested agents](spawn-nested-agents.md) for the output-path convention and the copy-to-`/out` rule outside the system-prompt context.

## Composing a scoped task prompt

When calling `sandbox_agent` or `sandbox_research`, split the instructions across five fields:

| Field | Purpose |
|-------|---------|
| `preamble` | Role and must-not constraints — prepended verbatim before the auto-generated environment block. |
| `prompt` | The actual task: what to do, what to read, where to write output. |
| `output_path` | Where the agent must write its final artefact. |
| `output_format` | Expected shape or format of the output. |
| `success_criteria` | Checklist of conditions the output must satisfy. |

`output_path`, `output_format`, and `success_criteria` are rendered as a `## Definition of done` block prepended to the task in the child's context file. The agent reads the acceptance bar before reading the task.

**Worked example** — spawn a research summariser:

```json
{
  "name": "sandbox_agent",
  "arguments": {
    "name": "summariser",
    "preamble": "You are a technical writer. Write in plain English. Do not fabricate citations.",
    "prompt": "Read /in/paper.pdf. Write a structured summary (background, method, results, limitations) to /out/summary.md.",
    "output_path": "/out/summary.md",
    "output_format": "Markdown with four H2 sections: Background, Method, Results, Limitations",
    "success_criteria": [
      "summary.md exists",
      "contains all four sections",
      "no fabricated citations"
    ]
  }
}
```

See [Writing the task prompt](spawn-nested-agents.md#writing-the-task-prompt) for a fuller discussion of the three-layer structure, including how `preamble` composes with the auto-generated environment section.

## Why each claim

- **sandbox_agent refuses `egress: "open"`** — `rejectOpenEgress` in `internal/sandbox/childserver.go:326-339` returns a tool-result error for any egress value of `"open"` on `sandbox_script`, `sandbox_agent`, and `sandbox_create` children. The comment names the exact threat: "inputs plus unrestricted outbound is the data-exfiltration shape demesne keeps off the child surface."
- **sandbox_research children are isolated (no /in, fresh workspace)** — `handleChildResearch` in `childserver.go:227-254` always sets `isolated: true` on the `childSpawn`; `buildChildLayout` in `agent.go:441-469` creates a fresh private workspace and skips inheriting `inputVolumes` / `previousJobs` when `c.isolated` is true.
- **Children inherit /in and share /workspace** — `buildChildLayout` in `agent.go:460-464`: non-isolated children copy `c.parent.inputVolumes` and set `workspaceHost = c.parent.workspaceHost`.
- **Child /out is /out/child/<name>** — `buildChildLayout` in `agent.go:449`: `outHost: filepath.Join(c.parent.outHost, "child", c.name)`.
- **Name rules: ≤40 chars, DNS-1123** — `validateChildName` in `internal/sandbox/children.go:129-147`: checks length > 40 and validates char-by-char (lowercase letters, digits, interior hyphens only).
- **Name uniqueness per parent** — `reserveName` in `children.go:49-60` uses a `usedNames` map and returns an error on collision.
- **Host MCP tools via sidecar tunnel** — `buildMCPWiring` and `ChildMCPServer` in `agent.go:653-676` and `childserver.go:96-152` wire the demesne self-server and host MCP servers through the aggregator + sidecar tunnel. Trust model: `docs/explanation/trust-boundary.md` §Host MCP tools.
- **Completed siblings under /in/previous-jobs** — `previousJobVolumes` in `children.go:101-120` mounts each sibling's outHost at `/in/previous-jobs/<name>` read-only.
