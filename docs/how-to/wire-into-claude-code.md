# Wire demesne into your MCP client

Demesne speaks JSON-RPC over stdio and wires into any MCP-compatible client by pointing the client
at the `demesne-mcp` binary with the required environment variables. This page covers Claude Code,
Claude Desktop, and VS Code. For a step-by-step install walkthrough, see the
[Quickstart](../tutorial/quickstart.md).

**Required env vars for all clients:**

| Variable | Notes |
|---|---|
| `OPEN_SANDBOX_DOMAIN` | Host:port of the OpenSandbox lifecycle server (e.g. `localhost:8080`) |
| `OPEN_SANDBOX_API_KEY` | API key for the OpenSandbox server |
| `DEMESNE_ALLOWED_PATHS` | Colon-separated host paths permitted as mount sources |

See the [full environment variable reference](#environment-variables) below.

---

## Claude Code

Claude Code loads MCP servers from `.mcp.json` in your project root (project scope, committed to
git) or from `~/.claude.json` (local/user scope, private).

### `.mcp.json` (project scope — recommended)

Create or edit `.mcp.json` in your project root:

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

Replace `/usr/local/bin/demesne-mcp` with the actual path to the binary (e.g. `~/go/bin/demesne-mcp`).

### `claude mcp add` CLI shortcut

```bash
# Add to project scope (writes .mcp.json):
claude mcp add --transport stdio --scope project demesne -- /usr/local/bin/demesne-mcp

# Then set env vars manually in .mcp.json, or pass them inline:
claude mcp add --transport stdio \
  --env OPEN_SANDBOX_DOMAIN=localhost:8080 \
  --env OPEN_SANDBOX_API_KEY=<key> \
  --env DEMESNE_ALLOWED_PATHS=/home/you/code \
  demesne -- /usr/local/bin/demesne-mcp
```

Scope flags: `--scope local` (default, `~/.claude.json`), `--scope project` (`.mcp.json`),
`--scope user` (`~/.claude.json`, all projects).

---

## Claude Desktop

Claude Desktop reads `claude_desktop_config.json`. Note that `type` is **not** required here —
stdio is implied.

**File locations:**

| Platform | Path |
|---|---|
| macOS | `~/Library/Application Support/Claude/claude_desktop_config.json` |
| Windows | `%APPDATA%\Claude\claude_desktop_config.json` |

Add a `demesne` entry under `mcpServers`:

```json
{
  "mcpServers": {
    "demesne": {
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

On macOS the binary path is typically `~/go/bin/demesne-mcp` or wherever `make build` placed it.
Restart Claude Desktop after editing this file.

---

## VS Code

VS Code uses `.vscode/mcp.json` with a `servers` key (not `mcpServers`). The `inputs` array lets
you prompt for secrets rather than hard-coding them.

Create or edit `.vscode/mcp.json` in your project:

```json
{
  "inputs": [
    {
      "type": "promptString",
      "id": "opensandbox-api-key",
      "description": "OpenSandbox API key for demesne",
      "password": true
    }
  ],
  "servers": {
    "demesne": {
      "type": "stdio",
      "command": "/usr/local/bin/demesne-mcp",
      "args": [],
      "env": {
        "OPEN_SANDBOX_DOMAIN": "localhost:8080",
        "OPEN_SANDBOX_API_KEY": "${input:opensandbox-api-key}",
        "DEMESNE_ALLOWED_PATHS": "/home/you/code:/tmp/demesne-test"
      }
    }
  }
}
```

VS Code prompts you for the API key on first use and stores it securely. You can also use `envFile`
instead of `inputs` to load a `.env` file.

---

## Environment variables

All env vars are read by `demesne-mcp` at startup from `internal/sandbox/config.go`:

| Variable | Required | Default | Description |
|---|---|---|---|
| `DEMESNE_ALLOWED_PATHS` | **yes** | — | Colon-separated host paths under which tools may mount files/directories or upload from. Symlinks are resolved before the containment check. |
| `OPEN_SANDBOX_DOMAIN` | **yes** | — | Host:port of the OpenSandbox lifecycle server (e.g. `localhost:8080`). |
| `OPEN_SANDBOX_API_KEY` | **yes** | — | API key for the OpenSandbox lifecycle server. |
| `OPEN_SANDBOX_PROTOCOL` | no | `http` | `http` or `https`. |
| `DEMESNE_OUTPUT_ROOT` | no | `/tmp/demesne/out` | Host directory under which per-job `/out` mounts are created. |
| `DEMESNE_CLAUDE_CODE_OAUTH_TOKEN` | no* | — | Long-lived Claude Code OAuth token from `claude setup-token`. **Required by `sandbox_agent` and `sandbox_research`**; other tools work without it. |
| `DEMESNE_CODEX_AUTH_FILE` | no | `~/.codex/auth.json` | Path to the Codex ChatGPT-OAuth token file (from `codex login`). Required only when using `agent="codex"` with `sandbox_agent` or `sandbox_research`. |
| `DEMESNE_HOST_MCP_CONFIG` | no | `~/.claude.json` | Claude Code MCP config file demesne reads to discover host stdio MCP servers to re-expose. |
| `DEMESNE_MCP_ALLOWLIST` | no | `~/.config/demesne/mcp-allowlist.json` | Per-server tool allowlist override file (auto-seeded with built-in read-only defaults on first run). |
| `DEMESNE_MCP_SOCKET` | no | `/tmp/demesne-mcp/<pid>/aggregator.sock` | Host path of the MCP aggregator unix socket. The runner bind-mounts it into each sandbox sidecar. |

\* `DEMESNE_CLAUDE_CODE_OAUTH_TOKEN` is optional at the env level but required at runtime when
`sandbox_agent` or `sandbox_research` is called.

---

## Verifying the connection

After wiring demesne into your client, ask it to list available tools. In Claude Code, run:

```
/mcp
```

or open the MCP tools panel; you should see the eight demesne tools (`sandbox_script`,
`sandbox_create`, `sandbox_exec`, `sandbox_upload`, `sandbox_download`, `sandbox_destroy`,
`sandbox_agent`, `sandbox_research`).

If the connection fails, check that `demesne-mcp` is executable at the configured path, that
`OPEN_SANDBOX_DOMAIN` points at a running OpenSandbox server, and that `DEMESNE_ALLOWED_PATHS`
contains at least one valid host path. The process writes startup errors to stderr, which most MCP
clients surface in their logs.
