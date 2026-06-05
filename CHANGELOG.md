# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- `DEMESNE_CODEX_MCP_CONFIG` env var (default `~/.codex/config.toml`): demesne now discovers stdio MCP servers from the Codex config and merges them with the Claude Code set. On a name conflict the Codex entry wins (with a logged warning). Codex's `env_vars` array (parent-process env-var names) is honoured.

### Changed
- **Breaking**: renamed env var `DEMESNE_HOST_MCP_CONFIG` → `DEMESNE_CLAUDE_CODE_MCP_CONFIG` (default unchanged: `~/.claude.json`). No back-compat alias.
- Default `DEMESNE_OUTPUT_ROOT` is now `~/.demesne/out` under the user's home, replacing the previous world-readable `/tmp/demesne/out`. Set `DEMESNE_OUTPUT_ROOT` explicitly to override.
- The effective output root is always appended to `DEMESNE_ALLOWED_PATHS`, so `/out` and nested `/in/previous-jobs/<name>` mounts work without the user listing the output root.

## [0.1.0] - 2026-06-05

First public release — an agent-agnostic, local, containerised agent-orchestration MCP server you drive from your agent of choice. It runs untrusted shell, scripts, and AI coding agents in disposable OpenSandbox containers, with read-only host mounts and egress allowlists.

### Tools
- **Sandboxes** — `sandbox_script` (one-shot) plus `sandbox_create` / `sandbox_exec` / `sandbox_upload` / `sandbox_download` / `sandbox_destroy` (persistent) run shell and scripts in disposable containers.
- **Agents** — `sandbox_agent` and `sandbox_research` run a coding-agent CLI inside a sandbox (`codex` by default when Codex credentials are configured, otherwise `claude-code`). Containerised agents can spawn child sandboxes and, with configuration, reach a read-only subset of the host's MCP server tools.

### Security and orchestration
- Read-only host inputs at `/in`, output-only `/out`, and per-tool egress allowlists; agent outbound HTTPS is confined to a credential-isolating per-sandbox proxy sidecar, so the agent never sees the real token.
- Separate, tail-bounded stdout/stderr in tool results; indicative per-run cost reporting; a results roll-up across the child-sandbox tree.
- Host MCP proxy: re-expose a curated, read-only subset of your configured MCP servers to containerised agents through a per-sandbox tunnel.

The milestone sections below (M1–M6) are the per-feature development log that rolls into this release.

## [M6-demesne-in-sandbox]

### Added
- **Child sandboxes**: agents inside a sandbox can spawn child sandboxes via demesne's own tools,
  re-exposed as an in-process `demesne` MCP server mounted on the M5 aggregator. Children inherit
  the parent's read-only `/in` and shared writable `/workspace`; output lands under
  `/out/child/<name>`.
- `results.json` per-job cost roll-up: `own_usage_usd` and `total_usage_usd` (summing descendant
  child sandboxes). Written atomically after each run; the parent's call returns only after all
  children finish.
- Per-instance aggregator socket (path derived from PID) so concurrent `demesne-mcp` sessions
  never share or unlink each other's socket.
- Shared `/workspace` mount across all agents spawned from a given parent; per-agent control files
  moved to a read-only config dir at `/in/.agent` to prevent cross-contamination.
- Trust-injection via `X-Demesne-Parent` tunnel header: the `demesne` in-process server receives
  the calling job's identity without exposing it to external upstream servers.
- `sandbox_script` and `sandbox_create` / `sandbox_exec` / `sandbox_destroy` exposed in-sandbox
  (upload/download deliberately excluded).
- Orphan reaper at startup: sandboxes left running by a previous crashed instance are reaped before
  the server begins accepting requests.

## [M5-mcp-proxy]

### Added
- **Host MCP proxy**: demesne re-exposes a curated, read-only subset of the stdio MCP servers in
  the host's Claude Code config (`~/.claude.json`) to sandboxed agents.
- In-process aggregator (`internal/mcpproxy`) discovers host stdio servers, spawns them lazily,
  and serves one Streamable-HTTP MCP endpoint per server on host loopback.
- Per-sandbox sidecar tunnel: one listener per server, sharing a single egress-bypass
  `http.Transport`; agents see each server under its upstream's own native tool names.
