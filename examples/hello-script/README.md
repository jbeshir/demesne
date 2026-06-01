# Hello, script — single-shot `sandbox_script` with a mounted host file

This example runs a single shell command in a fresh disposable sandbox using `sandbox_script`. It mounts one host file read-only into the sandbox at `/in/greeting.txt`, reads it back with `cat`, and prints the kernel version alongside it. After the sandbox exits, demesne preserves the output directory on the host.

It demonstrates:

- **Host file mount** — the `files` parameter makes a host path available inside the sandbox at `/in/<basename>`.
- **`/in/` read-only** — files mounted via `files` are read-only inside the sandbox; the command cannot modify them.
- **`/out` artefact pickup** — anything the command writes to `/out` inside the sandbox is preserved in the `output_dir` returned by the tool.

## Prerequisites

- demesne is running and reachable (i.e., you can pipe JSON-RPC to `demesne-mcp` on `$PATH`).
- OpenSandbox is running and `OPEN_SANDBOX_DOMAIN` / `OPEN_SANDBOX_API_KEY` are set on the demesne process.
- `DEMESNE_ALLOWED_PATHS` includes `/tmp/demesne-example` (demesne will reject mounts outside this list).

## Setup

Create the host file that will be mounted into the sandbox:

```bash
mkdir -p /tmp/demesne-example && echo 'hello, world' > /tmp/demesne-example/greeting.txt
```

## The call

The request is a standard JSON-RPC 2.0 `tools/call` envelope:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "sandbox_script",
    "arguments": {
      "command": "cat /in/greeting.txt && echo \"--- ran in $(uname -a)\"",
      "image": "anaconda",
      "egress": "none",
      "files": ["/tmp/demesne-example/greeting.txt"]
    }
  }
}
```

- `name` — the MCP tool name: `sandbox_script`.
- `command` — the shell command to run inside the sandbox with `/bin/sh -c`. The working directory inside the sandbox is `/out`.
- `image` — the container image. `anaconda` (the default) is used here.
- `egress` — outbound network policy. `none` blocks all egress; the example has no network dependency.
- `files` — list of absolute host paths to mount read-only at `/in/<basename>`. `/tmp/demesne-example/greeting.txt` appears as `/in/greeting.txt` inside the sandbox.

## Run it

```bash
bash run.sh
```

See [run.sh](run.sh) for the full script.

## What you'll see

A JSON-RPC response containing the tool result:

```
exit_code: 0
output_dir: /var/demesne/out/<job_id>
job_id: <UUID>
---
hello, world
--- ran in Linux ... x86_64 GNU/Linux
---stderr---
```

- `exit_code: 0` — the command succeeded.
- `output_dir` — host path where any files written to `/out` inside the sandbox are preserved. This sandbox wrote nothing to `/out`, but the directory exists and can be inspected.
- `job_id` — the UUID for this run (same as the UUID in `output_dir`).
- The lines after `---` are the command's stdout.

## See also

[`../../docs/reference/tools/sandbox_script.md`](../../docs/reference/tools/sandbox_script.md) — full parameter reference, error table, and JSON Schema link.
