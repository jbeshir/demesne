---
name: sandbox-etl-document
status: alpha
description: "Extract a mounted document batch into a schema-validated store with explicit quarantine."
---

Build a reportable data store, not product code. Never silently drop an item.

1. The host launches one `sandbox_agent` with the document directory mounted. The orchestrator enumerates `/in`, ignores `previous-jobs`, and requires exactly one intended document directory. On ambiguity, write `/out/SUMMARY.md` with `status: input-invalid` and stop.
2. Write `/workspace/schema.json` and `/workspace/taxonomy.md` before parsing. The schema defines every type, requiredness, confidence threshold, and evidence field. A web taxonomy may use `sandbox_research`; wait for a succeeded, exit-zero, nonempty output, retry once as `taxonomy-retry`, then stop with `status: taxonomy-failed` if still unavailable.
3. Run named parse and shard scripts. Default to `egress=none`; use `package-managers` only for a named unavailable parser. Each accepted stage must be terminal `succeeded`, exit 0, and produce nonempty declared artifacts. Retry it once under a new name; on failure, write a reasoned quarantine record and stop if its output is a required input.
4. Dispatch `extract-<NN>` children (at most 8 in flight), each owning one shard and writing one schema-shaped record per input plus evidence and confidence. Wait to terminal, validate JSONL, required record count, and schema; retry once as `extract-<NN>-retry`. Quarantine a failed shard with all item IDs and its reason.
5. Dispatch enrichment only for accepted extraction shards, then validation/load sequentially. Apply the same succeeded/exit-zero/artifact/shape gate and one-retry rule to every phase. The loader writes `/out/store/` and `/out/quarantine.jsonl`; it must identify every input as loaded or quarantined.
6. Copy accepted load output from `/out/child/<name>/store/` to parent `/out/store/`. Write `/out/SUMMARY.md` with phase outcomes, loaded count, quarantined count, retried jobs, and any incomplete coverage. Print `DONE` only when the output contract validates.

## Output contract

```
/out/store/
/out/quarantine.jsonl
/out/SUMMARY.md
```
