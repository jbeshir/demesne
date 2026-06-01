# `sandbox_agent`

Run an AI agent inside a fresh sandbox against the caller's prompt.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `prompt` | string | yes | — | Task for the agent. Free-form text. |
| `agent` | string | no | `claude-code` | Agent provider. `claude-code` (default) or `codex` (OpenAI Codex CLI, experimental — see README). |
| `model` | string | no | `sonnet` | Model for the agent. Provider-specific: claude-code uses `opus`, `sonnet` (default), or `haiku`; codex uses the gpt-5.x family. |
| `preamble` | string | no | — | Optional prose prepended verbatim to the generated agent context file (e.g. CLAUDE.md for claude-code) before the auto-generated environment section. |
| `egress` | string | no | `none` | Additional outbound network policy on top of the agent's backend proxy (which is always reachable). `none` (default) means only the proxy; `package-managers` also allows npm/PyPI/conda registries. `open` is rejected — use `sandbox_research` for unrestricted egress (which has no input mounts). |
| `files` | array of strings | no | — | Host file paths to mount read-only into `/in/<basename>`. Each path must be absolute and inside `DEMESNE_ALLOWED_PATHS`. |
| `directories` | array of strings | no | — | Host directory paths to mount read-only into `/in/<basename>`. Each path must be absolute and inside `DEMESNE_ALLOWED_PATHS`. |

## Annotations

| Hint | Value | Rationale |
|------|-------|-----------|
| `readOnlyHint` | `false` | Creates a sandbox, writes artefacts to `/out`, and tears the sandbox down. |
| `destructiveHint` | `false` | The agent runs in its own fresh sandbox; it does not mutate the caller's state or any pre-existing sandbox. |
| `idempotentHint` | `false` | LLM runs are non-deterministic; repeating the same prompt can produce different artefacts and API costs. |
| `openWorldHint` | `true` | Reaches the Anthropic API (or other vendor API) through the on-host proxy; with `egress=package-managers` it also reaches npm/PyPI/conda registries. The agent can also spawn child sandboxes via the demesne child MCP server. |

## Sample request

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "sandbox_agent",
    "arguments": {
      "prompt": "Write a Python script that reads /in/data.csv and outputs a summary to /out/summary.txt.",
      "agent": "claude-code",
      "model": "sonnet",
      "egress": "none",
      "files": ["/home/user/data.csv"]
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
        "text": "exit_code: 0\noutput_dir: /var/demesne/out/9c4d2e1a-.../out\njob_id: 9c4d2e1a-...\ncost_usd: 0.0042\ntotal_usage_usd: 0.0042\n---\nI've written summary.txt to /out/summary.txt.\n\n---stderr---\n"
      }
    ],
    "structuredContent": {
      "exit_code": 0,
      "output_dir": "/var/demesne/out/9c4d2e1a-.../out",
      "job_id": "9c4d2e1a-...",
      "cost_usd": 0.0042,
      "total_usage_usd": 0.0042,
      "stdout": "I've written summary.txt to /out/summary.txt.\n",
      "stderr": ""
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
---stderr---
<agent's stderr>
```

Returned as `structuredContent` against the declared output schema — see [Structured output](README.md#structured-output) for the cross-tool conventions. Fields for this tool:

| Field | Type |
|-------|------|
| `exit_code` | integer |
| `output_dir` | string |
| `job_id` | string |
| `cost_usd` | number |
| `total_usage_usd` | number |
| `stdout` | string |
| `stderr` | string |

The MCP `stderr` field is the last 16 KiB of `stderr.log`; the file is the complete stream.

`cost_usd` is indicative and `total_usage_usd` adds child sandboxes' costs — see [Indicative cost reporting](../../explanation/key-concepts.md#indicative-cost-reporting).

The `output_dir` contains:
- Any artefacts the agent wrote to `/out` inside the sandbox.
- `usage.json` — the per-run token and cost snapshot written by the sidecar proxy.
- `results.json` — a rolled-up cost tree covering this run and all descendants.
- `transcript.jsonl` — the agent's raw stdout (e.g. claude-code stream-json events).
- `stderr.log` — the agent's stderr.

See [`../usage-json.md`](../usage-json.md) and [`../results-json.md`](../results-json.md) for field-level documentation of those files.

### Host MCP proxy note

When demesne is configured with host MCP servers, the agent sees those servers through the sidecar tunnel. These servers are not advertised in the agent CLAUDE.md or MCP tool catalogue; agents discover them on demand via the standard MCP list methods. Tools are filtered through the read-only allowlist; calls that reach blocked servers or tools surface as errors from the agent's MCP calls, not directly from `sandbox_agent`. Resources, resource templates, prompts, and completion are relayed in full from any exposed upstream without allowlist filtering. Listings reflect a static snapshot taken at aggregator start. See `internal/mcpproxy/server.go` for the filtering logic.

## Errors

| Error | When it occurs |
|-------|----------------|
| `prompt is required` | `prompt` parameter is present but empty or whitespace-only. |
| `egress 'open' is not permitted for sandbox_agent; use sandbox_research for unrestricted egress` | `egress` was set to `open`; use `sandbox_research` instead. |
| `agent "<name>" is not registered (available: [...])` | The `agent` parameter names an unknown provider. |
| `DEMESNE_CLAUDE_CODE_OAUTH_TOKEN is required for sandbox_agent (run 'claude setup-token' to obtain one)` | The Claude Code OAuth token env var is not set on the demesne process. Required for `agent=claude-code`. |
| `DEMESNE_CODEX_AUTH_FILE (default ~/.codex/auth.json) is required for sandbox_agent when agent="codex"` | The Codex auth file is not set. Required for `agent=codex`. |
| `model "<name>" is not in the Anthropic whitelist ([opus sonnet haiku])` | `model` parameter is not one of the three valid Claude tiers. |
| `mount path must be absolute: <path>` | A path in `files` or `directories` is relative. |
| `mount path <path> is not within DEMESNE_ALLOWED_PATHS` | A path in `files` or `directories` is outside every `DEMESNE_ALLOWED_PATHS` entry. |
| `resolve mount path <path>: <OS error>` | Symlink resolution failed for an input path. |
| `mount basename "<base>" would collide: <p1> and <p2>` | Two input paths share the same basename. |
| `build sidecar image: <error>` | The demesne sidecar Docker image could not be built (e.g. Docker daemon not running, or `make sidecar-binary` was not run). |
| `build agent image: <error>` | The agent provider's container image could not be built or pulled. |
| `DOCKER::SANDBOX_EXECD_DISTRIBUTION_FAILED … passing bulk input to subprocess` | Transient buildah-copier race. Demesne retries up to 3 times; surfaces only if all attempts fail. |
| `VOLUME::HOST_PATH_NOT_ALLOWED` | OpenSandbox server rejected a bind mount. Check `~/.sandbox.toml` `allowed_host_paths`. |

## JSON Schema

See [sandbox_agent.schema.json](sandbox_agent.schema.json).
