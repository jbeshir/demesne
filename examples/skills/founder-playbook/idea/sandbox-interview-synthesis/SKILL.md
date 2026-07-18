---
name: sandbox-interview-synthesis
status: alpha
description: Synthesize mounted customer interviews into an evidence-cited verdict against a stated hypothesis. Use after interviews; not for interview design or outreach.
---

The invoked orchestrator owns every step below. Use native Demesne `sandbox_agent`, `sandbox_script`, `sandbox_wait`, and `sandbox_cancel` capabilities; omit `model` unless the host supplies a concrete schema-valid value.

## Procedure

1. Enumerate `/in`, excluding `/in/previous-jobs`. Use a prompt-provided mapping or validated contents to identify exactly one hypothesis mount and one interview corpus mount. Record discovered absolute paths in `/workspace/intake.json`. On zero or ambiguous matches, write `/out/FAILURE.md` and stop.
2. Create `/workspace/manifest.jsonl` in chronological `interview_seq` order. Log every unparseable, audio-only, or order-ambiguous item. Convert PDF/DOCX/RTF to text with `sandbox_script` before analysis; require a transcript for audio. Batch at most five interviews as `batch-01`, `batch-02`, and so on. Do not use a per-interview fallback.
3. Write `/workspace/op.md`, including the hypothesis and this required JSONL record schema: `item_id:string`, `source_path:string`, `interview_seq:integer`, `interviewee:string`, `confirmed:array<{claim:string,quote:string,note:string}>`, `challenged` with the same item schema, `surprised:array<{observation:string,quote:string}>`, and `leading_risk:string[]`. Every evidence item needs a nonempty verbatim quote.
4. Dispatch one background `debrief-batch-NN` child per batch. It writes `/out/debriefs.jsonl` and `/out/log.md`. Poll until terminal; accept only `status == succeeded`, `exit_code == 0`, both files present, and schema-valid records. Retry once as `debrief-batch-NN-r2`; on a second failure, record the batch/job/status/exit code/reason in `/out/failed-batches.jsonl`, write `/out/FAILURE.md`, and stop.
5. Dispatch fresh `audit-batch-NN` children only after their accepted debrief sibling completes. Each reads the matching completed-sibling debrief, writes `/out/audit.md` and `/out/audit.jsonl`, and flags imbalance when supporting evidence is at least twice challenging evidence or fewer than two substantive challenging entries occur in a five-interview batch. Apply the step-4 gate and retry policy.
6. Dispatch `synthesis` only after every accepted audit is available. It writes `/out/SYNTHESIS.md` with one of `SUPPORTED`, `MIXED-REFRAME`, `NOT-SUPPORTED`, or `CONFIRMATION-BIAS-WARNING`; systemic imbalance requires the last verdict. Gate it as in step 4.
7. The parent creates `/out/audits`, copies each accepted child audit to `/out/audits/audit-batch-NN.{md,jsonl}`, concatenates debriefs in batch order to `/out/debriefs.jsonl`, copies synthesis, manifest, and failure record if present, and verifies every expected source before delivery. Write `/out/SUMMARY.md` with processed/skipped counts, batches, flags, and any abort reason.

## Output contract

On success deliver `SYNTHESIS.md`, `debriefs.jsonl`, `audits/audit-batch-NN.md`, `audits/audit-batch-NN.jsonl`, `manifest.jsonl`, and `SUMMARY.md`. On failure deliver `FAILURE.md`, `failed-batches.jsonl`, and `SUMMARY.md`; do not claim a complete synthesis.
