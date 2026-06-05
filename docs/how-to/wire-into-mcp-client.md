# Wire demesne into your MCP client

Demesne speaks JSON-RPC over stdio and wires into any MCP-compatible client by pointing the client
at the `demesne-mcp` binary with the required environment variables.

This page centres the two coding-agent CLIs that can drive demesne's full feature set —
**Claude Code** and **Codex** — because demesne's file features (mounting host paths via
`files`/`directories` and returning host `output_dir` paths) only work for a client that runs
locally with host-filesystem access and can choose paths / read results back. Other MCP clients
can still call demesne but are text-only (see [Other MCP clients](#other-mcp-clients) below).

For a step-by-step install walkthrough, see the [Quickstart](../tutorial/quickstart.md).

**Required env vars for all clients:**

| Variable | Notes |
|---|---|
| `OPEN_SANDBOX_DOMAIN` | Host:port of the OpenSandbox lifecycle server (e.g. `localhost:8080`) |
| `OPEN_SANDBOX_API_KEY` | API key for the OpenSandbox server |
| `DEMESNE_ALLOWED_PATHS` | Colon-separated host paths permitted as mount sources |

See the [full environment variable reference](../reference/configuration.md#environment-variables).

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
        "DEMESNE_ALLOWED_PATHS": "/home/username/code"
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
  --env DEMESNE_ALLOWED_PATHS=/home/username/code \
  demesne -- /usr/local/bin/demesne-mcp
```

Keep `--transport stdio` between the last `--env` and the server name `demesne` — `--env` must
not be immediately followed by the server name.

Scope flags: `--scope local` (default, `~/.claude.json`), `--scope project` (`.mcp.json`),
`--scope user` (`~/.claude.json`, all projects).

---

## Codex

Codex (OpenAI's coding-agent CLI) reads MCP servers from `~/.codex/config.toml`. Add a
`[mcp_servers.demesne]` block:

```toml
[mcp_servers.demesne]
command = "/usr/local/bin/demesne-mcp"
args = []
env = { OPEN_SANDBOX_DOMAIN = "localhost:8080", OPEN_SANDBOX_API_KEY = "<your-api-key>", DEMESNE_ALLOWED_PATHS = "/home/username/code" }
```

The transport is inferred from `command` — there is no `type` key.

To forward variables from Codex's own environment instead of hardcoding the values, use
`env_vars` (a TOML array of variable names to pass through from the parent process):

```toml
[mcp_servers.demesne]
command = "/usr/local/bin/demesne-mcp"
args = []
env_vars = ["OPEN_SANDBOX_API_KEY"]
env = { OPEN_SANDBOX_DOMAIN = "localhost:8080", DEMESNE_ALLOWED_PATHS = "/home/username/code" }
```

### `codex mcp add` CLI shortcut

```bash
codex mcp add \
  --env OPEN_SANDBOX_DOMAIN=localhost:8080 \
  --env OPEN_SANDBOX_API_KEY=<key> \
  --env DEMESNE_ALLOWED_PATHS=/home/username/code \
  demesne -- /usr/local/bin/demesne-mcp
```

---

## Other MCP clients

demesne wires into any MCP-compatible client over stdio, but its **file features** — mounting
host paths via `files`/`directories` and returning host `output_dir` paths the caller can open —
need a co-located, filesystem-aware client like Claude Code or Codex on the same host.

File-path-blind clients — for example Claude Desktop, or containerized/remote agents reached
through an MCP proxy — can still run sandboxed work and receive the text result (stdout, stderr,
cost summary), but can't mount their own files or open the returned `output_dir` unless they're
paired with a filesystem MCP server on the same host that can do that for them.

Config-location pointers for two common file-blind clients:

- **Claude Desktop** — `claude_desktop_config.json` (macOS: `~/Library/Application Support/Claude/`,
  Windows: `%APPDATA%\Claude\`). Same `mcpServers` JSON shape as Claude Code; no `type` key
  required.
- **VS Code** — `.vscode/mcp.json` with a `servers` key (not `mcpServers`); supports `inputs` for
  prompting for secrets.

---

## Environment variables

All env vars are read by `demesne-mcp` at startup from `internal/sandbox/config.go`.

For the full table, see [docs/reference/configuration.md](../reference/configuration.md#environment-variables).

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
