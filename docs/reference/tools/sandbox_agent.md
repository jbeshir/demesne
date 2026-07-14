# `sandbox_agent`

Run an AI agent inside a fresh sandbox against the caller's prompt.

## Parameters

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `prompt` | string | yes | — | Task for the agent. Free-form text. |
| `model` | string | no | credential-aware | Model for the agent; the provider is inferred from the model. claude-code uses `fable` (most capable), `opus`, `sonnet`, or `haiku`; codex uses `gpt-5.6-sol`, `gpt-5.6-terra`, `gpt-5.6-luna`, `gpt-5.5`, or `gpt-5.4-mini`. Defaults to the credential-aware provider's default model: codex/gpt-5.6-sol when Codex credentials are configured, otherwise claude-code/sonnet. The MCP input schema's enum is filtered at registration time to the union of the configured providers' models. |
| `preamble` | string | no | — | Optional prose prepended verbatim to the generated agent context file (e.g. CLAUDE.md for claude-code) before the auto-generated environment section. |
| `egress` | string | no | `none` | Additional outbound network policy on top of the agent's backend proxy (which is always reachable). `none` (default) means only the proxy; `package-managers` also allows npm/PyPI/conda registries. `open` is rejected — use `sandbox_research` for unrestricted egress (which has no input mounts). |
| `files` | array of strings | no | — | Host file paths to mount read-only into `/in/<basename>`. Each path must be absolute and inside `DEMESNE_ALLOWED_PATHS`. The live MCP input schema's description for this parameter is populated at registration time with the configured `DEMESNE_ALLOWED_PATHS` roots (or a no-host-inputs warning when none are configured). |
| `directories` | array of strings | no | — | Host directory paths to mount read-only into `/in/<basename>`. Each path must be absolute and inside `DEMESNE_ALLOWED_PATHS`. The live MCP input schema's description for this parameter is populated at registration time with the configured `DEMESNE_ALLOWED_PATHS` roots (or a no-host-inputs warning when none are configured). |
| `output_path` | string | no | — | Optional. Where the agent should write its final artefact. Rendered as a Definition of done block. |
| `output_format` | string | no | — | Optional. Expected shape/format of the output. |
| `success_criteria` | array of strings | no | — | Optional. Checklist of conditions the output must satisfy. |
| `background` | boolean | no | `false` | When `true`, returns immediately with `{job_id, status:"running"}` instead of blocking; poll with `sandbox_status` / `sandbox_wait`, cancel with `sandbox_cancel`. |

## Async usage

Synchronous agent calls may run to completion (subject to the explicit 48h runtime limit) and remain cancellable. Pass `background: true` for concurrent work, detachment, status/progress polling, or deliberate job control. The response is `{job_id, status: "running"}`; poll it with `sandbox_status` or block (up to 120s per call) with `sandbox_wait`, and cancel its descendant subtree with `sandbox_cancel`.

## Annotations

| Hint | Value | Rationale |
|------|-------|-----------|
| `readOnlyHint` | `false` | Creates a sandbox, writes artefacts to `/out`, and tears the sandbox down. |
| `destructiveHint` | `false` | The agent runs in its own fresh sandbox; it does not mutate the caller's state or any pre-existing sandbox. |
| `idempotentHint` | `false` | LLM runs are non-deterministic; repeating the same prompt can produce different artefacts and API costs. |
| `openWorldHint` | `true` | Reaches the Anthropic API (or other vendor API) through the vendor proxy; with `egress=package-managers` it also reaches npm/PyPI/conda registries. The agent can also spawn child sandboxes via the demesne child MCP server. |

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

When demesne is configured with host MCP servers, the agent sees those servers through the sidecar tunnel. These servers are advertised in the agent's CLAUDE.md under `## Available host tools` and wired into the agent's MCP config. Tools are filtered through the read-only allowlist; calls that reach blocked servers or tools surface as errors from the agent's MCP calls, not directly from `sandbox_agent`. Resources, resource templates, prompts, and completion are relayed in full from any exposed upstream without allowlist filtering. Listings reflect a static snapshot taken at aggregator start. See `internal/mcpproxy/server.go` for the filtering logic.

## Errors

| Error | When it occurs |
|-------|----------------|
| `prompt is required` | `prompt` parameter is present but empty or whitespace-only. |
| `egress 'open' is not permitted; use sandbox_research for unrestricted egress` | `egress` was set to `open`; use `sandbox_research` instead. |
| `model "<name>" unknown model (known: [...])` | `model` parameter is not known to any configured provider. |
| `DEMESNE_CLAUDE_CODE_OAUTH_TOKEN is required for sandbox_agent (run 'claude setup-token' to obtain one)` | The Claude Code OAuth token env var is not set on the demesne process. Required when the resolved provider is claude-code. |
| `DEMESNE_CODEX_AUTH_FILE (default ~/.codex/auth.json) is required for sandbox_agent when using a codex model` | The Codex auth file is not set. Required when the resolved provider is codex. |
| `model "<name>" is not in the Anthropic allowlist ([sonnet opus fable haiku])` | `model` parameter is not one of the valid Claude tiers. |
| `model "<name>" is not in the Codex allowlist ([gpt-5.6-sol gpt-5.6-terra gpt-5.6-luna gpt-5.5 gpt-5.4-mini])` | `model` parameter is not one of the valid Codex models. |
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
