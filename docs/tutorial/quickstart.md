# Quickstart: your first `sandbox_script` call in ≤5 steps

This tutorial takes you from a clean machine to a successful `sandbox_script` call wired through Claude Code in five copy-pasteable steps. By the end you will have demesne running as a stdio MCP server and Claude Code invoking it to run a shell command inside a disposable container.

## Prerequisites

demesne runs your workloads as local containers via OpenSandbox, so two host requirements come first — settle them before installing OpenSandbox:

- **Platform** — your host must be able to run `linux/amd64` containers: native on `linux/amd64`, or via a Docker/Podman Machine VM (Rosetta on Apple Silicon) on macOS and Windows. Only `linux/amd64` is actively tested.
- **Container runtime** — install **Docker or Podman**. Rootless Podman, serving the Docker-compatible API, is supported and is the tested setup.
- **Rootless Podman only** — use cgroup v2, and set `fs.pipe-user-pages-soft=0` (a fan-out of concurrent containers exceeds the default pipe-page cap): `sudo sysctl -w fs.pipe-user-pages-soft=0`.

See [docs/reference/requirements.md](../reference/requirements.md) for the full host checklist. Everything below assumes a working container runtime.

---

## Step 1: Install demesne

### Option A: Download a release binary (recommended)

Pre-built binaries for `linux/amd64`, `darwin/amd64`, `darwin/arm64`, and `windows/amd64` are published on the [GitHub releases page](https://github.com/jbeshir/demesne/releases). Download the archive for your platform, extract it, and place `demesne-mcp` (or `demesne-mcp.exe` on Windows) somewhere on your `PATH`. No Go toolchain required.

```bash
# Example for linux/amd64 — replace VERSION with the latest release tag
VERSION=v0.5.0
curl -L "https://github.com/jbeshir/demesne/releases/download/${VERSION}/demesne-mcp_${VERSION#v}_linux_amd64.tar.gz" \
  | tar xz -C /usr/local/bin demesne-mcp
```

### Option B: Build from source (requires Go 1.26+)

```bash
go install github.com/jbeshir/demesne/cmd/demesne-mcp@latest
```

The binary lands in `$(go env GOPATH)/bin/demesne-mcp` (typically `~/go/bin/demesne-mcp`).

See [CONTRIBUTING.md](../../CONTRIBUTING.md) for the local `make build` development flow.

#### Expected output

```
$ demesne-mcp --help
# (prints usage; exact text varies by release)
```

---

## Step 2: Run a local OpenSandbox

Demesne delegates container lifecycle to [OpenSandbox](https://github.com/alibaba/OpenSandbox). The reference server runs locally against Docker or Podman. Install it and generate a config:

```
pipx install uv
uvx opensandbox-server init-config ~/.sandbox.toml --example docker
```

The `init-config` defaults are too permissive to use as a security boundary, so edit these three settings in `~/.sandbox.toml` before starting the server:

```toml
[server]
# Any non-empty value. Reuse it as OPEN_SANDBOX_API_KEY when you wire demesne in (Steps 3–4).
api_key = "your-secret-key"

[storage]
# Host paths demesne may bind-mount: the directories you want to share, plus
# demesne's output root (default ~/.demesne/out).
allowed_host_paths = ["/home/username/code", "/home/username/.demesne/out"]

[egress]
# Adds nftables IP-level filtering, so `egress: "none"` actually denies the network.
mode = "dns+nft"
```

`allowed_host_paths` is the setting that bites if you skip it — bind mounts then fail with `VOLUME::HOST_PATH_NOT_ALLOWED`. See [requirements.md](../reference/requirements.md) for the full checklist.

Then start the server:

```
uvx opensandbox-server --config ~/.sandbox.toml
```

OpenSandbox is **long-running** — it must stay up for the entire demesne session. The remaining steps assume it is still listening on `:8080`. Pick whichever approach suits your workflow:

1. **(Recommended)** Run it in its own dedicated terminal tab and leave it there.
2. **Background it** (logs to a file):
   ```bash
   nohup uvx opensandbox-server --config ~/.sandbox.toml >/tmp/opensandbox.log 2>&1 &
   # Follow logs with: tail -f /tmp/opensandbox.log
   ```
3. **Use tmux/screen** (keeps it recoverable):
   ```bash
   tmux new-session -d -s opensandbox 'uvx opensandbox-server --config ~/.sandbox.toml'
   ```

#### Expected output

```
$ uvx opensandbox-server --config ~/.sandbox.toml
INFO  Listening on :8080
```

---

## Step 3: Set env vars and start `demesne-mcp`

At minimum you need the three required variables from the [Configuration reference](../reference/configuration.md#environment-variables):

```bash
export OPEN_SANDBOX_DOMAIN=localhost:8080
export OPEN_SANDBOX_API_KEY=your-secret-key   # the [server] api_key you set in Step 2
export DEMESNE_ALLOWED_PATHS=/home/username/code
```

Optionally verify the binary starts cleanly (this is a smoke-check — Ctrl-C to exit; the real run happens in Step 4 when Claude Code spawns it):

```bash
demesne-mcp
```

#### Expected output

```
# (demesne-mcp blocks, waiting for JSON-RPC on stdin)
```

No output on startup is correct — it is waiting for a client. This manual invocation is optional and just confirms the binary is functional. Step 4 (Claude Code) is what actually runs `demesne-mcp` for real.

---

## Step 4: Wire into your agent (Claude Code or Codex)

Add demesne to your user-level Claude Code config (`~/.claude.json`, available in every project) with `claude mcp add`:

```bash
claude mcp add --transport stdio --scope user \
  --env OPEN_SANDBOX_DOMAIN=localhost:8080 \
  --env OPEN_SANDBOX_API_KEY=your-secret-key \
  --env DEMESNE_ALLOWED_PATHS=/home/username/code \
  demesne -- /usr/local/bin/demesne-mcp
```

Replace `/usr/local/bin/demesne-mcp` with the actual path from Step 1 (e.g. `~/go/bin/demesne-mcp`), and use the same `OPEN_SANDBOX_API_KEY` you set as `[server] api_key` in Step 2. Keep `--transport stdio` ahead of the server name `demesne`. Claude Code spawns `demesne-mcp` as a child process and talks to it over stdio.

demesne returns each run's `output_dir` under `~/.demesne/out`; so your agent can open those files without a permission prompt on every read, grant it read access — in Claude Code, add `~/.demesne/out` to `permissions.additionalDirectories` or start the session with `--add-dir ~/.demesne/out`. See [Let your agent read demesne's output](../how-to/wire-into-mcp-client.md#let-your-agent-read-demesnes-output).

Using Codex (or another client)? See [Wire demesne into your MCP client](../how-to/wire-into-mcp-client.md) for the Codex `config.toml` block and Claude Desktop / VS Code pointers.

#### Expected output

In Claude Code, after reloading the MCP config (restart Claude Code or run `/mcp`):

```
✓ demesne  connected
```

---

## Step 5: Make a `sandbox_script` call

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
output_dir: ~/.demesne/out/<job-id>
job_id: <uuid>
---
hello
Linux <container-hostname> 6.x.x ... x86_64 GNU/Linux
---stderr---
```

The command ran inside a disposable `continuumio/anaconda3` container (the default image). The `~/.demesne/out/<job-id>` directory on your host contains any files the command wrote to `/out` inside the sandbox.

---

## What next?

- **How-to guides** — [`../how-to/`](../how-to/) covers sharing host directories, egress control, spawning nested agents, and more.
- **Tool reference** — [`../reference/tools/`](../reference/tools/) has the full parameter tables, sample requests, and error tables for all 8 tools.
- **Concepts** — [`../explanation/`](../explanation/) explains the architecture, trust boundary, and key concepts in depth.
