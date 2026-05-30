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
| **M3** — `sandbox_agent`           | **Done**    | `2c9fe6ee-bc4e-0e9b-6faa-fa97a50939f2`                         |
| **M4** — `sandbox_research`        | **Done**    | `780688b1-ba47-c04f-f754-278ea4cab2f3`                         |
| **M5** — MCP proxy                 | **Done**    | `1bad3a63-e185-9ffb-be8e-6d739af75ce4`                         |
| **M6** — demesne-in-sandbox        | **Done**    | `86f18069-38d6-3e93-7170-0a557b9d70b8`                         |
| Cross-cutting additions            | Partially shipped | `b3dceb25-b74a-98e1-508e-bb9c7d976029`                    |
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

## M3 — `sandbox_agent` (done)

Shipped: `sandbox_agent` runs a Claude Code instance in a fresh sandbox
against a caller-supplied prompt. A per-sandbox proxy sidecar handles
all outbound HTTPS, joined into OpenSandbox's egress-sidecar network
namespace; the agent only ever talks to 127.0.0.1. Provider abstraction
is in place so future agents (Codex, etc.) slot in alongside
`claude-code`, and a separate proxy registry handles future MCP and
Go-module proxies inside the same per-sandbox sidecar.

**Decisions taken / how it diverged from the original sketch:**
- Provider package is named for the **vendor** (`internal/agents/anthropic/`),
  not the CLI, so future Anthropic-vendor agents (e.g. a slimmer
  research client) share the package. Provider registration is via
  `init() { agents.Register(...) }` plus a blank import from
  `cmd/demesne-mcp/main.go`.
- Proxies live in a separate top-level package
  (`internal/proxies/<vendor>/`) with their own registry. Each proxy
  declares `EgressHosts()` and `ListenAddr()`; the sandbox runner adds
  every registered proxy's hosts to the egress allowlist.
- Per-sandbox **proxy sidecar** (`cmd/demesne-sidecar` + image built
  from an embedded linux/amd64 binary) runs all registered proxies on
  127.0.0.1. The sidecar joins OpenSandbox's egress sidecar via
  `--network=container:<egress-sidecar-id>` so the proxy's outbound
  traffic flows through OpenSandbox's nftables/DNS filtering just like
  any other sandbox egress. This replaced an earlier host-side proxy
  design that didn't work: OpenSandbox's DNS-based filter never sees
  `host.docker.internal` lookups (resolved via /etc/hosts), so
  hostname-allow rules couldn't whitelist the proxy IP.
- Auth: long-lived `CLAUDE_CODE_OAUTH_TOKEN` from `claude setup-token`,
  injected into the container env. `claude -p` runs without `--bare`
  (bare mode ignores the OAuth token env var). The proxy is pass-through;
  the token transits unchanged.
- Egress: `BuildNetworkPolicy` gained an `extraAllow` parameter; the
  agent path passes `proxies.EgressHosts()`. For M3 that's just
  `api.anthropic.com`. Default egress for `sandbox_agent` is `none`
  (proxy upstream only); `package-managers` is also available.
- Sandbox layout: `/in` (read-only inputs + the generated `CLAUDE.md`),
  `/workspace` (writable scratch, agent's cwd), `/out` (writable output
  only). `CLAUDE.md` is single-file-mounted at `/in/CLAUDE.md` and
  symlinked from `/workspace/CLAUDE.md` so the CLI finds it relative to
  cwd. The cross-cutting "shared `/workspace` across siblings" feature
  is deferred to M6.
- `CLAUDE.md` generator takes a caller `preamble` (prepended verbatim)
  plus an auto-generated `## Environment` section listing inputs, env
  conventions, `/workspace`, `/out`, and a `## Task` section with the
  prompt.
- Agent image is built locally from an embedded Dockerfile
  (`internal/agents/anthropic/Dockerfile`) tagged
  `demesne-claude-code:<hash>`. Build is sync.Mutex-guarded and skipped
  when `docker image inspect` already finds the tag.

**Out of scope for M3 (now M4+ work):** `sandbox_research`, MCP proxy,
streaming output, `results.json` with usage roll-up, shared
`/workspace`, multiple provider registrations.

