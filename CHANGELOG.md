# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

[unreleased]: https://github.com/jbeshir/demesne/compare/e891550...HEAD
