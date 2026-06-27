# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- **`twine` sandbox image** (`internal/sandbox/twineimage`, `demesne-twine`): a demesne-built, lazily-built image layering the Tweego interactive-fiction compiler and the bundled Twine story formats (Harlowe, SugarCube, …) onto the browser image's Playwright/Chromium + Node base, so a Twine story can be compiled and headlessly playtested entirely at `egress=none`. Selectable as `image: "twine"` on `sandbox_script`/`sandbox_create`/`sandbox_exec` and the in-sandbox child surfaces.
- **`webgamedev` sandbox image** (`internal/sandbox/webgamedevimage`, `demesne-webgamedev`): a demesne-built, lazily-built image baking a warm Phaser + Vite + TypeScript template (with `node_modules` pre-installed) at `/opt/game-template` onto the same Playwright/Chromium + Node base, so an HTML5 game can be built and headlessly playtested entirely at `egress=none`. Selectable as `image: "webgamedev"`. Shares the `browser` image's base layer, so podman caches it once across all three. Fast-moving versions (Tweego, Phaser, Vite, TypeScript, the Playwright tag) are build `ARG`s with pinned defaults that join the content-hash cache key.

### Changed

### Fixed

## [0.2.0] - 2026-06-27

### Added
- **`background` option** on `sandbox_script`, `sandbox_agent`, and `sandbox_research` (host and in-sandbox child surfaces): when `true`, the tool returns immediately with `{job_id, status:"running"}` instead of blocking.
- **`sandbox_status`** tool (host and child): non-blocking status snapshot for a background job — returns status, elapsed time, a stdout tail, and cost/exit-code once terminal.
- **`sandbox_wait`** tool (host and child): blocks up to `timeout_seconds` (default 30, hard-capped at 120) for a background job to reach a terminal state; returns the final result or `{status:"running", message:"still running; call sandbox_wait again"}` on timeout.
- **`sandbox_cancel`** tool (host and child): cancels a background job and its entire descendant subtree depth-first, tearing down each sandbox via the existing sidecar/egress deferred path.
- **In-memory job registry** (`internal/sandbox/jobs.go`): job state lives in memory for the process lifetime with no on-disk persistence; jobs do NOT survive an MCP-server restart (a stale job_id then returns `ErrJobNotFound`); a TTL reaper retains terminal jobs ~1h to bound memory; orphaned containers from a crashed/restarted process are reaped independently by `ReapOrphans` via the `demesne.owner` label.
- **`sandbox_usage_report` tool**: token-usage & cost introspection for a finished job — walks the job's `/out` tree and breaks spend down by child/phase, model, token-type, and (claude-code) per-subagent, joining the per-request `usage.jsonl` against a distilled transcript `attribution.jsonl`; unattributed spend rolls up to `(main)` and is never dropped, with an `OutputRoot`-escape guard. `AgentResult` / `results.json` gain an additive `per_model_tokens` breakdown rolled up the descendant tree, and previously-silent parse / no-usage-block drops are now counted in `usage.json`.

