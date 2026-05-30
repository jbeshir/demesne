# `sandbox_script`

Run a single shell command in a fresh sandbox and return its stdout.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `command` | string | yes | — | Shell command to run inside the sandbox. Executed with `/bin/sh -c`. Working directory is `/out`. |
| `image` | string | no | `anaconda` | Container image. One of: `node` (node:22), `python` (python:3.12), `go` (golang:1), `anaconda` (continuumio/anaconda3:latest, default). |
| `egress` | string | no | `package-managers` | Outbound network policy. `package-managers` allows npm, PyPI, and conda registries; `none` denies all egress. |
| `files` | array of strings | no | — | Host file paths to mount read-only into `/in/<basename>`. Each path must be absolute and inside `DEMESNE_ALLOWED_PATHS`. |
| `directories` | array of strings | no | — | Host directory paths to mount read-only into `/in/<basename>`. Each path must be absolute and inside `DEMESNE_ALLOWED_PATHS`. |

## Annotations

| Hint | Logical value | Currently set in code? | Rationale |
|------|--------------|------------------------|-----------|
| `readOnlyHint` | `false` | No (not declared in tool registration) | The tool creates a sandbox, writes to `/out`, and tears the sandbox down. |
| `destructiveHint` | `false` | No (not declared in tool registration) | The sandbox is created and destroyed as a unit; from the caller's perspective no persistent state is mutated. |
| `idempotentHint` | `false` | No (not declared in tool registration) | Running the same command twice can re-fetch packages or produce different side effects. |
| `openWorldHint` | `true` | No (not declared in tool registration) | With `egress=package-managers` (the default) the sandbox can reach npm/PyPI/conda registries on the open internet. |

These values are documented here; wiring them into the Go tool registration is a follow-up code item recorded in CHANGES.md.

## Sample request

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "sandbox_script",
    "arguments": {
      "command": "python -c 'import sys; print(sys.version)'",
      "image": "python",
      "egress": "none",
      "files": ["/home/user/data.csv"],
      "directories": []
    }
  }
}
```

## Sample result

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "exit_code: 0\noutput_dir: /var/demesne/out/3f2a1b4c-...\njob_id: 3f2a1b4c-...\n---\n3.12.0 (main, ...)\n"
      }
    ],
    "isError": false
  }
}
```

The text payload format (from `internal/server/format.go`):

```
exit_code: <int>
output_dir: <host path of /out>
job_id: <UUID>
---
<stdout from the command>
```

The `output_dir` is preserved on the host after the sandbox is destroyed; any files written to `/out` inside the sandbox are available there.

## Errors

| Error | When it occurs |
|-------|----------------|
| `image "<name>" is not in the whitelist (node, python, anaconda, go)` | `image` parameter names an unknown container image. |
| `egress mode "<mode>" is not in the whitelist (none, package-managers, open)` | `egress` parameter is not one of the three valid modes. |
| `mount path must be absolute: <path>` | A path in `files` or `directories` is relative. |
| `mount path <path> is not within DEMESNE_ALLOWED_PATHS` | A path in `files` or `directories` is outside every configured `DEMESNE_ALLOWED_PATHS` entry. |
| `resolve mount path <path>: <OS error>` | Symlink resolution failed for a path in `files` or `directories` (e.g. dangling symlink). |
| `mount path is empty` | An empty string was passed in `files` or `directories`. |
| `mount basename "<base>" would collide: <p1> and <p2>` | Two input paths share the same basename; they would both map to `/in/<basename>`. |
| `<path> is not a regular file` | A path supplied in `files` is a directory or special file. |
| `<path> is not a directory` | A path supplied in `directories` is a regular file. |
| `DOCKER::SANDBOX_EXECD_DISTRIBUTION_FAILED … passing bulk input to subprocess` | Transient buildah-copier race (buildah issue #6573). Demesne retries up to 3 times with backoff; the error surfaces only if all attempts fail. |
| `VOLUME::HOST_PATH_NOT_ALLOWED` | OpenSandbox server rejected the bind mount because the host path is not in the server's `allowed_host_paths` list. Check `~/.sandbox.toml`. |
| `create sandbox: <error>` | OpenSandbox SDK returned an error during sandbox creation. |

## JSON Schema

See [sandbox_script.schema.json](sandbox_script.schema.json).
