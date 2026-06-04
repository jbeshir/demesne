#!/usr/bin/env bash
set -euo pipefail

DEMESNE=${DEMESNE_MCP:-demesne-mcp}
DIR=$(dirname "$0")
TMPFILE=$(mktemp)
trap 'rm -f "$TMPFILE"' EXIT

mcp_init() {
  printf '%s\n' '{"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"demesne-example","version":"1.0"}}}'
  printf '%s\n' '{"jsonrpc":"2.0","method":"notifications/initialized"}'
}

# ── Step 1: run worker ───────────────────────────────────────────────────────
printf '==> Running worker...\n'
(mcp_init; cat "$DIR/worker-request.json") | "$DEMESNE" > "$TMPFILE"
cat "$TMPFILE"

OUTPUT_DIR=$(grep '"id":1' "$TMPFILE" | jq -r '.result.structuredContent.output_dir')
printf '\n==> Worker output_dir: %s\n' "$OUTPUT_DIR"

# ── Step 2: run verifier ─────────────────────────────────────────────────────
# Mount the worker's output_dir read-only into the verifier at /in/<basename>.
# The basename of a demesne output_dir is "out", so haiku.txt lands at
# /in/out/haiku.txt and transcript.jsonl lands at /in/out/transcript.jsonl.
printf '\n==> Running verifier...\n'
VERIFIER_REQ=$(jq -n --arg dir "$OUTPUT_DIR" '{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "sandbox_agent",
    "arguments": {
      "preamble": "You are a strict haiku judge. Do not modify files.",
      "prompt": "Read /in/out/haiku.txt. Count syllables on each line. Also read /in/out/transcript.jsonl for the worker reasoning trace. Write PASS or FAIL followed by one sentence explaining your verdict to /out/verdict.txt.",
      "model": "haiku",
      "directories": [$dir],
      "output_path": "/out/verdict.txt",
      "success_criteria": [
        "verdict.txt exists",
        "first word is PASS or FAIL",
        "reason is one sentence"
      ]
    }
  }
}')
(mcp_init; printf '%s\n' "$VERIFIER_REQ") | "$DEMESNE"
