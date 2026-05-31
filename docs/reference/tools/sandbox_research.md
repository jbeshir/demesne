# `sandbox_research`

Run a long-running research agent in a fresh sandbox with unrestricted outbound internet access.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `prompt` | string | yes | — | Research task for the agent. Free-form text. |
| `agent` | string | no | `claude-code` | Agent provider. `claude-code` (default) or `codex` (OpenAI Codex CLI, experimental — see README). |
| `model` | string | no | `sonnet` | Model for the agent. Provider-specific: claude-code uses `opus`, `sonnet` (default), or `haiku`; codex uses the gpt-5.x family. |
| `preamble` | string | no | — | Optional prose prepended verbatim to the generated agent context file (e.g. CLAUDE.md for claude-code) before the auto-generated environment section. |

## Annotations

| Hint | Value | Rationale |
|------|-------|-----------|
| `readOnlyHint` | `false` | Creates a sandbox and writes artefacts to `/out`. |
| `destructiveHint` | `false` | The agent runs in its own fresh sandbox with no `/in` mounts; it does not mutate the caller's state. |
| `idempotentHint` | `false` | LLM runs are non-deterministic; the agent also fetches live data from the open internet. |
| `openWorldHint` | `true` | Egress is always fully open — any public HTTPS endpoint is reachable. The agent-vendor proxy still gates model API calls. |

## Sample request

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "sandbox_research",
    "arguments": {
      "prompt": "Search for the latest benchmarks comparing LLM inference frameworks and write a summary to /out/report.md.",
      "agent": "claude-code",
      "model": "sonnet"
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
        "text": "exit_code: 0\noutput_dir: /var/demesne/out/a1b2c3d4-.../out\njob_id: a1b2c3d4-...\ncost_usd: 0.0185\ntotal_usage_usd: 0.0185\n---\nI've written the benchmark summary to /out/report.md.\n"
      }
    ],
    "structuredContent": {
      "exit_code": 0,
      "output_dir": "/var/demesne/out/a1b2c3d4-.../out",
      "job_id": "a1b2c3d4-...",
      "cost_usd": 0.0185,
      "total_usage_usd": 0.0185,
      "stdout": "I've written the benchmark summary to /out/report.md.\n"
    },
    "isError": false
  }
}
```

The text payload format (from `internal/server/format.go`):

```
exit_code: <int>
output_dir: <host path of /out>
job_id: <UUID>
cost_usd: <float, 4 decimal places>
total_usage_usd: <float, 4 decimal places>
---
<agent's final answer / stdout>
```

The same result is also returned as `structuredContent` against a declared [`outputSchema`](https://modelcontextprotocol.io/specification/2025-06-18/server/tools#output-schema). Clients that support structured output — including Claude Code and the Codex CLI — consume it and ignore the text block above, which remains as a fallback for clients that don't:

| Field | Type |
|-------|------|
| `exit_code` | integer |
| `output_dir` | string |
| `job_id` | string |
| `cost_usd` | number |
| `total_usage_usd` | number |
| `stdout` | string |

`cost_usd` is the indicative spend this run incurred through its vendor proxy, computed from published API pricing. It is reported regardless of how the underlying OAuth token is billed (Claude Code OAuth tokens typically authorise against a Claude Console subscription, not per-request API billing). `total_usage_usd` adds the cost of any child sandboxes this agent spawned.

The `output_dir` contains:
- Any artefacts the agent wrote to `/out` inside the sandbox.
- `usage.json` — the per-run token and cost snapshot written by the sidecar proxy.
- `results.json` — a rolled-up cost tree covering this run and all descendants.
- `transcript.jsonl` — the agent's raw stdout.
- `stderr.log` — the agent's stderr.

See [`../usage-json.md`](../usage-json.md) and [`../results-json.md`](../results-json.md) for field-level documentation of those files.

Unlike `sandbox_agent`, this tool has no `files`, `directories`, or `egress` parameters. Egress is always `open`; there are no `/in` mounts. The combination of read-only host inputs and unrestricted outbound access is deliberately kept off the surface — use `sandbox_agent` for input mounts, or `sandbox_research` for open egress, never both.

### Host MCP proxy note

When demesne is configured with host MCP servers, the agent sees those servers through the sidecar tunnel. Tool calls that are allowlist-blocked surface as errors from the agent's MCP calls. See `internal/mcpproxy/server.go` for the filtering logic.

## Errors

| Error | When it occurs |
|-------|----------------|
| `prompt is required` | `prompt` parameter is present but empty or whitespace-only. |
| `agent "<name>" is not registered (available: [...])` | The `agent` parameter names an unknown provider. |
| `DEMESNE_CLAUDE_CODE_OAUTH_TOKEN is required for sandbox_research (run 'claude setup-token' to obtain one)` | The Claude Code OAuth token env var is not set on the demesne process. Required for `agent=claude-code`. |
| `DEMESNE_CODEX_AUTH_FILE (default ~/.codex/auth.json) is required for sandbox_research when agent="codex"` | The Codex auth file is not set. Required for `agent=codex`. |
| `model "<name>" is not in the Anthropic whitelist ([opus sonnet haiku])` | `model` parameter is not one of the three valid Claude tiers. |
| `build sidecar image: <error>` | The demesne sidecar Docker image could not be built. |
| `build agent image: <error>` | The agent provider's container image could not be built or pulled. |
| `DOCKER::SANDBOX_EXECD_DISTRIBUTION_FAILED … passing bulk input to subprocess` | Transient buildah-copier race. Demesne retries up to 3 times; surfaces only if all attempts fail. |

## JSON Schema

See [sandbox_research.schema.json](sandbox_research.schema.json).
