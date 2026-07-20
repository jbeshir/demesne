# `sandbox_status`

Get the current status of a background sandbox job.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `job_id` | string | yes | — | Job ID returned by a background `sandbox_script`, `sandbox_agent`, or `sandbox_research` call. |
| `include_stdout_tail` | boolean | no | `false` | Include the existing bounded tail of captured stdout. |

## Annotations

| Hint | Value | Rationale |
|------|-------|-----------|
| `readOnlyHint` | `true` | Reads job state; does not mutate anything. |
| `destructiveHint` | `false` | No state is destroyed or modified. |
| `idempotentHint` | `true` | Repeated calls with the same `job_id` return the same (or more-advanced) status without side effects. |
| `openWorldHint` | `false` | Only reads local job registry; no outbound network access. |

## Sample request

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "sandbox_status",
    "arguments": {
      "job_id": "job-3f2a1b4c-...",
      "include_stdout_tail": true
    }
  }
}
```

## Sample result — running

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "job_id: job-3f2a1b4c-...\nstatus: running\nelapsed_seconds: 12.3\n"
      }
    ],
    "structuredContent": {
      "job_id": "job-3f2a1b4c-...",
      "status": "running",
      "elapsed_seconds": 12.3
    },
    "isError": false
  }
}
```

## Sample result — succeeded

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "job_id: job-3f2a1b4c-...\nstatus: succeeded\nelapsed_seconds: 34.1\n---stdout_tail---\nAll done.\n"
      }
    ],
    "structuredContent": {
      "job_id": "job-3f2a1b4c-...",
      "status": "succeeded",
      "elapsed_seconds": 34.1,
      "stdout_tail": "All done.\n",
      "exit_code": 0,
      "cost_usd": 0.0021,
      "total_usage_usd": 0.0021
    },
    "isError": false
  }
}
```

The text payload format (from `internal/server/format.go`):

```
job_id: <job id>
status: <running|succeeded|failed|cancelled>
elapsed_seconds: <float, one decimal place>
[message: <string>]
[---stdout_tail---
<last bytes of captured stdout>]
```

Returned as `structuredContent` against the declared output schema — see [Structured output](README.md#structured-output). Fields for this tool:

| Field | Type | Present when |
|-------|------|-------------|
| `job_id` | string | always |
| `status` | string | always — one of `running`, `succeeded`, `failed`, `cancelled` |
| `elapsed_seconds` | number | always |
| `stdout_tail` | string | `include_stdout_tail` is true and stdout has been captured |
| `exit_code` | integer | terminal state (succeeded or failed) |
| `cost_usd` | number | terminal state and cost was tracked |
| `total_usage_usd` | number | terminal state and cost was tracked |
| `message` | string | extra context available (e.g. internal error detail) |

## Errors

| Error | When it occurs |
|-------|----------------|
| `job_id is required` | `job_id` parameter is present but empty. |
| `job not found` | The `job_id` is unknown — either never issued, already expired (jobs are retained for 1 hour after completion), or from a different server instance. |

## JSON Schema

See [sandbox_status.schema.json](sandbox_status.schema.json).