- Aggregator listens on a unix socket (`DEMESNE_MCP_SOCKET`, default
  `/tmp/demesne-mcp/aggregator.sock`; per-PID path `/tmp/demesne-mcp/<pid>/aggregator.sock`
  introduced in M6) so the tunnel works under rootless podman where host TCP
  ports are unreachable from the sandbox network namespace.
- Per-server tool allowlist: built-in read-only defaults (`internal/mcpproxy/defaults.go`),
  overridable via `DEMESNE_MCP_ALLOWLIST` JSON file; auto-seeded on first run.
- `DEMESNE_HOST_MCP_CONFIG` env var (default `~/.claude.json`) to override the MCP server
  discovery source.

## [M4-sandbox-research]

### Added
- `sandbox_research` tool: runs a Claude Code agent with no input mounts and unrestricted outbound
  internet egress (`open` mode).
- `EgressOpen` mode: `BuildNetworkPolicy` returns `DefaultAction: "allow"` (no per-host rules).
  `sandbox_agent` rejects `open` at the MCP boundary so the inputs + open-egress combination
  remains unreachable.
- Indicative cost reporting via `usage.json`: the Anthropic proxy accumulates per-model token
  counts and computes USD via an embedded pricing table, rewriting `usage.json` atomically after
  every response.
- Pricing table at `internal/proxies/anthropic/pricing.go`; longest-prefix-match on the model
  family name so dated model IDs route correctly. Cost cap removed — billing is subscription-based
  for the typical OAuth path.

## [M3-sandbox-agent]

### Added
- `sandbox_agent` tool: runs a Claude Code CLI instance in a fresh sandbox against a
  caller-supplied prompt.
- Per-sandbox **proxy sidecar** (`cmd/demesne-sidecar`): all registered proxies run on 127.0.0.1
  inside the egress sidecar's network namespace; agent outbound HTTPS is restricted to the
  per-vendor API proxy.
- Anthropic proxy (`internal/proxies/anthropic`): intercepts model API traffic, accumulates usage,
  and writes `usage.json` to the run's `/out`.
- `agent` parameter: `claude-code` (default) or `codex` (experimental). `model` parameter:
  provider-specific; claude-code accepts `opus`, `sonnet` (default), or `haiku`.
- `preamble` parameter: free-form prose prepended to the generated agent context file before the
  auto-generated `## Environment` and `## Task` sections.
- Sandbox layout: `/in` (read-only inputs + generated context), `/workspace` (writable scratch,
  agent cwd), `/out` (output only).
- Default egress for `sandbox_agent` is `none` (proxy upstream only); `package-managers` also
  available.

## [M2-persistent-sandboxes]

### Added
- `sandbox_create` tool: creates a long-lived sandbox and returns a `sandbox_id` handle.
- `sandbox_exec` tool: runs a shell command in an existing sandbox; refreshes the 24-hour TTL.
- `sandbox_upload` tool: copies a host file into a running sandbox (`src` must be inside
  `DEMESNE_ALLOWED_PATHS`).
- `sandbox_download` tool: copies a file out of a running sandbox, writing it under
  `<output_dir>/downloads/<basename>`.
- `sandbox_destroy` tool: tears down a sandbox; host output directory is preserved.
- Per-command timeout increased to 12 hours to support long-running data-processing scripts.
- Image, egress, and mounts are fixed at create time; shared logic factored into
  `internal/sandbox/runner.go` helpers.

## [M1-sandbox-script]

### Added
- `sandbox_script` tool: runs one shell command in a fresh container and tears it down. Returns
  exit code, stdout, and the `/out` host path.
- Image allowlist: `node` (`node:22`), `python` (`python:3.12`), `go` (`golang:1`), `anaconda`
  (`continuumio/anaconda3:latest`). Default: `anaconda`.
- Egress allowlist: `package-managers` (npm/PyPI/conda registries) or `none` (deny all).
  Default: `package-managers`.
- `files` and `directories` parameters: host paths mounted read-only at `/in/<basename>`. Each
  path must be absolute and inside `DEMESNE_ALLOWED_PATHS`.
- `DEMESNE_ALLOWED_PATHS` (required): colon-separated host paths permitted as mount sources.
- Writable `/out` mount: per-job output directory created under `DEMESNE_OUTPUT_ROOT`.
- CI workflow, integration test suite (`runner_integration_test.go`), and README with architecture
  diagrams.

[unreleased]: https://github.com/jbeshir/demesne/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/jbeshir/demesne/releases/tag/v0.1.0
