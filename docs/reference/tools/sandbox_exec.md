# `sandbox_exec`

Run a shell command in an existing sandbox.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `sandbox_id` | string | yes | — | Sandbox handle returned by `sandbox_create`. |
| `command` | string | yes | — | Shell command to run inside the sandbox. Executed with `/bin/sh -c`. Working directory is `/out`. |

## Annotations

| Hint | Value | Rationale |
|------|-------|-----------|
| `readOnlyHint` | `false` | Executes arbitrary commands that can mutate the sandbox filesystem. |
| `destructiveHint` | `true` | Can delete files inside the sandbox and mutate persistent sandbox state; the TTL is also refreshed by 24h before the command runs. |
| `idempotentHint` | `false` | Running the same command twice can produce different results. |
| `openWorldHint` | `true` | The sandbox retains the egress policy set at `sandbox_create` time; with `package-managers` it can reach registries on the open internet. |

## Sample request

```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "tools/call",
  "params": {
    "name": "sandbox_exec",
    "arguments": {
      "sandbox_id": "b7e23a1f-...",
      "command": "pip install pandas && python -c 'import pandas; print(pandas.__version__)'"
    }
  }
}
```

## Sample result

```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "exit_code: 0\n---\n2.2.0\n"
      }
    ],
    "isError": false
  }
}
```

The text payload format (from `internal/server/tools.go`):

```
exit_code: <int>
---
<stdout from the command>
```

The sandbox TTL is refreshed by 24 hours before the command runs.

## Errors

| Error | When it occurs |
|-------|----------------|
| `attach to sandbox <id>: <error>` | The `sandbox_id` is unknown or the sandbox has expired and been destroyed by OpenSandbox. |
| `run command: <error>` | OpenSandbox SDK returned an error executing the command inside the sandbox. |

## JSON Schema

See [sandbox_exec.schema.json](sandbox_exec.schema.json).
