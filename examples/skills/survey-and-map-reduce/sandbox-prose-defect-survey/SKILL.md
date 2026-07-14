---
name: sandbox-prose-defect-survey
status: alpha
description: "Survey mounted prose for evidence-backed writing defects without rewriting it."
---

Produce a report-only prose audit; do not modify the prose or route it into a rewrite.

1. The host launches one `sandbox_agent` with the target directory mounted. Enumerate `/in`, ignore `previous-jobs`, and bind exactly one intended prose directory. On ambiguity, write `/out/EXECUTIVE_SUMMARY.md` with `status: input-invalid` and stop.
2. Write `/workspace/manifest.jsonl` and a finite lens list with measurable checks. Split input into deterministic shards no larger than 100,000 estimated tokens.
3. Dispatch `detect-<lens>-<NN>` children, at most 8 in flight. Give each its exact files, lens, and finding schema. Each writes `/out/REPORT.md` with location, quoted evidence, severity, and suggested revision direction; it explicitly says clean when appropriate.
4. For each job, call `sandbox_wait` until terminal. Accept only `status=succeeded`, `exit_code=0`, nonempty report, and valid finding records. Retry once with a new `-retry` name. After that, record the lens/shard as a coverage gap; do not represent it as reviewed.
5. Spawn `reducer` after all jobs are accepted or gaps are recorded. It reads accepted reports under `/in/previous-jobs/<name>/REPORT.md`, writes `/out/EXECUTIVE_SUMMARY.md` and `/out/reports-index.jsonl`, and names every gap. Validate it; retry once as `reducer-retry`; on a second failure, write `status: reducer-failed` and stop.
6. Copy accepted reports and reducer artifacts from child paths to parent `/out`. The summary reports only validated coverage and does not claim code, factual, or accessibility testing.

## Output contract

```
/out/EXECUTIVE_SUMMARY.md
/out/reports-index.jsonl
/out/reports/NN-<lens>-<shard>.md
```
