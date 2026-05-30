# `sandbox_upload`

Copy a host file into an existing sandbox.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `sandbox_id` | string | yes | — | Sandbox handle returned by `sandbox_create`. |
| `src` | string | yes | — | Host file path to upload. Must be absolute and inside `DEMESNE_ALLOWED_PATHS`. Symlinks are resolved before the check. |
| `dst` | string | yes | — | Destination path inside the sandbox. Must be absolute. Parent directory must already exist. |

## Annotations

| Hint | Value | Rationale |
|------|-------|-----------|
| `readOnlyHint` | `false` | Writes a file into the sandbox filesystem. |
| `destructiveHint` | `true` | Overwrites the destination path in the sandbox if it already exists. |
| `idempotentHint` | `true` | Uploading the same `src` to the same `dst` leaves the sandbox in the same final state. |
| `openWorldHint` | `false` | Operates sandbox-internally; no outbound network access is involved. |

## Sample request

```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "tools/call",
  "params": {
    "name": "sandbox_upload",
    "arguments": {
      "sandbox_id": "b7e23a1f-...",
      "src": "/home/user/data.csv",
      "dst": "/data.csv"
    }
  }
}
```

## Sample result

```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "uploaded: data.csv -> /data.csv"
      }
    ],
    "isError": false
  }
}
```

The text payload format (from `internal/server/tools.go`):

```
uploaded: <basename of src> -> <dst>
```

## Errors

| Error | When it occurs |
|-------|----------------|
| `sandbox_id, src, and dst are required` | Any of the three required parameters is present but empty. |
| `mount path must be absolute: <path>` | `src` is a relative path. |
| `mount path <path> is not within DEMESNE_ALLOWED_PATHS` | `src` is outside every `DEMESNE_ALLOWED_PATHS` entry after symlink resolution. |
| `resolve mount path <path>: <OS error>` | Symlink resolution failed for `src`. |
| `mount path is empty` | `src` is an empty string. |
| `stat <src>: <OS error>` | The resolved `src` path could not be stat'd (e.g. file does not exist). |
| `<src> is not a regular file` | `src` resolves to a directory or special file rather than a regular file. |
| `attach to sandbox <id>: <error>` | The `sandbox_id` is unknown or the sandbox has expired. |
| `upload <src> -> <dst>: <error>` | OpenSandbox UploadFile call failed (e.g. parent directory of `dst` does not exist inside the sandbox). |

## JSON Schema

See [sandbox_upload.schema.json](sandbox_upload.schema.json).
