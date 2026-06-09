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
| `DEMESNE_CODEX_AUTH_FILE` | no* | `~/.codex/auth.json` | Path to the Codex ChatGPT-OAuth token file (from `codex login`). Used by `sandbox_agent` and `sandbox_research`; when present, demesne prefers Codex as the default agent. Required when calling either tool with `agent="codex"`. |
| `DEMESNE_CLAUDE_CODE_OAUTH_TOKEN` | no* | — | Long-lived Claude Code OAuth token from `claude setup-token`. Used by `sandbox_agent` and `sandbox_research`. Required when calling either tool with `agent="claude-code"` or when no Codex auth file is configured. |
| `DEMESNE_CLAUDE_CODE_MCP_CONFIG` | no | `~/.claude.json` | Claude Code MCP config file demesne reads to discover host stdio MCP servers to re-expose. |
| `DEMESNE_CODEX_MCP_CONFIG` | no | `~/.codex/config.toml` | Codex MCP config file demesne reads to discover host stdio MCP servers; merged with the Claude Code config, Codex wins on name conflict. Also honours `env_vars` (parent-process env-var names forwarded into the server's environment). |
| `DEMESNE_MCP_ALLOWLIST` | no | `~/.config/demesne/mcp-allowlist.json` | Per-server tool allowlist override file (auto-seeded with built-in read-only defaults on first run). |
| `DEMESNE_MCP_SOCKET` | no | `/tmp/demesne-mcp/<pid>/aggregator.sock` | Host path of the MCP aggregator unix socket. The runner bind-mounts it into each sandbox sidecar; a unix socket (rather than a host TCP port) is what lets the sandbox reach the aggregator under rootless podman — see [architecture.md](../explanation/architecture.md). |

\* `DEMESNE_CLAUDE_CODE_OAUTH_TOKEN` and `DEMESNE_CODEX_AUTH_FILE` are both optional at the env level. `sandbox_agent` and `sandbox_research` require whichever credential matches the resolved agent at runtime: when the `agent` parameter is omitted, demesne prefers `codex` if its auth file exists and falls back to `claude-code` if only that token is set.

The output root is always appended to the effective mount allowlist, so /out and nested /in/previous-jobs/<name> mounts work without the user listing the output root in DEMESNE_ALLOWED_PATHS.

## Agent providers

`sandbox_agent` and `sandbox_research` run one of two coding-agent providers in the sandbox:

- **Codex** (preferred default). Authenticate with `codex login` (the OpenAI Codex CLI); point demesne at the resulting `auth.json` via `DEMESNE_CODEX_AUTH_FILE` (default `~/.codex/auth.json`). When this file exists, demesne uses Codex by default.
- **Claude Code** (fallback default). Produce a long-lived token with `claude setup-token`; export it as `DEMESNE_CLAUDE_CODE_OAUTH_TOKEN`. Used by default when Codex is not configured.

When the `agent` parameter is omitted demesne picks Codex if its credentials are configured, otherwise Claude Code; explicit `agent="codex"` or `agent="claude-code"` always wins. Configuring both providers is fine — Codex is still preferred. Set neither and demesne errors with a Codex setup-path message.

`sandbox_agent` and `sandbox_research` advertise the `agent` / `model` enums in their MCP input schema filtered to the providers whose credentials are configured (codex-first; when neither is configured the enum is omitted and the tools error at call time).

## Container images

`sandbox_script`, `sandbox_create`, and `sandbox_exec` accept an `image` parameter naming one of the four allowlisted images (`internal/sandbox/images.go`):

| Name | Container image |
|------|-----------------|
| `node` | `node:22` |
| `python` | `python:3.12` |
| `go` | `golang:1` (batteries-included: Go toolchain + git + gcc + make) |
| `anaconda` | `continuumio/anaconda3:latest` (default) |
| `browser` | demesne-built from an embedded Dockerfile, like the agent images (Playwright + Chromium/Firefox/WebKit + Node); rendering works at `egress=none` |
| `media` | demesne-built from an embedded Dockerfile, like the agent images (ffmpeg + ImageMagick + libvips + audio tooling for video/audio/image conversion) |

`sandbox_agent` and `sandbox_research` use the agent provider's own image, built locally from an embedded Dockerfile (`demesne-claude-code:<hash>` for the claude-code provider; `demesne-codex:<hash>` for the codex provider).

Like the agent images, `browser` and `media` are each built once on the host from a fixed embedded Dockerfile and cached (`demesne-browser:<hash>` and `demesne-media:<hash>`) — agents select the image but don't configure or trigger its build. Because sandboxes are always created host-side, the cached images are equally available to nested in-sandbox callers, which is what lets in-sandbox pipelines (for example, end-to-end React development) render their own React content in a `browser` sandbox.
