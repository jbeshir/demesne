---
name: sandbox-corpus-map-reduce
description: Apply the SAME extraction, scoring, or tagging operation to EVERY item in a corpus of files (PDFs, logs, transcripts, contracts, papers, code files) and reduce the per-item outputs to a ranked, tabulated, or summarised answer. A slow-tier orchestrator builds a manifest, writes the per-item op spec with a locked output schema, shards the corpus, fans out medium-tier map children (batches ≤4), then a separate slow-tier reducer concatenates per-item records and synthesises REPORT.md plus data.jsonl. Apply when the user has a directory of documents and wants the same question answered for each — "extract all claims from every paper", "score each contract for X compliance", "tag every transcript's methodology", "find X across all files in this folder", "build a ranked table from this collection", "summarise every item". Skip when the corpus is a single item (call sandbox_agent directly), when the goal is finding defect types in a codebase (use sandbox-code-defect-survey or sandbox-prose-defect-survey), or when open-web research drives the analysis rather than mounted local files (use sandbox-product-research).
allowed-tools: Read, Glob, Grep, Bash, Write, Edit, mcp__demesne__sandbox_agent, mcp__demesne__sandbox_script
---

Apply a uniform extraction, scoring, or tagging operation across an entire corpus of mounted files, then reduce per-item outputs to a synthesis report. You author one orchestrator prompt, launch a slow-tier `sandbox_agent`, and it runs the full pipeline autonomously. The deliverable is `/out/REPORT.md` and `/out/data.jsonl`; there is no code landing.

**Watch out (cross-cutting):** This is a report-only pipeline: no edits, no commits, no branches. Scripts and data work run in demesne, never on the host; the host's only role is reading `/out/REPORT.md` and deciding on follow-up.

## Procedure

1. **Intake.** List every item in `/in/<corpus>` and write `/workspace/manifest.jsonl` (one record per item: `{"path": "/in/<corpus>/...", "size_bytes": N, "type": "pdf|txt|jsonl|..."}`). Note items that cannot be identified.

2. **Write the op spec at `/workspace/op.md`.** Include the per-item operation in plain English and the **locked JSONL schema** that every `extracted.jsonl` must conform to. Schema must include `item_id`, `source_path`, and all operation-specific fields as mandatory keys. Vague questions produce vague schemas; over-specify the rule.

3. **Shard the manifest.** Slice into S shards sized by context budget, targeting ~100K tokens of item content per shard. Heuristic: PDFs ~10 items/shard; medium prose (contracts, reports) ~20–50; dense academic papers ~5–10; one-line log records ~hundreds to thousands per shard. Write each shard as `/workspace/shard-NN.jsonl` (a list of paths). If total shard count exceeds 4, plan sequential batches of ≤4.

   If the corpus needs **decompression or format conversion** before extraction, or if staging per-shard chunks is what keeps each map child's context budget in range, run a pre-shard `sandbox_script` (`image=python`, `egress=package-managers`) that converts and stages the corpus into `/workspace/shards/<NN>/`; map children then read from `/workspace/shards/` instead of `/in/<corpus>`. Mount size alone is not a reason to pre-process — demesne bind-mounts `/in` read-only regardless of corpus size.

4. **MAP — spawn one medium-tier `sandbox_agent` per shard** (`name=map-01`, `map-02`, …; lowercase DNS-1123: letters, digits, interior hyphens only, ≤40 chars). Spawn in batches of ≤4 — a recommended batch size, not a demesne-enforced cap.

   **Schema lock:** finalise the schema in `op.md` before any mapper runs. A child emitting `{"claim": ...}` while others emit `{"claims": ...}` produces unmerge-able shards the reducer cannot repair.

   Each child's prompt must embed: its exact path list (the orchestrator decides which paths each child is responsible for — demesne does not split the corpus automatically), the full `/workspace/op.md`, the locked schema verbatim, and instructions to write `/out/extracted.jsonl` (one record per item, schema-compliant) and `/out/log.md` (items skipped, parse errors, anomalies with reason). `log.md` is required — a child that silently skips items produces a report that claims completeness it does not have. Context budget: a child whose context fills mid-corpus silently truncates; `log.md` is the only signal.

