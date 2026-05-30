# `sandbox_download`

Copy a file out of an existing sandbox to the host.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `sandbox_id` | string | yes | — | Sandbox handle returned by `sandbox_create`. |
| `src` | string | yes | — | Absolute path inside the sandbox to download. |

## Annotations

| Hint | Value | Rationale |
|------|-------|-----------|
| `readOnlyHint` | `true` | Does not mutate the sandbox; only reads a file from it. The file is written to `<output_dir>/downloads/<basename>` on the host, but that directory is the sandbox's own output area that the caller already controls. Per MCP spec semantics, `readOnlyHint` concerns the tool's *environment* (the sandbox), and writing to the caller's designated scratch dir does not violate read-only intent. Note: if your client interprets `readOnlyHint` strictly as "no host filesystem writes at all", treat this tool as read-write. |
| `destructiveHint` | `false` | Does not destroy or overwrite any data the caller did not produce. |
| `idempotentHint` | `true` | Downloading the same `src` repeatedly overwrites the same host destination path with the same contents. |
| `openWorldHint` | `false` | Operates sandbox-internally; no outbound network access is involved. |

## Sample request

```json
{
  "jsonrpc": "2.0",
  "id": 5,
  "method": "tools/call",
  "params": {
    "name": "sandbox_download",
    "arguments": {
      "sandbox_id": "b7e23a1f-...",
      "src": "/results.json"
    }
  }
}
```

## Sample result

```json
{
  "jsonrpc": "2.0",
  "id": 5,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "downloaded: /results.json -> /var/demesne/out/b7e23a1f-.../downloads/results.json"
      }
    ],
    "isError": false
  }
}
```

The text payload format (from `internal/server/tools.go`):

```
downloaded: <src> -> <host path>
```

The host path is `<output_dir>/downloads/<basename of src>`, where `output_dir` is the path returned by `sandbox_create`.

## Errors

| Error | When it occurs |
|-------|----------------|
| `sandbox_id and src are required` | `sandbox_id` or `src` is present but empty. |
| `attach to sandbox <id>: <error>` | The `sandbox_id` is unknown or the sandbox has expired. |
| `inspect sandbox <id>: <error>` | `GetInfo` call to OpenSandbox failed (e.g. sandbox not reachable). |
| `sandbox <id> is missing demesne.job metadata; not a demesne-managed sandbox` | The sandbox exists in OpenSandbox but was not created by demesne. |
| `create downloads dir: <OS error>` | Failed to create the `downloads/` subdirectory under `output_dir` on the host. |
| `download <src>: <error>` | OpenSandbox DownloadFile call failed (e.g. `src` does not exist inside the sandbox). |
| `create <host path>: <OS error>` | Failed to create the destination file on the host. |

## JSON Schema

See [sandbox_download.schema.json](sandbox_download.schema.json).