## M4 — `sandbox_research` (done)

Shipped: `sandbox_research` runs a Claude Code instance with no input
mounts and unrestricted outbound internet egress. The per-sandbox
Anthropic proxy stays in front of the model API and reports indicative
cumulative cost via `usage.json`.

**Decisions taken / divergences from the original sketch:**
- Shared plumbing: `Runner.Agent` and `Runner.Research` both call a
  private `runAgent(internalAgentSpec)`. The spec carries the tool
  metadata label; the public adapters are thin request/result
  translators.
- New `EgressOpen = "open"` mode; `BuildNetworkPolicy` returns
  `DefaultAction: "allow"` (no rules). `sandbox_agent` rejects `open`
  at the MCP boundary so the inputs+open-egress combination is
  unreachable.
- **Indicative cost reporting (no cap).** The Anthropic proxy parses
  SSE `message_start` / `message_delta` events (and non-streaming JSON
  responses) for the `usage` block, accumulates per-model token counts,
  computes USD via an embedded pricing table, and rewrites
  `usage.json` atomically after every response. An enforced cost cap
  was built and then removed: for the typical Claude Code OAuth path
  the user is billed against a Console subscription rather than per
  request, so the figure is informational only and a hard ceiling
  conveyed the wrong model of how billing actually works.
- Pricing table lives at `internal/proxies/anthropic/pricing.go`;
  longest-prefix-match keyed on the family name so dated Anthropic
  model IDs (e.g. `claude-opus-4-8-20260101`) route to the family
  entry. Updating prices is a single-file change.
- Sidecar runtime gained a `ProxyConfig` (renamed from `ProxyTokens`)
  with `ResultsHost`. The sidecar bind-mounts a per-job results dir
  only into the sidecar (not the agent), so the agent can't tamper
  with the usage record. The runner copies `usage.json` into `/out`
  after the run for caller visibility.
