# Sandbox agent — one prompt, one artefact

This example shows how to hand a one-shot task to a sub-agent: the agent runs inside a fresh sandbox, writes its output to `/out`, and demesne returns a summary along with token-usage and cost artefacts.

## Ask your agent

> "Spin up a sub-agent that writes a Fibonacci script, runs it, and saves the output to /out/fib.txt."

The agent will call `sandbox_agent` with a prompt describing the task. The sub-agent runs the Claude Code CLI (`claude-code` provider, `sonnet` model by default) inside a fresh sandbox with `egress: "package-managers"` so it can reach PyPI if needed. You need `DEMESNE_CLAUDE_CODE_OAUTH_TOKEN` set — see ["Getting an OAuth token (claude-code provider)"](../../docs/explanation/trust-boundary.md#getting-an-oauth-token-claude-code-provider) in `docs/explanation/trust-boundary.md`.

## What you get

The tool returns the sub-agent's summary text. The `output_dir` on the host contains:

- **`fib.txt`** — the file the agent was asked to produce.
- **`usage.json`** — per-model token counts and an indicative cost for this run. See [`../../docs/reference/usage-json.md`](../../docs/reference/usage-json.md).
- **`results.json`** — own cost plus a rolled-up cost tree covering this run and any child sandboxes it spawned. See [`../../docs/reference/results-json.md`](../../docs/reference/results-json.md).

## Under the hood

### The call

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "sandbox_agent",
    "arguments": {
      "prompt": "Write a Python script that prints the first 10 Fibonacci numbers to /out/fib.txt, then cat it.",
      "agent": "claude-code",
      "model": "sonnet",
      "egress": "package-managers"
    }
  }
}
```

- `prompt` — the task text handed to the agent. The agent runs inside a fresh sandbox and can write artefacts to `/out`.
- `agent` — `claude-code` (the default). The agent runs the Claude Code CLI inside the sandbox.
- `model` — `sonnet` (the default). Claude Sonnet is used for cost-efficiency on simple tasks.
- `egress` — `package-managers` allows the agent to reach npm/PyPI/conda registries in addition to the Anthropic API proxy (which is always reachable). Use `none` to lock down all egress except the API proxy.

**Note:** `egress: "open"` is not permitted for `sandbox_agent`. If you need unrestricted outbound access, use `sandbox_research` instead — but be aware that `sandbox_research` runs in a private workspace with no `/in` mounts. See [`../../docs/reference/nested-sandboxes.md`](../../docs/reference/nested-sandboxes.md) for the layout and conventions children follow.

### Run it

```bash
bash run.sh
```

See [run.sh](run.sh) for the full script.

### Artefacts

| File | Reference |
|------|-----------|
| `<output_dir>/fib.txt` | agent-produced output |
| `<output_dir>/usage.json` | [`../../docs/reference/usage-json.md`](../../docs/reference/usage-json.md) |
| `<output_dir>/results.json` | [`../../docs/reference/results-json.md`](../../docs/reference/results-json.md) |
