# Demesne configuration reference

## Environment variables

All env vars are read by `demesne-mcp` at startup. Source of truth: `internal/sandbox/config.go` `LoadConfigFromEnv`.

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `DEMESNE_ALLOWED_PATHS` | **yes** | — | Colon-separated list of host paths under which tools may mount files/directories or upload from. Anything outside is rejected. Symlinks are resolved before the containment check. |
| `OPEN_SANDBOX_DOMAIN` | **yes** | — | Host:port of the OpenSandbox lifecycle server (e.g. `localhost:8080`). |
| `OPEN_SANDBOX_API_KEY` | **yes** | — | API key for the OpenSandbox lifecycle server. |
| `OPEN_SANDBOX_PROTOCOL` | no | `http` | `http` or `https`. |
| `DEMESNE_OUTPUT_ROOT` | no | `/tmp/demesne/out` | Host directory under which per-job `/out` mounts are created. |
| `DEMESNE_CLAUDE_CODE_OAUTH_TOKEN` | no* | — | Long-lived Claude Code OAuth token from `claude setup-token`. Required by `sandbox_agent` and `sandbox_research` with the default `claude-code` agent. |
| `DEMESNE_CODEX_AUTH_FILE` | no | `~/.codex/auth.json` | Path to the Codex ChatGPT-OAuth token file (from `codex login`). Required by `sandbox_agent` and `sandbox_research` when called with `agent="codex"`. |
| `DEMESNE_HOST_MCP_CONFIG` | no | `~/.claude.json` | Claude Code MCP config file demesne reads to discover host stdio MCP servers to re-expose. |
| `DEMESNE_MCP_ALLOWLIST` | no | `~/.config/demesne/mcp-allowlist.json` | Per-server tool allowlist override file (auto-seeded with built-in read-only defaults on first run). |
| `DEMESNE_MCP_SOCKET` | no | `/tmp/demesne-mcp/<pid>/aggregator.sock` | Host path of the MCP aggregator unix socket. The runner bind-mounts it into each sandbox sidecar; a unix socket (rather than a host TCP port) is what lets the sandbox reach the aggregator under rootless podman — see [architecture.md](../explanation/architecture.md). |

\* `DEMESNE_CLAUDE_CODE_OAUTH_TOKEN` is optional at the env level but required at runtime when `sandbox_agent` or `sandbox_research` is called with the default `claude-code` agent.

## Container images

`sandbox_script`, `sandbox_create`, and `sandbox_exec` accept an `image` parameter naming one of the four allowlisted images (`internal/sandbox/images.go`):

| Name | Container image |
|------|-----------------|
| `node` | `node:22` |
| `python` | `python:3.12` |
| `go` | `golang:1` (batteries-included: Go toolchain + git + gcc + make) |
| `anaconda` | `continuumio/anaconda3:latest` (default) |

`sandbox_agent` and `sandbox_research` use the agent provider's own image, built locally from an embedded Dockerfile (`demesne-claude-code:<hash>` for the claude-code provider; `demesne-codex:<hash>` for the codex provider).
