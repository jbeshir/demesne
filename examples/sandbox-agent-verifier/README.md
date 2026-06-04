# Sandbox agent — worker + verifier

This example demonstrates the verifier pattern: a worker `sandbox_agent` produces an artefact, then a second `sandbox_agent` (the verifier) evaluates it against explicit criteria. The verifier reads both the artefact and the worker's `transcript.jsonl` reasoning trace.

## Prerequisites

Same as [sandbox-agent-hello](../sandbox-agent-hello/README.md): demesne running with `DEMESNE_CLAUDE_CODE_OAUTH_TOKEN` set and OpenSandbox configured.

## The pattern

```
caller
 ├── sandbox_agent (worker)  →  /out/haiku.txt + transcript.jsonl
 └── sandbox_agent (verifier, directories=[worker output_dir])
                             →  /out/verdict.txt
```

The verifier mounts the worker's `output_dir` read-only. Because demesne output directories are named `out`, the contents appear at `/in/out/` inside the verifier sandbox:

- `/in/out/haiku.txt` — the worker's artefact
- `/in/out/transcript.jsonl` — the worker's full reasoning trace (see [`../../docs/reference/transcript-jsonl.md`](../../docs/reference/transcript-jsonl.md))

## Why a separate verifier instead of self-critique

An external judge has a fresh context window and no stake in the worker's output — it cannot rationalise away errors it did not produce. See [Spawning a verifier/judge child](../../docs/how-to/spawn-nested-agents.md#spawning-a-verifierjudge-child) for the in-agent version of this pattern (where both agents are siblings and the judge reads via `/in/previous-jobs/<name>/`).

## Worker call

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "sandbox_agent",
    "arguments": {
      "prompt": "Write a haiku about distributed systems to /out/haiku.txt.",
      "model": "haiku",
      "output_path": "/out/haiku.txt",
      "success_criteria": ["haiku.txt exists", "exactly three lines", "5-7-5 syllable structure"]
    }
  }
}
```

## Verifier call

After extracting `output_dir` from the worker result, pass it as a `directories` mount:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "sandbox_agent",
    "arguments": {
      "preamble": "You are a strict haiku judge. Do not modify files.",
      "prompt": "Read /in/out/haiku.txt. Count syllables. Read /in/out/transcript.jsonl for the worker reasoning trace. Write PASS or FAIL + one sentence to /out/verdict.txt.",
      "model": "haiku",
      "directories": ["<worker output_dir>"],
      "output_path": "/out/verdict.txt",
      "success_criteria": ["verdict.txt exists", "first word is PASS or FAIL"]
    }
  }
}
```

## Run it

```bash
bash run.sh
```

The script runs both calls sequentially, extracts `output_dir` from the worker response with `jq`, and injects it into the verifier call.
