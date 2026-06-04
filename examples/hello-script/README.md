# Hello, script — single-shot `sandbox_script` with a mounted host file

This example shows the simplest demesne pipeline: run one shell command in a fresh disposable sandbox, with a host file mounted read-only inside it. The sandbox exits, demesne returns the stdout, and the output directory is preserved on the host.

## Ask your agent

> "I've got a file at /tmp/demesne-example/greeting.txt — can you cat it in a sandbox and tell me what kernel that sandbox is running?"

The agent will reach for `sandbox_script`, pass the host path via `files`, and set `egress: "none"` since the command has no network dependency. You need `DEMESNE_ALLOWED_PATHS` to include `/tmp/demesne-example` so demesne accepts the mount.

## What you get

The tool returns `exit_code: 0`, the stdout from the command (the file contents followed by `uname -a` output), and an `output_dir` host path where anything the sandbox wrote to `/out` would be preserved. Because this command only reads and prints, the output directory exists but is empty.

`egress: "none"` ensured the sandbox had no outbound network access. The host file appeared at `/in/greeting.txt` inside the sandbox — demesne mounts each path in `files` read-only at `/in/<basename>`.

## Under the hood

### Setup

Create the host file that will be mounted into the sandbox:

```bash
mkdir -p /tmp/demesne-example && echo 'hello, world' > /tmp/demesne-example/greeting.txt
```

### The call

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

### Run it

```bash
bash run.sh
```

See [run.sh](run.sh) for the full script.

### What you'll see

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
