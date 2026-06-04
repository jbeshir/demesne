# `transcript.jsonl`

The agent's redirected stdout, written as newline-delimited JSON (NDJSON). Every `sandbox_agent` and `sandbox_research` run produces one.

## Location

| Context | Path |
|---------|------|
| Inside the sandbox | `/out/transcript.jsonl` |
| On the host | `<output_dir>/transcript.jsonl` |

`output_dir` is returned in the MCP result of every `sandbox_agent` / `sandbox_research` call and corresponds to the sandbox's `/out` on the host filesystem.

## Contents

Each line is a JSON object emitted by the agent CLI with NDJSON / stream-json output enabled.

**claude-code** (default agent): invoked with `--output-format stream-json --verbose`. Event shapes:

- `{"type":"assistant","message":{...}}` — intermediate assistant turn; `message.content` is an array of content blocks (`text`, `tool_use`, etc.).
- `{"type":"result","subtype":"success","result":"...","usage":{...}}` — terminal event; `result` holds the final answer, emitted once at exit.
- Other event types (tool_result, system, etc.) may appear; unrecognised lines are silently skipped by the parser.

A typical result-event line:

```json
{"type":"result","subtype":"success","result":"Done. Written to /out/report.txt.","usage":{"input_tokens":1234,"output_tokens":56,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}
```

For the full event schema see the Claude Code CLI reference or the parser source at `internal/agents/anthropic/streamjson.go`.

**codex** (experimental agent): invoked with `--json`. The terminal event is `{"type":"item.completed","item":{"type":"agent_message","text":"..."}}`. Parser source: `internal/agents/codex/streamjson.go`.

## How `ResultText` extracts the final answer

`ResultText` in the agent-provider package scans the transcript line by line:

- **claude-code** (`internal/agents/anthropic/streamjson.go`): prefers the `result` field of the terminal `{"type":"result"}` event; falls back to concatenating text from all `{"type":"assistant"}` events if no result event is present (e.g. the run errored).
- **codex** (`internal/agents/codex/streamjson.go`): returns the text of the last `item.completed` / `agent_message` event.

Malformed lines are silently skipped in both parsers.

## Relationship to the MCP `stdout` result field

The `stdout` field in the MCP result is the *parsed final answer* from `ResultText` — not the raw transcript. It is bounded by `tailStdout` to the last **32 KiB**. When truncated, the field ends with:

```
[stdout truncated to last N bytes; full transcript in <output_dir>/transcript.jsonl]
```

The transcript file is the authoritative full record. Read it when:

- The `stdout` field contains the truncation marker.
- You need intermediate reasoning steps or tool-call traces (e.g. a verifier child needs the full reasoning trail — see [Spawning a verifier/judge child](../../how-to/spawn-nested-agents.md#spawning-a-verifierjudge-child)).
