# Architecture

Demesne is a **stdio MCP server** that exposes eight tools — `sandbox_script`, `sandbox_create`, `sandbox_exec`, `sandbox_upload`, `sandbox_download`, `sandbox_destroy`, `sandbox_agent`, and `sandbox_research` — all of which delegate container lifecycle to an [OpenSandbox](https://github.com/alibaba/OpenSandbox) server over HTTP. The calling AI agent (Claude Code or any MCP client) communicates with demesne over JSON-RPC on stdin/stdout; demesne never opens a TCP listener of its own.

For a glossary of the terms used throughout this document see [key-concepts.md](key-concepts.md). For the trust-edge diagram (agent → sidecar → egress → vendor) see [trust-boundary.md](trust-boundary.md). For the external-service dependency map see [dependencies.md](dependencies.md).

---

## Component map

| Package | Role |
|---|---|
| `cmd/demesne-mcp` | Entry point. Loads env config, constructs a `sandbox.Runner`, and serves MCP over stdio via the `internal/server` handlers. |
| `cmd/demesne-sidecar` | Entry point for the per-sandbox sidecar binary. Launched inside the sidecar container; registers and starts whichever proxies the sandbox needs. |
| `internal/server` | Registers the eight MCP tools, parses tool arguments, and delegates to `internal/sandbox`. |
| `internal/sandbox` | Core runner: validates mounts, resolves images, builds network policies, calls the OpenSandbox SDK, and implements each tool's lifecycle (create, exec, agent, research, …). Also owns the child-sandbox registry and results roll-up. |
| `internal/agents` | Agent provider implementations under `internal/agents/anthropic` (claude-code) and `internal/agents/codex` (codex, experimental). Each subpackage owns the provider's Dockerfile, wrapper script, context-file renderer, and MCP config generator. |
| `internal/proxies` | Proxy implementations used by the sidecar, organised by vendor: `anthropic` (port 8088), `openai` (port 8086), `goproxy` (port 8087), `mcp` (ports 8089+). Shared base logic lives in `proxycommon`. |
| `internal/mcpproxy` | Host-side MCP aggregator: discovers stdio servers from `~/.claude.json`, enforces the tool allowlist, and serves one HTTP MCP endpoint per server on a unix socket. |
| `internal/sidecar` | Sidecar runtime: builds and starts the sidecar container via the OpenSandbox SDK, waits for it to be ready, and tears it down with the parent sandbox. |

---

## Per-sandbox proxy sidecar

Every sandbox (script, persistent, and agent alike) spawns a companion **sidecar container** that joins OpenSandbox's egress-sidecar network namespace. The sidecar is built from the embedded `cmd/demesne-sidecar` binary and runs whichever proxies the sandbox needs, all bound to `127.0.0.1` on well-known ports.

Because the sidecar runs in the egress-sidecar netns, it can reach the open internet via a `SO_MARK`-tagged socket that bypasses OpenSandbox's egress filter (`CAP_NET_ADMIN` is granted only to the sidecar, not the sandbox). The sandbox itself reaches the internet only through the sidecar's loopback proxies — giving demesne precise control over what the sandboxed process can actually reach regardless of the declared egress policy.

See [trust-boundary.md](trust-boundary.md) for the full trust-edge diagram showing how traffic flows from the agent through the sidecar to upstream vendor APIs.

---

## The four proxies

**Anthropic API proxy — `127.0.0.1:8088`** (`internal/proxies/anthropic`)

Present in `sandbox_agent` and `sandbox_research` sandboxes using the claude-code provider. The proxy holds the real `DEMESNE_CLAUDE_CODE_OAUTH_TOKEN` from the sidecar environment; the sandboxed claude-code sees only a per-sandbox synthetic bearer token (`demesne-agent-…`) that the proxy validates and swaps for the real token before forwarding upstream. The proxy also parses `usage` blocks in every API response (both streaming SSE and JSON bodies), accumulates token counts per model family against the embedded pricing table (`internal/proxies/anthropic/pricing.go`), and writes indicative cost snapshots to `usage.json` after each request.

**OpenAI / Codex proxy — `127.0.0.1:8086`** (`internal/proxies/openai`)

Present in `sandbox_agent` sandboxes using the codex provider. Works the same way as the Anthropic proxy but for the ChatGPT Codex backend: demesne reads the host's OAuth token set from `DEMESNE_CODEX_AUTH_FILE` (default `~/.codex/auth.json`, written by `codex login`), holds it off-agent, refreshes it autonomously, and injects a fresh access token into each forwarded request. The sandboxed Codex sees only the synthetic token and never the real credential.

**Go-module proxy — `127.0.0.1:8087`** (`internal/proxies/goproxy`)

Present in **every** sandbox. The sandbox's `GOPROXY` environment variable is set to point at this proxy, which forwards to `proxy.golang.org` (including the `/sumdb/` checksum endpoint). This lets `go get`/`go mod download` resolve modules even in `egress=none` sandboxes, without adding `proxy.golang.org` to the per-sandbox egress allowlist. The Go checksum database (`sum.golang.org`) is the one host demesne adds explicitly to every sandbox's egress allowlist, keeping module verification on.

**MCP tunnel — `127.0.0.1:8089+`** (`internal/proxies/mcp`)

Present in `sandbox_agent` and `sandbox_research` sandboxes. One loopback listener per discovered host MCP server (8089 for the first, 8090 for the second, …). Each listener forwards over the aggregator unix socket (bind-mounted into the sidecar). The agent is pointed at these listeners via `--mcp-config --strict-mcp-config`, so each upstream server appears under its native tool names with no synthetic prefix.

---

## MCP aggregator and per-sandbox tunnel

At startup, `cmd/demesne-mcp` launches an in-process **MCP aggregator** (`internal/mcpproxy`). The aggregator reads `~/.claude.json` (or `DEMESNE_HOST_MCP_CONFIG`), discovers configured stdio MCP servers, and serves one `/<server>/mcp` HTTP endpoint per server on a **unix socket** (default `/tmp/demesne-mcp/<pid>/aggregator.sock`). Upstream server processes are spawned lazily on first use and kept alive.

Only tools on the read-only allowlist are ever advertised: the aggregator intersects each upstream's `tools/list` with the built-in per-server defaults (overridable via `~/.config/demesne/mcp-allowlist.json`) and filters the result, so a non-allowlisted tool never appears in `tools/list` and cannot be called.

The runner bind-mounts the aggregator socket into each sandbox's sidecar. The sidecar then runs the MCP tunnel proxy (one listener per server) sharing a **single egress-bypassing HTTP transport** over that socket. This design uses a unix socket rather than a host TCP port because under rootless podman the sandbox network namespace cannot reach a host-process TCP listener — a bind-mounted socket crosses the boundary as a regular file.

---

## `X-Demesne-Parent` trust header

When the sidecar MCP tunnel forwards a demesne tool call from an agent sandbox, it injects a `X-Demesne-Parent: <jobID>` header on the demesne binding only — stripping any value the client may have supplied so the agent cannot forge it. The in-process demesne self-server (mounted on the aggregator) reads this header and looks the job ID up in the runner's jobID → spawning-context registry, which is populated for every agent run.

This gives the demesne handlers the parent's identity without exposing it to the upstream MCP servers (which receive only the original tool request on their own endpoint). No separate auth mechanism is needed: the trust is structural, matching the sandbox-edge trust boundary already in place.

---

## Child-sandbox routing

`sandbox_agent` and `sandbox_research` re-expose demesne's own tools as an in-process **demesne self-server** mounted on the aggregator alongside the discovered host MCP servers (via an `ExtraServers` hook). The agent reaches this self-server through the same sidecar tunnel mechanism, so it can call `sandbox_script`, `sandbox_agent`, `sandbox_research`, `sandbox_create`, `sandbox_exec`, and `sandbox_destroy` (upload/download are excluded, as child sandboxes take no mount params).

Child sandboxes spawned by `sandbox_agent` inherit the parent's read-only `/in` and share the parent's writable `/workspace`; their `/out` is `/out/child/<name>`, visible to the parent and all ancestors. Grandchildren nest further: `/out/child/<name>/child/<grandchild>`, and so on, so the whole descendant tree materialises under the root run's `/out`. `sandbox_research` children are the exception — they get a fresh private workspace with no `/in` mounts.

Names are required and must be unique per parent. There is no recursion depth cap. Each agent run writes `<out>/results.json` with `own_usage_usd` and `total_usage_usd` (the latter sums the whole descendant tree's indicative cost).

The host process performs the actual container spawning (a sibling sandbox, not podman-in-podman). The `X-Demesne-Parent` header mechanism described above is what allows the demesne self-server to correctly attribute a child sandbox to its parent context.
