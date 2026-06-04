# Demesne

<!-- mcp-name: io.github.jbeshir/demesne-mcp -->

A local-first MCP server that runs untrusted shell, scripts, and AI coding agents in disposable containers, with read-only host mounts and egress allowlists.

[![Go Version](https://img.shields.io/github/go-mod/go-version/jbeshir/demesne)](https://github.com/jbeshir/demesne)
[![License](https://img.shields.io/github/license/jbeshir/demesne)](LICENSE)
[![Latest Release](https://img.shields.io/github/v/release/jbeshir/demesne)](https://github.com/jbeshir/demesne/releases/latest)
[![CI](https://github.com/jbeshir/demesne/actions/workflows/ci.yml/badge.svg)](https://github.com/jbeshir/demesne/actions/workflows/ci.yml)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/jbeshir/demesne/badge)](https://scorecard.dev/viewer/?uri=github.com/jbeshir/demesne)

For developers running a single local AI coding agent and wanting a hardened sandbox boundary for untrusted code, scripts, and tool use.

> [!WARNING]
> **Alpha — best-effort.** demesne is early software, and is largely built using itself (its own sandboxed agents do much of the work). Expect rough edges, gaps, and breaking changes between versions. Treat it as alpha and best-effort, and review what it does before relying on it.

### Wire into Claude Code in ~30 seconds

```json
{
  "mcpServers": {
    "demesne": {
      "type": "stdio",
      "command": "/usr/local/bin/demesne-mcp",
      "args": [],
      "env": {
        "OPEN_SANDBOX_DOMAIN": "localhost:8080",
        "OPEN_SANDBOX_API_KEY": "<your-api-key>",
        "DEMESNE_ALLOWED_PATHS": "/home/you/code:/tmp/demesne-test"
      }
    }
  }
}
```

`sandbox_agent` and `sandbox_research` additionally need `DEMESNE_CLAUDE_CODE_OAUTH_TOKEN` (from `claude setup-token`) for the default `claude-code` agent, or `DEMESNE_CODEX_AUTH_FILE` (from `codex login`) for `agent="codex"` — see [docs/how-to/wire-into-claude-code.md](docs/how-to/wire-into-claude-code.md) for the full env reference.

See [docs/how-to/wire-into-claude-code.md](docs/how-to/wire-into-claude-code.md) for Claude Desktop, VS Code, and full env var reference.

### Read more

| | |
|---|---|
| 🚀 [Quickstart](docs/tutorial/quickstart.md) | Five steps to your first `sandbox_script` call |
| 📚 [Docs](docs/) | Tutorials, how-to recipes, reference, explanation |
| 🧪 [Examples](examples/) | Runnable example calls |

## Requirements

Demesne embeds a `linux/amd64` helper binary that runs inside every sandbox container, so the host's container runtime must be able to execute `linux/amd64` containers. This is the standard path on:

- **linux/amd64** — native.
- **darwin/amd64** and **windows/amd64** — via the Docker/Podman Machine Linux VM.
- **darwin/arm64 (Apple Silicon)** — via Rosetta, which Docker Desktop enables by default and Podman supports via `podman machine init --rosetta`.

Releases are published for `linux/amd64`, `darwin/amd64`, `darwin/arm64`, and `windows/amd64`. **Only `linux/amd64` is actively tested**; the other platforms build cleanly but are best-effort. linux/arm64 is reachable with `qemu-user-static` binfmt but no native binary is shipped.

Building from source via `go install` requires Go 1.26+ (see [docs/reference/requirements.md](docs/reference/requirements.md) for the full host-prerequisites checklist).

## Status

See [CHANGELOG.md](CHANGELOG.md) for milestone history.

## Tools

| Tool               | Description                                                                                                                                                                                                                                | Reference |
|--------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----|
| `sandbox_script`   | Run a shell command in a fresh sandbox and tear it down. Returns exit code, stdout, stderr, and the `/out` host path.                                                                                                                     | [ref](docs/reference/tools/sandbox_script.md) |
| `sandbox_create`   | Create a persistent sandbox. Returns a `sandbox_id` handle and the `/out` host path. TTL is 24h, refreshed by each `sandbox_exec`.                                                                                                          | [ref](docs/reference/tools/sandbox_create.md) |
| `sandbox_exec`     | Run a shell command in an existing sandbox. Refreshes TTL. Returns exit code, stdout, and stderr.                                                                                                                                           | [ref](docs/reference/tools/sandbox_exec.md) |
| `sandbox_upload`   | Copy a host file into an existing sandbox.                                                                                                                                                                                                  | [ref](docs/reference/tools/sandbox_upload.md) |
| `sandbox_download` | Copy a file out of an existing sandbox; written under `<output_dir>/downloads/<basename>`. Returns the host path.                                                                                                                           | [ref](docs/reference/tools/sandbox_download.md) |
| `sandbox_destroy`  | Kill an existing sandbox. Host output dir is preserved.                                                                                                                                                                                     | [ref](docs/reference/tools/sandbox_destroy.md) |
| `sandbox_agent`    | Run an AI coding agent (`claude-code` by default, or `codex` — experimental) in a fresh sandbox against a caller-supplied prompt. Outbound HTTPS is restricted to the vendor proxy. Returns exit code, stdout, stderr, the `/out` host path, and the (indicative) cost summary.              | [ref](docs/reference/tools/sandbox_agent.md) |
| `sandbox_research` | Run a long-running research agent with no input mounts and unrestricted outbound internet access. Returns exit code, stdout, stderr, the `/out` host path, and the (indicative) cost summary.                                                     | [ref](docs/reference/tools/sandbox_research.md) |

For a step-by-step walkthrough of the persistent-sandbox lifecycle, see the [Quickstart](docs/tutorial/quickstart.md) and the [`sandbox_create`](docs/reference/tools/sandbox_create.md) / [`sandbox_exec`](docs/reference/tools/sandbox_exec.md) reference pages.

## Configuration

| Environment variable      | Required | Default               | Description                                                                                                       |
|---------------------------|----------|-----------------------|-------------------------------------------------------------------------------------------------------------------|
| `DEMESNE_ALLOWED_PATHS`  | yes      |                       | Colon-separated list of host paths under which tools may mount files/directories or upload from. Anything outside is rejected. Symlinks are resolved before the containment check. |
| `DEMESNE_OUTPUT_ROOT`    | no       | `/tmp/demesne/out`   | Host directory under which per-job `/out` mounts are created.                                                     |
| `OPEN_SANDBOX_DOMAIN`     | yes      |                       | Host:port of the OpenSandbox lifecycle server (e.g. `localhost:8080`).                                            |
| `OPEN_SANDBOX_PROTOCOL`   | no       | `http`                | `http` or `https`.                                                                                                |
| `OPEN_SANDBOX_API_KEY`    | yes      |                       | API key for the OpenSandbox lifecycle server.                                                                     |
| `DEMESNE_CLAUDE_CODE_OAUTH_TOKEN` | no | | Long-lived Claude Code OAuth token, generated by running `claude setup-token` on the host. Required by `sandbox_agent` and `sandbox_research`; other tools work without it. |
| `DEMESNE_CODEX_AUTH_FILE` | no | `~/.codex/auth.json` | Path to the Codex ChatGPT-OAuth token file (written by `codex login`) for the Codex provider (experimental). The proxy holds and refreshes this token set off-agent. Required when `sandbox_agent` or `sandbox_research` is invoked with `agent="codex"`; other tools and providers work without it. |
| `DEMESNE_HOST_MCP_CONFIG` | no | `~/.claude.json` | Path to the Claude Code MCP config demesne reads to discover host stdio MCP servers. |
| `DEMESNE_MCP_ALLOWLIST`  | no | `~/.config/demesne/mcp-allowlist.json` | Path to the per-server tool allowlist override file (auto-seeded with built-in read-only defaults on first run). |
| `DEMESNE_MCP_SOCKET`     | no | `/tmp/demesne-mcp/<pid>/aggregator.sock` | Host path of the MCP aggregator's unix socket. The runner bind-mounts it into each sandbox sidecar; a socket (not a host TCP port) is what lets the sandbox reach the aggregator under rootless podman. |

Full reference: [docs/reference/configuration.md](docs/reference/configuration.md).

## Run a local OpenSandbox

The reference OpenSandbox server runs locally against Docker:

```
pipx install uv
uvx opensandbox-server init-config ~/.sandbox.toml --example docker
uvx opensandbox-server --config ~/.sandbox.toml
```

Feed the lifecycle host:port and API key to Demesne via `OPEN_SANDBOX_DOMAIN`
and `OPEN_SANDBOX_API_KEY`.

See [docs/reference/requirements.md §OpenSandbox configuration](docs/reference/requirements.md#opensandbox-configuration) for the required `~/.sandbox.toml` edits.

## Build and run

```
make build
DEMESNE_ALLOWED_PATHS=/tmp/demesne-test \
  OPEN_SANDBOX_DOMAIN=localhost:8080 \
  OPEN_SANDBOX_API_KEY=... \
  ./bin/demesne-mcp
```

The binary speaks JSON-RPC over stdio. Wire it into Claude Code's MCP config (or any MCP client) to invoke `sandbox_script`.

## Validation

```
make lint
make test-short
make build
```

Integration tests in `internal/sandbox/runner_integration_test.go` drive
a real OpenSandbox end-to-end. They live behind the `integration` build
tag, so the default test path doesn't touch them. To run them:

```
make setup-files     # one-off: copies .env.dist to .env
$EDITOR .env         # fill in OPEN_SANDBOX_API_KEY
make test-integration
```

`make setup-tools` installs the `godotenv` CLI that `test-integration`
uses to load `.env`.

The integration suite covers: the `/out` mount round-trip; `egress: "none"`
blocks both DNS and raw-IP egress; `egress: "package-managers"` allows
pypi.org; the full persistent-sandbox lifecycle
(create / exec / upload / exec / download / destroy); and that
`sandbox_exec` refreshes the sandbox TTL. The raw-IP assertion requires
the `[egress] mode = "dns+nft"` config in `~/.sandbox.toml` (see the
[Quickstart](docs/tutorial/quickstart.md#step-2-run-a-local-opensandbox));
against a `mode = "dns"` server it will fail.
