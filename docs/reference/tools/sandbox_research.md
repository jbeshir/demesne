# `sandbox_research`

Run a long-running research agent in a fresh sandbox with unrestricted outbound internet access.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `prompt` | string | yes | — | Research task for the agent. Free-form text. |
| `agent` | string | no | auto | Agent provider. `codex` or `claude-code` — defaults to `codex` when Codex credentials are configured, otherwise `claude-code`. |
| `model` | string | no | `sonnet` | Model for the agent. Provider-specific: claude-code uses `opus`, `sonnet` (default), or `haiku`; codex uses `gpt-5.5` (default) or `gpt-5.4-mini`. |
| `preamble` | string | no | — | Optional prose prepended verbatim to the generated agent context file (e.g. CLAUDE.md for claude-code) before the auto-generated environment section. |
| `output_path` | string | no | — | Optional. Where the agent should write its final artefact. Rendered as a Definition of done block. |
| `output_format` | string | no | — | Optional. Expected shape/format of the output. |
| `success_criteria` | array of strings | no | — | Optional. Checklist of conditions the output must satisfy. |

## Annotations

| Hint | Value | Rationale |
|------|-------|-----------|
| `readOnlyHint` | `false` | Creates a sandbox and writes artefacts to `/out`. |
| `destructiveHint` | `false` | The agent runs in its own fresh sandbox with no `/in` mounts; it does not mutate the caller's state. |
| `idempotentHint` | `false` | LLM runs are non-deterministic; the agent also fetches live data from the open internet. |
| `openWorldHint` | `true` | Egress is always fully open — any public HTTPS endpoint is reachable. The vendor proxy still gates model API calls. |

## Sample request

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "sandbox_research",
    "arguments": {
      "prompt": "Search for the latest benchmarks comparing LLM inference frameworks and write a summary to /out/report.md."
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
        "text": "exit_code: 0\noutput_dir: /var/demesne/out/a1b2c3d4-.../out\njob_id: a1b2c3d4-...\ncost_usd: 0.0185\ntotal_usage_usd: 0.0185\n---\nI've written the benchmark summary to /out/report.md.\n\n---stderr---\n"
      }
    ],
    "structuredContent": {
      "exit_code": 0,
      "output_dir": "/var/demesne/out/a1b2c3d4-.../out",
      "job_id": "a1b2c3d4-...",
      "cost_usd": 0.0185,
      "total_usage_usd": 0.0185,
      "stdout": "I've written the benchmark summary to /out/report.md.\n",
      "stderr": ""
    },
    "isError": false
  }
}
```

Output format, cost reporting, the `output_dir` contents, and the host MCP proxy note are the same as for [`sandbox_agent`](sandbox_agent.md) — see that page. Unlike `sandbox_agent`, this tool has no `files`/`directories`/`egress` parameters: egress is always `open`, there are no `/in` mounts, and the inputs-plus-open-egress shape is deliberately kept off the surface — see [Egress modes](../../explanation/key-concepts.md#egress-modes).

## Errors

| Error | When it occurs |
|-------|----------------|
| `prompt is required` | `prompt` parameter is present but empty or whitespace-only. |
| `agent "<name>" is not registered (available: [...])` | The `agent` parameter names an unknown provider. |
| `DEMESNE_CLAUDE_CODE_OAUTH_TOKEN is required for sandbox_research (run 'claude setup-token' to obtain one)` | The Claude Code OAuth token env var is not set on the demesne process. Required for `agent=claude-code`. |
| `DEMESNE_CODEX_AUTH_FILE (default ~/.codex/auth.json) is required for sandbox_research when agent="codex"` | The Codex auth file is not set. Required for `agent=codex`. |
| `model "<name>" is not in the Anthropic allowlist ([sonnet opus haiku])` | `model` parameter is not one of the three valid Claude tiers. |
| `model "<name>" is not in the Codex allowlist ([gpt-5.5 gpt-5.4-mini])` | `model` parameter is not one of the two valid Codex models (`agent="codex"`). |
| `build sidecar image: <error>` | The demesne sidecar Docker image could not be built. |
| `build agent image: <error>` | The agent provider's container image could not be built or pulled. |
| `DOCKER::SANDBOX_EXECD_DISTRIBUTION_FAILED … passing bulk input to subprocess` | Transient buildah-copier race. Demesne retries up to 3 times; surfaces only if all attempts fail. |

## JSON Schema

See [sandbox_research.schema.json](sandbox_research.schema.json).
