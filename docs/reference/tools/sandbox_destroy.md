# `sandbox_destroy`

Destroy an existing sandbox.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `sandbox_id` | string | yes | — | Sandbox handle returned by `sandbox_create`. |

## Annotations

| Hint | Value | Rationale |
|------|-------|-----------|
| `readOnlyHint` | `false` | Kills the sandbox container; in-container state is irrecoverably destroyed. |
| `destructiveHint` | `true` | The sandbox container is killed and cannot be recovered. The host `output_dir` (containing `/out` artefacts and any `sandbox_download` results) is preserved on the host, but in-container state is lost. |
| `idempotentHint` | `true` | Destroying an already-destroyed or expired sandbox surfaces an error from the `attach` call (the sandbox no longer exists in OpenSandbox), so in practice the operation is not silently idempotent — it errors on a second call. The logical intent is idempotent (the desired end-state is "sandbox gone"), but callers should expect an error on repeated calls. |
| `openWorldHint` | `false` | Only kills the container; no outbound network access is involved. |

## Sample request

```json
{
  "jsonrpc": "2.0",
  "id": 6,
  "method": "tools/call",
  "params": {
    "name": "sandbox_destroy",
    "arguments": {
      "sandbox_id": "b7e23a1f-..."
    }
  }
}
```

## Sample result

```json
{
  "jsonrpc": "2.0",
  "id": 6,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "destroyed: b7e23a1f-..."
      }
    ],
    "isError": false
  }
}
```

The text payload format (from `internal/server/tools.go`):

```
destroyed: <sandbox_id>
```

The host output directory produced by `sandbox_create` is not removed; inspect or remove it separately when no longer needed.

## Errors

| Error | When it occurs |
|-------|----------------|
| `sandbox_id is required` | `sandbox_id` parameter is present but empty. |
| `attach to sandbox <id>: <error>` | The `sandbox_id` is unknown or the sandbox has already been destroyed/expired. |
| `kill sandbox <id>: <error>` | OpenSandbox SDK `Kill` call failed. |

## JSON Schema

See [sandbox_destroy.schema.json](sandbox_destroy.schema.json).
