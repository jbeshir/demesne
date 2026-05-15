# Demesne Roadmap

Multi-milestone plan for demesne. Lightweight by design — each milestone lists
goal, scope, key decisions, and out-of-scope items. Expand to a full per-file
plan (in `~/.claude/plans/` or a feature branch's plan doc) only when starting
that milestone.

## Source of truth

The long-term vision lives in Workflowy node
[`Demesne`](https://workflowy.com/#/6bee0a06a7b8) (id
`6bee0a06-a7b8-c2b6-86d9-60932f2ad57a`, under *Interesting Projects*). If this
file disagrees with that node, the node wins — update this file to match.

## Status

| Milestone                          | Status      | Workflowy node                                                 |
|------------------------------------|-------------|----------------------------------------------------------------|
| Private GitHub repo                | Done        | `2128b887-721d-2c87-b4a8-62070672f651`                         |
| **M1** — `sandbox_script`          | **Done**    | `18eeafba-e5a6-6c81-b75c-648b4533694c`                         |
| **M2** — persistent sandboxes      | **Done**    | `d77fdc59-ce80-cf01-0c02-ea9e5d6c1064`                         |
| M3 — `sandbox_agent`               | Not started | `2c9fe6ee-bc4e-0e9b-6faa-fa97a50939f2`                         |
| M4 — `sandbox_research`            | Not started | `780688b1-ba47-c04f-f754-278ea4cab2f3`                         |
| M5 — MCP proxy                     | Not started | `1bad3a63-e185-9ffb-be8e-6d739af75ce4`                         |
| M6 — demesne-in-sandbox           | Not started | `86f18069-38d6-3e93-7170-0a557b9d70b8`                         |
| Cross-cutting additions            | Not started | `b3dceb25-b74a-98e1-508e-bb9c7d976029`                         |
| Quality gates (descriptions/tests) | Ongoing     | `e9fa65f6`, `974e9a09`, `f23b4cd2`                             |

## M1 — `sandbox_script` (done)

A single MCP tool that runs one shell command in a fresh sandbox, returns
stdout, and tears the sandbox down. Image whitelist (`node`/`python`/`anaconda`),
egress whitelist (`none`/`package-managers`), `/in/<basename>` read-only mounts,
writable `/out`, allowed-paths whitelist on host inputs.

Shipped: `cmd/demesne-mcp`, `internal/server`, `internal/sandbox`, CI workflow,
README with mermaid diagrams, and `runner_integration_test.go` (env-gated
end-to-end test against a real OpenSandbox, runnable via `make
test-integration` with config in `.env` / `.env.dist`).

## M2 — persistent sandboxes (done)

Shipped: `sandbox_create` / `sandbox_exec` / `sandbox_upload` /
`sandbox_download` / `sandbox_destroy`. `sandbox_script` retained as the
single-shot fast path.

**Decisions taken:**
- Handle = OpenSandbox sandbox ID (no host-side state). Re-attach via
  `ConnectSandbox` on every operation. Our internal jobID (host /out dir
  name) lives in sandbox metadata as `demesne.job`; `sandbox_download`
  reads it from `GetInfo` to find the host downloads dir.
- TTL = 24h (OpenSandbox default), refreshed on each `sandbox_exec` via
  `sb.Renew`. Active sandboxes stay alive; idle ones expire — no janitor.
- `sandbox_download` writes under `<output_dir>/downloads/<basename>` so
  the caller never has to choose (or be authorised for) a host
  destination.
- Per-command timeout bumped to 12h (was 30m) — long data-processing
  scripts are a legitimate use case.
- `sandbox_script` stays independent (not refactored to wrap
  create+exec+destroy). Shared logic factored into
  `internal/sandbox/runner.go` helpers (`prepareSandbox`, `attach`,
  `connectionConfig`).
- Image / egress / mounts are fixed at create time. Runtime mutation
  (`EgressClient`-based egress changes) deferred to a later milestone.

## M3 — `sandbox_agent`

**Goal:** run an AI agent inside a sandbox against a prompt; return results.

**Tool to add:**
- `sandbox_agent` — params: agent (`claude-code` initially), model
  (`opus`/`sonnet`/`haiku`), prompt. Returns the agent's output.

**Key decisions:**
- Embed Dockerfiles for agent images (built locally via Docker/Podman before
  OpenSandbox sees them). Image build cache strategy.
- Subpackage layout: one provider per subpackage (e.g. `internal/agents/anthropic/`),
  each owning its `CLAUDE.md` generator, env vars, etc. The top-level runner
  command must not know any model/provider/agent names — registration only.
- Anthropic API proxy: a host-side HTTP proxy mounted into the container so
  Claude Code calls the proxy, which forwards to api.anthropic.com. Lets us
  rate-limit, log, and inject `IS_SANDBOX=true` etc. Skip permission checks
  inside the sandbox.
- Long-lived Claude Code auth token: comes in via demesne config, gets
  set in the sandbox env. Mechanism for rotation.

**Out of scope for M3:** MCP proxy, child sandboxes, the demesne-in-sandbox
reduced toolset.

## M4 — `sandbox_research`

**Goal:** an agent invocation with unrestricted internet for long-running
research.

**Tool to add:**
- `sandbox_research` — like `sandbox_agent` but no input mounts and egress
  fully open.

**Key decisions:**
- Should this share most of M3's plumbing (likely yes — it's `sandbox_agent`
  with an alternate egress policy and no inputs).
