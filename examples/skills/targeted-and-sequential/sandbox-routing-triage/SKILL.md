---
name: sandbox-routing-triage
status: alpha
description: "Classify a mounted heterogeneous batch and route each item without silently dropping it."
---

Route; do not solve the work. Every item ends in a dispatched, quarantined, escalated, or handler-error state.

1. The host mounts the batch. The orchestrator enumerates `/in`, ignores `previous-jobs`, and selects exactly one intended batch directory. On ambiguity, write `/out/SUMMARY.md` `status: input-invalid` and stop. Write `/workspace/queue.jsonl` with every item; unreadable items become `escalate-human`.
2. Write a closed `/workspace/classes.md` before classification. Each class has a slug, handler contract, and numeric threshold. `escalate-human` has no child and always goes to quarantine. Omit `model` unless the host supplies a concrete allowed model.
3. Classify using `classify-<NN>` jobs; shard only above 150 items. Each writes schema-valid routes `{id,class,confidence,reason}`. Wait through `running` results and accept only `succeeded`, exit 0, nonempty valid routes. Retry once under a new name; on a second failure quarantine every affected item with `status: classifier-error`.
4. Write parent `/out/routes.jsonl` before handlers. Dispatch one `handler-<class>` per eligible class, at most 8 in flight. A handler receives only its slice and writes `/out/results/`. For each job require terminal success, exit 0, and nonempty result directory; retry once with a unique `-retry` name. On a second failure, mark every associated route `handler-error` and quarantine it. Do not use a `jq`-specific requirement; grouping may use any available deterministic parser.
5. Copy accepted child results from `/out/child/<name>/results/` to parent `/out/results/<class>/`. Write `/out/quarantine.jsonl`, finalize `/out/routes.jsonl`, and write `/out/SUMMARY.md` with item counts, retries, errors, and coverage. Print `DONE` only after every input item has exactly one final status.

## Output contract

```
/out/SUMMARY.md
/out/routes.jsonl
/out/quarantine.jsonl
/out/results/<class>/
```
