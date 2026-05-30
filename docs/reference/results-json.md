# `results.json` reference

Every `sandbox_agent` and `sandbox_research` run writes `results.json` to its output directory after the agent exits. The file rolls up the run's own indicative cost with the cost of every descendant sandbox, so the root run's `results.json` carries the whole tree's total. The source-of-truth struct is `Results` in `internal/sandbox/results.go`. The file is written atomically (write to `.results.json.tmp` then rename) after all children have finished — children always complete before their parent's tool call returns, so their `results.json` files exist by the time the parent rolls them up.

## Fields

| Field | JSON tag | Type | Description |
|-------|----------|------|-------------|
| Job ID | `job_id` | string | UUID identifying this specific run, assigned at spawn time. |
| Tool | `tool` | string | Name of the demesne tool that created this run (`"sandbox_agent"` or `"sandbox_research"`). |
| Name | `name` | string | Child name within the parent, if this run was spawned as a child (the `name` parameter). Omitted for root runs. |
| Depth | `depth` | integer | Nesting depth: `0` for a root run, `1` for a first-level child, and so on. No cap is enforced. |
| Exit code | `exit_code` | integer | Exit code the agent process returned. `0` indicates success. |
| Own cost | `own_usage_usd` | number | Indicative USD cost incurred by this run alone, read from `usage.json` after the agent exits. |
| Total cost | `total_usage_usd` | number | Sum of `own_usage_usd` and the `total_usage_usd` of every direct child (which themselves include their descendants). |
| Children | `children` | array of strings | Sorted list of child names that were successfully spawned under this run (each name corresponds to a subdirectory under `<output_dir>/child/<name>`). Omitted when there are no children. |

## Notes

- **`own_usage_usd` and `total_usage_usd` are indicative.** They are derived from `usage.json` which is computed from demesne's embedded pricing table, not from actual billing. For Claude Code OAuth users, real charges are against a Claude Console subscription. See [Indicative cost reporting](../explanation/key-concepts.md) and [`usage.json` reference](usage-json.md).
- **`total_usage_usd` sums descendants bottom-up.** Each child's `results.json` is written before its parent reads it; the parent reads every `<output_dir>/child/<name>/results.json` and sums their `total_usage_usd` values. This means the root run's `results.json` carries the cost of the entire descendant tree.
- **Written after children finish.** The write happens in `writeResults` in `internal/sandbox/results.go`, called from `runAgent` after `runAgent` returns — at which point all child tool calls within the agent have already completed, so all child `results.json` files are present.
- `results.json` is best-effort: write failures are silently dropped. The headline cost is also reported in the tool result text (`total_usage_usd` field), so the information is not lost even if the file write fails.

## Example

A parent `sandbox_agent` run with one child `sandbox_agent` named `"analyzer"`:

**`<output_dir>/child/analyzer/results.json`** (child, no children of its own):

```json
{
  "job_id": "b3f12c44-8a1e-4d77-9e32-1f0a6b7c2d88",
  "tool": "sandbox_agent",
  "name": "analyzer",
  "depth": 1,
  "exit_code": 0,
  "own_usage_usd": 0.0087,
  "total_usage_usd": 0.0087
}
```

**`<output_dir>/results.json`** (root, one child):

```json
{
  "job_id": "a1e09f33-5c2b-4810-8d41-9e7f3c1b4a22",
  "tool": "sandbox_agent",
  "depth": 0,
  "exit_code": 0,
  "own_usage_usd": 0.0124,
  "total_usage_usd": 0.0211,
  "children": ["analyzer"]
}
```

`total_usage_usd` in the root is `0.0124 + 0.0087 = 0.0211`.
