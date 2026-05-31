# `sandbox_create`

Create a persistent sandbox and return its handle.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `image` | string | no | `anaconda` | Container image. One of: `node` (node:22), `python` (python:3.12), `go` (golang:1), `anaconda` (continuumio/anaconda3:latest, default). |
| `egress` | string | no | `package-managers` | Outbound network policy. `package-managers` allows npm, PyPI, and conda registries; `none` denies all egress. |
| `files` | array of strings | no | — | Host file paths to mount read-only into `/in/<basename>`. Each path must be absolute and inside `DEMESNE_ALLOWED_PATHS`. |
| `directories` | array of strings | no | — | Host directory paths to mount read-only into `/in/<basename>`. Each path must be absolute and inside `DEMESNE_ALLOWED_PATHS`. |

## Annotations

| Hint | Value | Rationale |
|------|-------|-----------|
| `readOnlyHint` | `false` | Creates a new sandbox container and host output directory. |
| `destructiveHint` | `false` | Only creates new resources; does not mutate or destroy existing state. |
| `idempotentHint` | `false` | Each call mints a fresh sandbox with a new `sandbox_id`. |
| `openWorldHint` | `true` | With `egress=package-managers` (the default) the sandbox can reach npm/PyPI/conda registries on the open internet. |

## Sample request

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "sandbox_create",
    "arguments": {
      "image": "python",
      "egress": "package-managers",
      "files": [],
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
        "text": "sandbox_id: b7e23a1f-...\noutput_dir: /var/demesne/out/b7e23a1f-..."
      }
    ],
    "structuredContent": {
      "sandbox_id": "b7e23a1f-...",
      "output_dir": "/var/demesne/out/b7e23a1f-..."
    },
    "isError": false
  }
}
```

The text payload format (from `internal/server/format.go`):

```
sandbox_id: <OpenSandbox UUID>
output_dir: <host path of /out>
```

The same result is also returned as `structuredContent` against a declared [`outputSchema`](https://modelcontextprotocol.io/specification/2025-06-18/server/tools#output-schema). Clients that support structured output — including Claude Code and the Codex CLI — consume it and ignore the text block above, which remains as a fallback for clients that don't:

| Field | Type |
|-------|------|
| `sandbox_id` | string |
| `output_dir` | string |

Pass `sandbox_id` to `sandbox_exec`, `sandbox_upload`, `sandbox_download`, and `sandbox_destroy`. The sandbox TTL is 24 hours from creation, refreshed by each `sandbox_exec` call. Call `sandbox_destroy` to tear it down explicitly before the TTL expires.

## Errors

| Error | When it occurs |
|-------|----------------|
| `image "<name>" is not in the whitelist (node, python, anaconda, go)` | `image` parameter names an unknown container image. |
| `egress mode "<mode>" is not in the whitelist (none, package-managers, open)` | `egress` parameter is not one of the three valid modes. |
| `mount path must be absolute: <path>` | A path in `files` or `directories` is relative. |
| `mount path <path> is not within DEMESNE_ALLOWED_PATHS` | A path in `files` or `directories` is outside every configured `DEMESNE_ALLOWED_PATHS` entry. |
| `resolve mount path <path>: <OS error>` | Symlink resolution failed for a path in `files` or `directories`. |
| `mount path is empty` | An empty string was passed in `files` or `directories`. |
| `mount basename "<base>" would collide: <p1> and <p2>` | Two input paths share the same basename. |
| `<path> is not a regular file` | A path in `files` is a directory or special file. |
| `<path> is not a directory` | A path in `directories` is a regular file. |
| `DOCKER::SANDBOX_EXECD_DISTRIBUTION_FAILED … passing bulk input to subprocess` | Transient buildah-copier race. Demesne retries up to 3 times; surfaces only if all attempts fail. |
| `VOLUME::HOST_PATH_NOT_ALLOWED` | OpenSandbox server rejected the bind mount because the host path is not in the server's `allowed_host_paths` list. Check `~/.sandbox.toml`. |
| `create sandbox: <error>` | OpenSandbox SDK returned an error during sandbox creation. |

## JSON Schema

See [sandbox_create.schema.json](sandbox_create.schema.json).
