---
name: sandbox-corpus-map-reduce
status: alpha
description: "Apply a locked per-item operation to a mounted corpus and synthesize its results."
---

Produce a report and data set; do not edit the corpus, commit, or start a fix workflow.

## Procedure

1. Enumerate `/in`, excluding `previous-jobs`. Identify exactly one directory matching the host-supplied corpus identity; on zero or multiple matches, write `/out/SUMMARY.md` with `status: input-invalid` and stop. Use the discovered path throughout.
2. Write `/workspace/manifest.jsonl` and `/workspace/op.md` before starting work. The op file locks a JSONL schema with mandatory `item_id`, `source_path`, and operation fields. Assign every item deterministically to one shard; use at most 100,000 estimated input tokens per shard. Record unassignable items in the manifest.
3. Use `sandbox_script` with `egress=none` for local staging. Permit `package-managers` only for a named missing converter and record it. Each map child receives one exact shard and writes `/out/extracted.jsonl` and `/out/log.md`.
4. Dispatch `map-<NN>` children with at most 8 in flight. Use the provider default model unless the host supplies a concrete allowed `model`. For each job, wait until terminal; require `succeeded`, exit code 0, nonempty `extracted.jsonl`, and valid JSONL records matching the locked schema. Retry once as `map-<NN>-retry`. After a second failure, quarantine that shard in `/workspace/failures.jsonl`; never silently omit it.
5. Spawn `reducer` only after all shards are accepted or quarantined. It reads accepted outputs from `/in/previous-jobs/<name>/extracted.jsonl`, writes `/out/REPORT.md` and `/out/data.jsonl`, and flags each quarantined shard. Apply the same terminal, exit-code, nonempty-output check and one retry (`reducer-retry`). If both fail, write `/out/SUMMARY.md` with `status: reducer-failed` and stop.
6. Copy accepted reducer artifacts from `/out/child/<name>/` to the parent `/out`; copy the manifest. Write `/out/SUMMARY.md` with totals, accepted shards, quarantined shards, schema failures, and actual coverage.

## Output contract

```
/out/REPORT.md
/out/data.jsonl
/out/manifest.jsonl
/out/SUMMARY.md
```
