# Sandbox agent — one prompt, one artefact

This example fires a single `sandbox_agent` call that asks the agent to write a small Python script, run it, and deposit the output at `/out/fib.txt`. It demonstrates how to invoke a one-shot agent sandbox and where to find the token-usage and cost artefacts that demesne produces alongside the agent's own output.

## Prerequisites

- demesne is running with `DEMESNE_CLAUDE_CODE_OAUTH_TOKEN` set. See the ["Getting an OAuth token (claude-code provider)"](../../docs/explanation/trust-boundary.md) section of `docs/explanation/trust-boundary.md` for how to obtain one.
- OpenSandbox is running and `OPEN_SANDBOX_DOMAIN` / `OPEN_SANDBOX_API_KEY` are set on the demesne process.

## The call

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

**Note:** `egress: "open"` is not permitted for `sandbox_agent`. If you need unrestricted outbound access, use `sandbox_research` instead — but be aware that `sandbox_research` runs in a private workspace with no `/in` mounts. See [`../../docs/how-to/spawn-nested-agents.md`](../../docs/how-to/spawn-nested-agents.md) for a discussion of when to use which.

## Artefacts produced

After the agent exits, the `output_dir` returned in the result contains:

- **`<output_dir>/usage.json`** — per-model token counts and indicative cost for this run. See [`../../docs/reference/usage-json.md`](../../docs/reference/usage-json.md) for the field-level reference.
- **`<output_dir>/results.json`** — own cost plus a rolled-up cost tree covering this run and any child sandboxes it spawned. See [`../../docs/reference/results-json.md`](../../docs/reference/results-json.md) for the field-level reference.
- **`<output_dir>/fib.txt`** (or whatever the agent writes to `/out`) — depends on the agent's behaviour. In this example the prompt asks for `fib.txt`, so you should find it there.

## Indicative cost

The `cost_usd` field in the result (and in `usage.json`) is **indicative**. Demesne computes it from an embedded pricing table and the token counts reported by the API. For Claude Code OAuth users, billing is against a Claude Console subscription rather than on a per-request basis, so the figure is useful for relative cost tracking but is not what is actually charged. See the `cost_usd` note in [`../../docs/reference/usage-json.md`](../../docs/reference/usage-json.md).

## Run it

```bash
bash run.sh
```

See [run.sh](run.sh) for the full script.
