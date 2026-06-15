# `sandbox_wait`

Block until a background sandbox job reaches a terminal state or the timeout elapses.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `job_id` | string | yes | — | Job ID returned by a background `sandbox_script`, `sandbox_agent`, or `sandbox_research` call. |
| `timeout_seconds` | number | no | `30` | Maximum seconds to wait. 0 or omitted → 30 s default; hard-capped at 120 s. |

## Annotations

| Hint | Value | Rationale |
|------|-------|-----------|
| `readOnlyHint` | `true` | Waits on job state; does not mutate anything. |
| `destructiveHint` | `false` | No state is destroyed or modified. |
| `idempotentHint` | `true` | Waiting on an already-terminal job returns immediately with its final result without side effects. |
| `openWorldHint` | `false` | Only blocks on local job state; no outbound network access. |

## Sample request

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "sandbox_wait",
    "arguments": {
      "job_id": "job-3f2a1b4c-...",
      "timeout_seconds": 60
    }
  }
}
```

## Sample result — completed

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "job_id: job-3f2a1b4c-...\nstatus: succeeded\n---\nAll done.\n"
      }
    ],
    "structuredContent": {
      "job_id": "job-3f2a1b4c-...",
      "status": "succeeded",
      "result_text": "All done.\n",
      "output_path": "/var/demesne/out/3f2a1b4c-.../out",
      "exit_code": 0,
      "cost_usd": 0.0021,
      "total_usage_usd": 0.0021
    },
    "isError": false
  }
}
```

## Sample result — timeout (still running)

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "job_id: job-3f2a1b4c-...\nstatus: running\nmessage: still running; call sandbox_wait again\n"
      }
    ],
    "structuredContent": {
      "job_id": "job-3f2a1b4c-...",
      "status": "running",
      "message": "still running; call sandbox_wait again"
    },
    "isError": false
  }
}
```

A timeout is a **normal** (non-error) result — `isError` remains `false`. The `status` field is `"running"` and `message` is `"still running; call sandbox_wait again"`. Call `sandbox_wait` again to continue polling.

The text payload format (from `internal/server/format.go`):

```
job_id: <job id>
status: <running|succeeded|failed|cancelled>
[message: <string>]
[---
<result text from the completed job>]
```

Returned as `structuredContent` against the declared output schema — see [Structured output](README.md#structured-output). Fields for this tool:

| Field | Type | Present when |
|-------|------|-------------|
| `job_id` | string | always |
| `status` | string | always — one of `running`, `succeeded`, `failed`, `cancelled` |
| `message` | string | timeout (`"still running; call sandbox_wait again"`) |
| `result_text` | string | terminal state — the job's final stdout/answer |
| `output_path` | string | terminal state — host path of the job's `/out` mount |
| `exit_code` | integer | terminal state |
| `cost_usd` | number | terminal state and cost was tracked |
| `total_usage_usd` | number | terminal state and cost was tracked |

## Errors

| Error | When it occurs |
|-------|----------------|
| `job_id is required` | `job_id` parameter is present but empty. |
| `job not found` | The `job_id` is unknown — either never issued, already expired (jobs are retained for 1 hour after completion), or from a different server instance. |

## JSON Schema

See [sandbox_wait.schema.json](sandbox_wait.schema.json).
