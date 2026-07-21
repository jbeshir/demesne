# Demesne configuration reference

## Environment variables

All env vars are read by `demesne-mcp` at startup. Source of truth: `internal/sandbox/config.go` `LoadConfigFromEnv`.

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `DEMESNE_ALLOWED_PATHS` | **yes** | — | Colon-separated list of host paths under which tools may mount files/directories or upload from. Anything outside is rejected. Symlinks are resolved before the containment check. The effective list also always includes DEMESNE_OUTPUT_ROOT (see below). |
| `OPEN_SANDBOX_DOMAIN` | **yes** | — | Host:port of the OpenSandbox lifecycle server (e.g. `localhost:8080`). |
| `OPEN_SANDBOX_API_KEY` | **yes** | — | API key for the OpenSandbox lifecycle server. |
| `OPEN_SANDBOX_PROTOCOL` | no | `http` | `http` or `https`. |
| `DEMESNE_OUTPUT_ROOT` | no | `~/.demesne/out` | Host directory under which per-job `/out` mounts are created. |
| `DEMESNE_CODEX_ENABLED` | no | `true` | Enables the OpenAI Codex agent provider. Accepts Go boolean syntax (`true`/`false`, `1`/`0`, `t`/`f`, including accepted case variants); invalid values fail startup. When false, Codex is excluded from resolution and advertised models even if credentials exist. |
| `DEMESNE_CLAUDE_CODE_ENABLED` | no | `true` | Enables the Anthropic Claude Code agent provider. Uses the same Go boolean syntax and startup validation. When false, Claude Code is excluded from resolution and advertised models even if credentials exist. |
| `DEMESNE_CODEX_AUTH_FILE` | no* | `~/.codex/auth.json` | Path to the Codex ChatGPT-OAuth token file (from `codex login`). Used by `sandbox_agent` and `sandbox_research`; when Codex is enabled and this file exists, demesne prefers Codex as the default agent. Required when an enabled Codex model is specified. |
| `DEMESNE_CLAUDE_CODE_OAUTH_TOKEN` | no* | — | Long-lived Claude Code OAuth token from `claude setup-token`. Used by `sandbox_agent` and `sandbox_research`. Required when an enabled Claude Code model is specified, or when Claude Code is the first enabled provider with configured credentials. |
| `DEMESNE_CLAUDE_CODE_MCP_CONFIG` | no | `~/.claude.json` | Claude Code MCP config file demesne reads to discover host stdio MCP servers to re-expose. |
| `DEMESNE_CODEX_MCP_CONFIG` | no | `~/.codex/config.toml` | Codex MCP config file demesne reads to discover host stdio MCP servers; merged with the Claude Code config, Codex wins on name conflict. Also honours `env_vars` (parent-process env-var names forwarded into the server's environment). |
| `DEMESNE_MCP_ALLOWLIST` | no | `~/.config/demesne/mcp-allowlist.json` | Per-server tool allowlist override file (auto-seeded with built-in read-only defaults on first run). |
| `DEMESNE_MCP_SOCKET` | no | `/tmp/demesne-mcp/<pid>/aggregator.sock` | Host path of the MCP aggregator unix socket. The runner bind-mounts it into each sandbox sidecar; a unix socket (rather than a host TCP port) is what lets the sandbox reach the aggregator under rootless podman — see [architecture.md](../explanation/architecture.md). |

\* `DEMESNE_CLAUDE_CODE_OAUTH_TOKEN` and `DEMESNE_CODEX_AUTH_FILE` are both optional at the env level. `sandbox_agent` and `sandbox_research` require whichever credential matches the resolved provider at runtime: when no model is specified, demesne prefers an enabled, configured `codex` provider and otherwise uses an enabled, configured `claude-code` provider. If no enabled provider has credentials, the call reports the setup requirement for the first enabled provider in Codex-first order.

The output root is always appended to the effective mount allowlist, so /out and nested /in/previous-jobs/<name> mounts work without the user listing the output root in DEMESNE_ALLOWED_PATHS.

## Agent providers

`sandbox_agent` and `sandbox_research` run one of two coding-agent providers in the sandbox:

- **Codex** (preferred default when enabled and configured). Authenticate with `codex login` (the OpenAI Codex CLI); point demesne at the resulting `auth.json` via `DEMESNE_CODEX_AUTH_FILE` (default `~/.codex/auth.json`).
- **Claude Code** (fallback default when enabled and configured). Produce a long-lived token with `claude setup-token`; export it as `DEMESNE_CLAUDE_CODE_OAUTH_TOKEN`.

When no model is specified, demesne picks the first enabled provider with configured credentials, in Codex-first order. The Codex default model is `gpt-5.6-sol`; `gpt-5.6-terra`, `gpt-5.6-luna`, `gpt-5.5`, and `gpt-5.4-mini` remain selectable. If no enabled provider has credentials, demesne reports the credential requirement for the first enabled provider in the same order. If neither provider is enabled, it reports that no agent providers are enabled.

Providers can be disabled independently with `DEMESNE_CODEX_ENABLED=false` or `DEMESNE_CLAUDE_CODE_ENABLED=false`. A disabled provider is never selected as the default, is absent from the live `model` enum, and an explicit request for one of its models returns an unavailable error before credentials are read or the provider is invoked. If both providers are disabled, agent calls fail because no provider is enabled. These controls do not disable host MCP-config discovery.

`sandbox_agent` and `sandbox_research` advertise the `model` enum in their MCP input schema filtered to the union of enabled providers with configured credentials (Codex-first). When none are both enabled and configured, the enum is omitted and the tools report the applicable error at call time.

## Container images

`sandbox_script`, `sandbox_create`, and `sandbox_exec` accept an `image` parameter naming one of the eight allowlisted images (`internal/sandbox/images.go`):

| Name | Container image |
|------|-----------------|
| `node` | `node:22` |
| `python` | `python:3.12` |
| `go` | `golang:1` (batteries-included: Go toolchain + git + gcc + make) |
| `anaconda` | `continuumio/anaconda3:latest` (default) |
| `browser` | demesne-built from an embedded Dockerfile, like the agent images (Playwright + Chromium/Firefox/WebKit + Node); rendering works at `egress=none` |
| `media` | demesne-built from an embedded Dockerfile, like the agent images (ffmpeg + ImageMagick + libvips + audio tooling for video/audio/image conversion) |
| `twine` | demesne-built from an embedded Dockerfile, like the agent images (Tweego + Twine story formats + Chromium); offline interactive-fiction build/playtest works at `egress=none` |
| `webgamedev` | demesne-built from an embedded Dockerfile, like the agent images (a warm Phaser + Vite + TypeScript template + Chromium); offline HTML5-game build/playtest works at `egress=none` |

`sandbox_agent` and `sandbox_research` use the agent provider's own image, built locally from an embedded Dockerfile (`demesne-claude-code:<hash>` for the claude-code provider; `demesne-codex:<hash>` for the codex provider).

Like the agent images, `browser`, `media`, `twine`, and `webgamedev` are each built once on the host from a fixed embedded Dockerfile and cached (`demesne-browser:<hash>`, `demesne-media:<hash>`, `demesne-twine:<hash>`, and `demesne-webgamedev:<hash>`) — agents select the image but don't configure or trigger its build. Because sandboxes are always created host-side, the cached images are equally available to nested in-sandbox callers, which is what lets in-sandbox pipelines (for example, end-to-end React development) render their own React content in a `browser` sandbox. The `twine` and `webgamedev` images share the `browser` image's Playwright/Chromium base layer, so podman caches it once across all three.
