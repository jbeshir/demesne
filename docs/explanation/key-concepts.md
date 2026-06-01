# Key concepts

## Core concepts

- **MCP (Model Context Protocol)** — JSON-RPC over stdio. Demesne is a stdio-transport MCP server; an AI agent (the parent process) sends `tools/call` requests and reads results from stdout.
- **OpenSandbox** — Alibaba's container-based sandbox runtime. Demesne talks to a lifecycle server over HTTP using their Go SDK.
- **Sandbox** — a container instance. `sandbox_script` creates one, runs a command, kills it. `sandbox_create` returns a long-lived handle; commands run against it via `sandbox_exec` until `sandbox_destroy`. Persistent sandboxes have a 24h TTL that's refreshed by each `sandbox_exec` call.
- **Image whitelist** — four accepted names for shell tools: `node` (`node:22`), `python` (`python:3.12`), `go` (`golang:1`, batteries-included with the Go toolchain + git + gcc + make), and `anaconda` (`continuumio/anaconda3:latest`, the default). `sandbox_agent` and `sandbox_research` use the agent provider's own image, built locally from an embedded Dockerfile (today: `demesne-claude-code:<hash>` for the claude-code provider; `demesne-codex:<hash>` for the codex provider).
- **Mounts** — caller-supplied host files and directories are mounted read-only at `/in/<basename>`. A writable `/out` mount is provisioned automatically; its host path is returned so the caller can read produced artifacts. `sandbox_agent` adds a writable `/workspace` mount and runs the agent from a private subdirectory of it; the generated context file (`CLAUDE.md` for claude-code) and MCP config are mounted read-only under `/in/.agent` (kept off `/workspace` so sibling/child agents sharing the mount can't clobber each other's control files). `sandbox_research` is the same but never has any other `/in/<basename>` mounts. `sandbox_upload` and `sandbox_download` move individual files in and out at runtime via the SDK (not through `/out`).
- **AllowedPaths** — env-configured whitelist (`DEMESNE_ALLOWED_PATHS`) of host paths under which inputs may be mounted or uploaded. Both the candidate path and the allowlist entries are symlink-resolved before the containment check, so symlink escapes are rejected.
- **Sandbox ID** — handle returned by `sandbox_create` (the OpenSandbox-issued UUID). Passed to `sandbox_exec` / `sandbox_upload` / `sandbox_download` / `sandbox_destroy`. The host output directory for a persistent sandbox is returned as `output_dir` in the create response; treat it as opaque.

## Agent sandboxes

### Egress modes

`none` denies all outbound; `package-managers` (default for `sandbox_script` / `sandbox_create`) allows registry.npmjs.org, pypi.org, files.pythonhosted.org, repo.anaconda.com, and conda.anaconda.org; `open` allows everything and is only reachable through `sandbox_research`. `sandbox_agent` rejects `open` — combining read-only `/in` mounts with unrestricted outbound is the data-exfiltration shape demesne keeps off the surface. Image and egress are fixed at create time.

### Child sandboxes

For `sandbox_agent` and `sandbox_research`, demesne re-exposes its own tools to the agent as an in-process `demesne` MCP server mounted on the host MCP proxy (so it rides the same per-sandbox tunnel). The agent can spawn child sandboxes (`sandbox_script` / `agent` / `research` / `create` / `exec` / `destroy`; no `upload`/`download`). `sandbox_agent` children inherit the parent's read-only `/in` and shared writable `/workspace`; their `/out` is `/out/child/<name>` (visible to parent and ancestors; descendants nest deeper). `sandbox_research` children are the exception: they get a fresh private workspace with no `/in` mounts. Names are required and unique per parent. Identity is conveyed by a trusted header the sidecar tunnel injects (the agent can't forge it), so there's no auth — consistent with the sandbox-edge trust boundary. There is no recursion depth cap. Each agent run writes `<out>/results.json` with `own_usage_usd` and a `total_usage_usd` that sums the whole descendant tree.

### Per-sandbox proxy sidecar

Every sandbox gets a sidecar container that joins OpenSandbox's egress-sidecar network namespace and runs demesne's proxies on `127.0.0.1`. The Go module proxy on `127.0.0.1:8087` runs in every sandbox so `go get`/`go mod download` resolve even under `egress=none`. `sandbox_agent` / `sandbox_research` additionally run the **Anthropic API proxy** (`8088`) and the **MCP tunnel** (`8089+`). Each proxy bypasses OpenSandbox's egress filter via `SO_MARK` (the sidecar has `CAP_NET_ADMIN`; the sandbox does not). The Go checksum database (`sum.golang.org`) is the exception: `go` contacts it directly, so demesne adds it to every sandbox's egress allowlist (keeping module verification on). The real OAuth token transits the Anthropic proxy unchanged.

### Indicative cost reporting

The sandbox's vendor proxy parses upstream API responses for `usage` blocks (both streaming SSE and JSON bodies), accumulates token counts per model family, and computes USD cost from an embedded pricing table. Snapshots are rewritten atomically to `usage.json` after every request and surfaced in the MCP response as `cost_usd`. The value is **indicative** — for example, Claude Code OAuth tokens typically authorise against a Claude Console subscription rather than per-request API billing, so the figure is useful for budgeting but not what the user is actually charged.

## Host MCP integration

### Host MCP tools

For `sandbox_agent` and `sandbox_research`, demesne can re-expose the stdio MCP servers already configured in your Claude Code config (`~/.claude.json`). At startup demesne discovers those servers, spawns them lazily on demand, and serves one HTTP MCP endpoint per server on host loopback (the in-process *aggregator*). Each sandbox's sidecar runs one tunnel listener per server (ports `8089`, `8090`, …) sharing a single outbound connection to the host. The agent sees each server in its own `--mcp-config` entry under the upstream's **native tool names** (no synthetic prefix). Only tools on the read-only allowlist are ever exposed — `tools/list` itself is filtered at the aggregator. There is no auth between the agent, the tunnel, and the aggregator: the sandbox edge is the trust boundary, and the aggregator listens only on a host-reachable interface. The allowlist is built-in read-only defaults per known server, overridable per-server via a JSON file (auto-seeded at `~/.config/demesne/mcp-allowlist.json`). Resources, resource templates, prompts, and completion are relayed in full without allowlist filtering.
