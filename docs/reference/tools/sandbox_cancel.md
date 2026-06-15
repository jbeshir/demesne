# `sandbox_cancel`

Request cancellation of a background sandbox job and its entire descendant subtree.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `job_id` | string | yes | — | Job ID returned by a background `sandbox_script`, `sandbox_agent`, or `sandbox_research` call. |

## Annotations

| Hint | Value | Rationale |
|------|-------|-----------|
| `readOnlyHint` | `false` | Cancels the job and tears down its container(s). |
| `destructiveHint` | `true` | Any in-progress work in the job's sandbox is discarded. The host output directory is preserved. |
| `idempotentHint` | `true` | Calling cancel on an already-terminal job (succeeded/failed/cancelled) returns its final status without error or side effects. |
| `openWorldHint` | `false` | Only signals the local job; no outbound network access. |

## Sample request

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "sandbox_cancel",
    "arguments": {
      "job_id": "job-3f2a1b4c-..."
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
        "text": "job_id: job-3f2a1b4c-...\nstatus: cancelled"
      }
    ],
    "structuredContent": {
      "job_id": "job-3f2a1b4c-...",
      "status": "cancelled"
    },
    "isError": false
  }
}
```

The text payload format (from `internal/server/format.go`):

```
job_id: <job id>
status: cancelled
```

Returned as `structuredContent` against the declared output schema — see [Structured output](README.md#structured-output). Fields for this tool:

| Field | Type |
|-------|------|
| `job_id` | string |
| `status` | string — `cancelled` for a live job, or the job's existing terminal status if it had already completed |

### Cancellation order

Cancel walks the job tree depth-first: child jobs are cancelled before their parent. For each live job, the cancellation signal propagates into the sandbox's context, which triggers the existing deferred teardown path (sidecar/egress teardown first, then `Destroy`). Jobs already in a terminal state at cancel time are skipped.

## Errors

| Error | When it occurs |
|-------|----------------|
| `job_id is required` | `job_id` parameter is present but empty. |
| `job not found` | The `job_id` is unknown — either never issued, already expired (jobs are retained for 1 hour after completion), or from a different server instance. |

## JSON Schema

See [sandbox_cancel.schema.json](sandbox_cancel.schema.json).