- Cost limits — research runs can be long and expensive; need a usage cap.

**Out of scope:** anything that doesn't follow from M3 + an open egress policy.

## M5 — MCP proxy

**Goal:** expose a curated, read-only subset of host MCP servers to
sandboxed agents.

**Mechanism:**
- Read host MCP config (e.g. `~/.config/claude/mcp.json` or equivalent).
- Spin up the relevant MCP servers on the host.
- Expose a single HTTP MCP endpoint inside the container, gating to a
  whitelist of approved read-only tools (e.g. Workflowy read, search). Point
  the in-container agent at this endpoint via config.

**Key decisions:**
- Which tools count as "read-only" — explicit allowlist per server, not
  derived heuristics.
- Auth between sandbox and the host MCP endpoint (shared secret over the
  mounted config? mTLS over a unix socket bind-mount?).
- Lifecycle: per-sandbox proxy instance, or a single shared proxy?

**Out of scope:** mutating tools, anything not on the read-only allowlist.

## M6 — demesne-in-sandbox (child sandboxes)

**Goal:** let an agent inside a sandbox call demesne again to spawn child
sandboxes.

**Adjustments to the in-sandbox flavour:**
- No `files`/`directories` params on `sandbox_*` tools. Children inherit
  the parent's inputs.
- Children get names + descriptions; names are unique within a parent.
- Child output paths are `{parent-output}/child/{name}`.

**Key decisions:**
- How demesne-in-sandbox authenticates to the host demesne (mounted unix
  socket? token in env?).
- Recursion depth limits.
- Cost accounting roll-up (lands with the cross-cutting `results.json` work).

**Out of scope:** non-tree topologies, sharing children across unrelated
parents.

## Cross-cutting additions

These land alongside the milestones that need them, not as a separate phase.

- **Streaming output to `/out`** — agents write incremental output to their
  output directory while running; demesne extracts the final state into
  `results.json` on termination. Affects M3+.
- **`results.json`** — per-job summary including `own_usage_usd` and
  `total_usage_usd` (the latter rolls up child sandboxes). Affects M3+.
- **Stderr logs** — each agent's stderr captured to a file in the output
  directory.
- **`/workspace` mount** — writable temp workspace at `/workspace`, stored at
  `/tmp/demesne/workspaces/{uuid}` on the host, shared across all agents
  spawned from a given parent. Prompting instructions tell agents to use it
  for iteration/editing/review and coordinate via subagent instructions.
- **`/in/previous-jobs/{name}`** — mount the output of all previous sibling
  agents into new agents at this path.

## Ongoing quality gates

Apply per milestone, not as a separate phase:

- **Tool descriptions** — every tool registration, manifest entry, and README
  table stays in lockstep; descriptions are high-quality enough to drive
  correct LLM use.
- **Test coverage** — every milestone ships unit tests for new logic plus at
  least one integration test gated on a real OpenSandbox.
- **Code quality** — no duplicate code, no history-shaped complexity. Run
  the `/jbeshir-agent-skills:quality-pass` skill before each PR.

## How to execute a milestone

1. Read this file + the linked Workflowy node for that milestone.
2. Spawn the **Plan** subagent (or `/plan` skill) with the milestone's
   "Goal" + "Key decisions" + the current state of `internal/sandbox/`.
   Output: a milestone-specific plan in `~/.claude/plans/`.
3. Get the plan approved (`ExitPlanMode`).
4. Implement, lint, test, write the integration test.
5. Run `/jbeshir-agent-skills:quality-pass`.
6. PR + CI.
7. Update this file: mark the milestone done; capture any decisions that
   diverged from the original plan; add anything that bled into a later
   milestone's "Out of scope" section.

## Out of scope (project-wide, for now)

- Authentication between MCP client and demesne (trust the stdio parent).
- MCPB cross-platform packaging beyond the existing Makefile stub.
- Pause/resume, snapshots, custom resource limits, images outside the
  whitelist.
- Multi-tenant operation (one demesne process serves one MCP client).