- `Agent.GenerateContext` grew an `egress string` parameter so the
  CLI's CLAUDE.md tells the model exactly what's reachable
  (`none` / `package-managers` / `open`). The `open` variant also
  carries a research framing note ("flush partial findings to `/out`
  as you go").

**Out of scope (deferred):**
- `results.json` proper (with `total_usage_usd` rolled up across
  child sandboxes) — lands with M6.

## M5 — MCP proxy (done)

Shipped: demesne re-exposes a curated, read-only subset of the
stdio MCP servers in the host's Claude Code config to sandboxed
agents. An in-process aggregator discovers the servers, spawns
them lazily, and serves one Streamable-HTTP MCP endpoint per
server on host loopback; each sandbox's sidecar runs one tunnel
listener per server (sharing a single outbound connection) and the
agent is pointed at them via `--mcp-config --strict-mcp-config`.

**Decisions taken / divergences from the original sketch:**
- **Hybrid architecture**: the aggregator lives in-process inside
  `demesne-mcp` (no separate binary), but the sandbox still sees
  HTTP MCP endpoints via the sidecar tunnel
  (`internal/proxies/mcp/`).
- **Per-server endpoints, native tool names**: rather than one
  aggregated endpoint with synthetic `{server}__{tool}` names, the
  aggregator mounts one MCPServer per upstream at `/{server}/mcp`
  and the sidecar runs one listener per server (ports `8089+`).
  The agent sees each server separately under its upstream's own
  tool names. Listeners share a single egress-bypass
  `http.Transport` (the "one outward tunnel" property).
- **No auth** between agent, tunnel, and aggregator. The sandbox
  edge plus the egress filter are the trust boundary; the
  aggregator listens only on a host-reachable IP within the same
  trust domain. (Contrast the Anthropic proxy, whose token check
  exists to substitute a credential the agent must not see.)
- **Allowlist**: built-in read-only defaults per known server
  (`internal/mcpproxy/defaults.go`), overridable per server via a
  JSON file at `~/.config/demesne/mcp-allowlist.json`
  (`"default"` / `"*"` / explicit list / `[]`), auto-seeded on
  first run. Enforcement is at the aggregator: a non-allowlisted
  tool never appears in `tools/list`.
- **Host reachability via unix socket**: the aggregator listens on a
  unix socket; the runner bind-mounts it into each sidecar and the
  tunnel forwards over it. This was the crux of M5 — under rootless
  podman the sandbox network namespace can't reach a host-process
  TCP port (and `--add-host` is rejected in `--network=container:`
  mode), so every TCP-to-host scheme failed with a 502. A
  bind-mounted socket is a filesystem object and crosses the
  namespace boundary regardless. Socket path defaults to
  `/tmp/demesne-mcp/aggregator.sock` (`DEMESNE_MCP_SOCKET`).
- Discovery reads `~/.claude.json` (overridable via
  `DEMESNE_HOST_MCP_CONFIG`); stdio servers only, the `demesne`
  self-entry is skipped.

**Out of scope (deferred):** HTTP/SSE upstream servers, write
tools, per-sandbox MCP quotas, MCP resources/prompts.

## M6 — demesne-in-sandbox (child sandboxes) (done)

Shipped: an agent inside a sandbox can call demesne again to spawn
child sandboxes. demesne's own tools are re-exposed as an in-process
`demesne` MCP server mounted on the M5 aggregator (alongside the
discovered stdio upstreams), so the agent reaches them through the
same sidecar tunnel + unix socket. Per-job `results.json` rolls up
own + descendant cost.

**Decisions taken / divergences from the original sketch:**
- **Routed through the existing MCP proxy.** Rather than a new
  socket/component, the aggregator gained an `ExtraServers` hook; the
  runner registers its own `demesne` server there. It appears in
  `Servers()`/`Catalogue()` like any upstream, so `buildMCPWiring`,
  the sidecar tunnel, `--mcp-config`, and the CLAUDE.md host-tools
  listing all pick it up unchanged.
- **No auth; identity via a trusted tunnel header.** The `demesne`
  server is per-caller (each call spawns into the calling sandbox's
  own subtree), but external upstreams are context-free. The sidecar
  tunnel injects `X-Demesne-Parent: <jobID>` only on the demesne
  binding (stripping any client-supplied value first — the agent
  reaches only the loopback listener, never the socket). mcp-go's
  `WithHTTPContextFunc` lifts it into the tool handler ctx; the runner
  resolves it against a jobID→context registry it populates for every
  agent run. Consistent with the sandbox-edge trust boundary.
- **Tool scope:** spawners (`sandbox_script`/`agent`/`research`) plus
  the persistent family (`sandbox_create`/`exec`/`destroy`).
  `upload`/`download` are deliberately not exposed in-sandbox. No tool
  takes mount params; every child inherits the parent's read-only
  `/in` and shared writable `/workspace`, and writes to
  `/out/child/<name>`. Names are required and unique per parent.
- **No recursion depth cap.** Runs are subscription-billed; the bound
  is the session window, not a configured depth.
- **Shared-/workspace collision fix.** Because siblings share the
  `/workspace` mount, per-agent control files moved off it: the
  generated context file + MCP config now live in a read-only config
  dir mounted at `/in/.agent`, and each agent runs from a private cwd
  `/workspace/.demesne/<jobID>`. This was forced by Claude Code
  walking ancestor dirs for `CLAUDE.md` — a shared `/workspace/CLAUDE.md`
  would leak into descendants. `--strict-mcp-config` suppresses any
  ancestor `.mcp.json`.
- **results.json (cross-cutting) landed here.** Each run writes
  `own_usage_usd` + `total_usage_usd` (summed over `/out/child/*`
  descendant results) to `<out>/results.json`. Children finish before
  the parent's call returns, so the roll-up is bottom-up.

**Out of scope (deferred):** non-tree topologies, sharing children
across unrelated parents, per-sandbox MCP quotas, streaming/incremental
`/out` writes, depth/fan-out caps.

## Cross-cutting additions

These land alongside the milestones that need them, not as a separate phase.

- **Streaming output to `/out`** — agents write incremental output to their
  output directory while running; demesne extracts the final state into
  `results.json` on termination. Affects M3+.
- **`results.json`** — **shipped in M6.** Per-job summary at
  `<out>/results.json` with `own_usage_usd` and `total_usage_usd` (the
  latter rolls up descendant child sandboxes).
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
   Output: a milestone-specific plan in `plans/` (in-repo).
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