### Changed
- **claude-code agent image tracks the host Claude Code version**: the image build folds a `CLAUDE_CODE_VERSION` build arg (resolved from the host `claude --version`, falling back to `npm view` then `latest`) into its content-hash cache key, so the sandbox CLI rebuilds automatically on a host Claude Code upgrade instead of drifting behind and rejecting a freshly released model alias. No demesne release is needed for the sandbox to track the host. Image builders without a `BuildArgs` resolver hash exactly as before.
- **Build-toolchain telemetry disabled in the sandbox env**: `sandboxEnv()` injects telemetry/analytics opt-out vars (wrangler `WRANGLER_SEND_METRICS`/`ERROR_REPORTS`, the Next/Nuxt/Angular/Storybook/Vercel/Yarn CLIs, npm update-notifier/funding noise, pip's version check, Prisma/HashiCorp checkpoint, Nx Cloud, plus `DO_NOT_TRACK=1` as a catch-all) so build tools don't phone home. Under restricted egress these calls previously stalled the build against the deny-by-default network policy until they timed out, so this also de-flakes and speeds up sandboxed builds.
- **Internal job hooks** (`JobHooks`, `internalAgentSpec`, `sandboxPrepOptions`): the mid-run job-tracking plumbing was reduced to a single `OnOutputReady(outHost, resultsHost)` callback that records the live output/results paths for `sandbox_status`; the write-only `OnSandboxCreated` hook and run-UUID parameter (and their now-dead job fields) were dropped. Internal only — no behaviour change; the MCP tool surface (`sandbox_status`/`sandbox_wait`/`sandbox_cancel`) is unchanged.

### Removed
- **`agent` parameter** on `sandbox_agent` / `sandbox_research` (and the in-sandbox child variants): model aliases are globally unique across providers, so the provider is now inferred from `model` via a registry-driven lookup guarded by a uniqueness test. Setting only a claude-code `model` such as `sonnet` no longer fails against the codex-first default provider (`model "sonnet" is not in the Codex allowlist`). An empty `model` preserves the credential-aware default: codex / `gpt-5.5` when Codex credentials are configured, otherwise claude-code / `sonnet`.

## [0.1.1] - 2026-06-10

### Added
- **`fable` model tier**: the Claude `fable` alias (most capable tier, above `opus`) is now selectable as the `model` for `sandbox_agent` / `sandbox_research` and the in-sandbox child variants when claude-code credentials are configured. Added to the pricing catalog so its usage counts toward cost reporting and the cap.
- **`media` sandbox image**: a new demesne-built image (FROM ubuntu:24.04) carrying ffmpeg, ImageMagick, libvips, and a broad audio toolbox (sox, lame, flac, opus-tools) for video/audio/image conversion. Wired through `sandbox_script` / `sandbox_create` / in-sandbox child variants exactly like the existing `browser` image; built lazily on the host on first use and content-hash cached via `agentcommon.ImageBuilder`.

## [0.1.0] - 2026-06-06

First public release — an agent-agnostic, local, containerised agent-orchestration MCP server you drive from your agent of choice. It runs untrusted shell, scripts, and AI coding agents in disposable OpenSandbox containers, with read-only host mounts and egress allowlists.

### Tools
- **Sandboxes** — `sandbox_script` (one-shot) plus `sandbox_create` / `sandbox_exec` / `sandbox_upload` / `sandbox_download` / `sandbox_destroy` (persistent) run shell and scripts in disposable containers.
- **Agents** — `sandbox_agent` and `sandbox_research` run a coding-agent CLI inside a sandbox: `codex` by default when Codex credentials are configured, otherwise `claude-code`. Each tool advertises its `agent` / `model` options filtered to the providers you have credentials for. Containerised agents can spawn child sandboxes and, with configuration, reach a read-only subset of the host's MCP server tools.

### Security and orchestration
- Read-only host inputs at `/in`; an output-only `/out` whose host directory defaults to `~/.demesne/out` (always included in the mount allowlist); per-tool egress allowlists; agent outbound HTTPS confined to a credential-isolating per-sandbox proxy sidecar, so the agent never sees the real token.
- Separate, tail-bounded stdout/stderr in tool results; indicative per-run cost reporting; a results roll-up across the child-sandbox tree.
- Host MCP proxy: re-expose a curated, read-only subset of the stdio MCP servers from your Claude Code (`DEMESNE_CLAUDE_CODE_MCP_CONFIG`, default `~/.claude.json`) and Codex (`DEMESNE_CODEX_MCP_CONFIG`, default `~/.codex/config.toml`) configs — merged, with Codex winning on name conflicts — to containerised agents through a per-sandbox tunnel.

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

[unreleased]: https://github.com/jbeshir/demesne/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/jbeshir/demesne/compare/v0.1.1...v0.2.0
[0.1.1]: https://github.com/jbeshir/demesne/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/jbeshir/demesne/releases/tag/v0.1.0