5. **REDUCE — spawn one slow-tier `sandbox_agent` (`name=reducer`) only after all map batches complete.** The reducer reads siblings' outputs via `/in/previous-jobs/map-NN/extracted.jsonl`; that mount registers at child create but files appear only once the sibling completes. Spawning the reducer concurrently with the last batch leaves some shards absent.

   The reducer concatenates all `extracted.jsonl` files into `/workspace/all.jsonl` (flagging but not dropping schema-divergent records), then synthesises `/out/REPORT.md` (answer to the original question, citing `item_id`s for every claim) and `/out/data.jsonl` (cleaned concatenation, anomalies annotated). The reducer never does per-item extraction — it only reduces. Keep it separate: a map child's context holds raw items; collapsing reduction into it short-changes either the items or the synthesis.

   If all `extracted.jsonl` files combined exceed ~150K tokens, add one intermediate-reduce tier: group map outputs into batches of ≤4, one intermediate reducer per group, then a final reducer reads the intermediate outputs. Cap at depth 2; deeper indicates the op schema is too wide or the corpus too large for this pipeline without narrowing the question.

6. **Deliver.** In the orchestrator's own process, `cp` the reducer's `/out/REPORT.md` and `/out/data.jsonl` into the orchestrator's `/out`. Do not delegate this copy to a `sandbox_script` child — its `/out` is `/out/child/<name>` and would strand the files. Also write `/out/manifest.jsonl` (corpus listing from step 1) and `/out/SUMMARY.md` (items processed, skipped, schema-drift flags, map children spawned). Print `DONE`.

## Writing the orchestrator prompt

Brief it as a complete document:

1. **The corpus and the question** — what the files are, where mounted (`/in/<corpus>`), and the exact question to answer. Vague questions produce vague schemas.
2. **The op spec requirement** — write `/workspace/op.md` with the per-item operation in plain English AND the locked JSONL schema before spawning any map child. Schema must include `item_id`, `source_path`, and all operation-specific fields.
3. **Shard sizing rule** — context-budget calculation, not a fixed M. Embed the heuristic (PDF: ~10/shard; medium prose: ~20–50; log lines: hundreds to thousands/shard). If total shard count > 4, run map children in sequential batches of ≤4.
4. **Map child prompt discipline** — embed exact path list, full op spec, schema verbatim, instructions to write `extracted.jsonl` and `log.md`, and a "do not silently skip items — log everything that fails" requirement. Schema compliance must be enforced in the prompt; the reducer cannot repair structural drift.
5. **Reducer brief** — spawn only after all map batches complete. Two passes: concatenate then synthesise. Report must cite `item_id`s; do not assert cross-item conclusions without citing specific records.
6. **Pre-process path** — when the corpus needs decompression or format conversion, or when staging chunks is required to keep map child context budgets in range.
7. **Output contract** — the files listed below; report-only, no edits, builds, or commits.

## Output contract

```
/out/
  REPORT.md          # synthesis: the answer to the original question (cites item_ids)
  data.jsonl         # per-item extracted records (concatenated, anomalies annotated)
  manifest.jsonl     # the corpus listing (path, size_bytes, type per item)
  SUMMARY.md         # run summary: items processed/skipped, schema flags, child count
```

`REPORT.md` sections in order: **TL;DR** (3–5 bullet answer, written last, placed first), **Methodology** (op spec summary, shard count, items total / skipped), **Findings** (substantive answer with `item_id` citations), **Anomalies** (items that couldn't be parsed or had schema drift), **Full Data** (pointer to `data.jsonl`).

## Launching the orchestrator

- **`directories: ["<abs path to corpus>"]` is mandatory.** Forgetting it mounts nothing — all map children wake up with no items to read and the pipeline produces nothing.
- Map children inherit this mount and read their slice of `/in/<corpus>` directly.
- Tier: **slow** for the orchestrator and reducer; **medium** for map children.
- Child names: `map-01`, `map-02`, …, `reducer`. Lowercase letters, digits, interior hyphens only, ≤40 chars.