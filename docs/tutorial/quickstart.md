# Quickstart: your first `sandbox_script` call in ≤5 steps

This tutorial takes you from a clean machine to a successful `sandbox_script` call wired through Claude Code in five copy-pasteable steps. By the end you will have demesne running as a stdio MCP server and Claude Code invoking it to run a shell command inside a disposable container.

## Step 1 — Install demesne

### Option A: `go install` (requires Go 1.26+)

```bash
go install github.com/jbeshir/demesne/cmd/demesne-mcp@latest
```

The binary lands in `$(go env GOPATH)/bin/demesne-mcp` (typically `~/go/bin/demesne-mcp`).

### Option B: Download a release binary

Pre-built binaries for `linux/amd64`, `darwin/amd64`, `darwin/arm64`, and `windows/amd64` are published on the [GitHub releases page](https://github.com/jbeshir/demesne/releases). Download the archive for your platform, extract it, and place `demesne-mcp` (or `demesne-mcp.exe` on Windows) somewhere on your `PATH`.

```bash
# Example for linux/amd64 — replace VERSION with the latest release tag
VERSION=v0.5.0
curl -L "https://github.com/jbeshir/demesne/releases/download/${VERSION}/demesne-mcp_${VERSION#v}_linux_amd64.tar.gz" \
  | tar xz -C /usr/local/bin demesne-mcp
```

#### Expected output

```
$ demesne-mcp --help
# (prints usage; exact text varies by release)
```

---

## Step 2 — Run a local OpenSandbox

Demesne delegates container lifecycle to [OpenSandbox](https://github.com/alibaba/OpenSandbox). The reference server runs locally against Docker:

```
pipx install uv
uvx opensandbox-server init-config ~/.sandbox.toml --example docker
uvx opensandbox-server --config ~/.sandbox.toml
```

Feed the lifecycle host:port and API key to Demesne via `OPEN_SANDBOX_DOMAIN`
and `OPEN_SANDBOX_API_KEY`.

### Required `~/.sandbox.toml` edits

The packaged docker example defaults are too permissive for use as a security
boundary. Change two settings before starting the server:

- **`[egress] mode = "dns+nft"`** (default is `"dns"`). The default only
  filters egress at DNS lookup; raw-IP outbound traffic still succeeds, so
  `egress: "none"` in `sandbox_script` does not actually deny network. The
  `dns+nft` mode adds nftables-based IP filtering and makes `none` mean
  none.
- **`[server] api_key = "<some-secret>"`** (default is empty). With an empty
  key, the server requires either an interactive `YES` at startup or
  `OPENSANDBOX_INSECURE_SERVER=YES` in the environment.
- **`[storage] allowed_host_paths = ["/tmp", "/home/<you>/code"]`** (or
  whichever directories you want bind-mountable). The example sets `[]`
  with a comment saying "all paths allowed", but empirically empty means
  *nothing* is allowed — every bind mount fails with
  `VOLUME::HOST_PATH_NOT_ALLOWED`. Both OpenSandbox's allowlist and
  demesne's `DEMESNE_ALLOWED_PATHS` must include each host path you
  intend to mount.

#### Expected output

```
$ uvx opensandbox-server --config ~/.sandbox.toml
INFO  Listening on :8080
```

---

## Step 3 — Set env vars and start `demesne-mcp`

At minimum you need the three required variables from the [Configuration table](../../README.md#configuration):

```bash
export OPEN_SANDBOX_DOMAIN=localhost:8080
export OPEN_SANDBOX_API_KEY=your-secret-key
export DEMESNE_ALLOWED_PATHS=/tmp
```

Then verify the binary starts and speaks JSON-RPC over stdio:

```bash
demesne-mcp
```

Leave it running in a terminal or let the MCP client manage the process (see Step 4).

#### Expected output

```
# (demesne-mcp blocks, waiting for JSON-RPC on stdin)
```

No output on startup is correct — it is waiting for a client.

---

## Step 4 — Wire into Claude Code

Create or edit `.mcp.json` in your project root (this is the project-scoped MCP config committed to git) with one entry for demesne:

```json
{
  "mcpServers": {
    "demesne": {
      "type": "stdio",
      "command": "/usr/local/bin/demesne-mcp",
      "args": [],
      "env": {
        "DEMESNE_ALLOWED_PATHS": "/tmp",
        "OPEN_SANDBOX_DOMAIN": "localhost:8080",
        "OPEN_SANDBOX_API_KEY": "your-secret-key"
      }
    }
  }
}
```

Replace `/usr/local/bin/demesne-mcp` with the actual path from Step 1 (e.g. `~/go/bin/demesne-mcp`). Claude Code will spawn `demesne-mcp` as a child process and communicate over stdio.

For Claude Desktop and VS Code MCP config variants, see [Wire demesne into your MCP client](../how-to/wire-into-claude-code.md).

#### Expected output

In Claude Code, after reloading the MCP config (restart Claude Code or run `/mcp`):

```
✓ demesne  connected
```

---

## Step 5 — Make a `sandbox_script` call

In a Claude Code session, ask Claude to run a command:

```
Use the sandbox_script tool with command "echo hello && uname -a"
```

Or invoke the tool directly from the Claude Code developer console:

```
tools/call sandbox_script command="echo hello && uname -a"
```

#### Expected output

```
exit_code: 0
output_dir: /tmp/demesne/out/<job-id>/out
job_id: <uuid>
---
hello
Linux <container-hostname> 6.x.x ... x86_64 GNU/Linux
```

The command ran inside a disposable `continuumio/anaconda3` container (the default image). The `/tmp/demesne/out/<job-id>/out` directory on your host contains any files the command wrote to `/out` inside the sandbox.

---

## What next?

- **How-to guides** — [`../how-to/`](../how-to/) covers sharing host directories, egress control, spawning nested agents, and more.
- **Tool reference** — [`../reference/tools/`](../reference/tools/) has the full parameter tables, sample requests, and error tables for all 8 tools.
- **Concepts** — [`../explanation/`](../explanation/) explains the architecture, trust boundary, and key concepts in depth.
