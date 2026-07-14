---
name: sandbox-enterprise-gap-analysis
status: alpha
description: "Compare verified product evidence with a named enterprise buyer’s requirements and deliver a ranked, evidence-linked gap plan. Do not promise compliance or implement changes."
---

Discover mounts at runtime, selecting product/current-state evidence and buyer requirements by manifest inspection; record choices in `/workspace/intake.md` and stop on ambiguity. Host paths are absolute and mount under their basename.

Use documented tool parameters and omit `model` unless a host supplies a supported concrete value. Run nested research or analysis stages synchronously. Accept only `succeeded`, exit code 0, and nonempty expected output; retry once then record a quarantine/failure row and mark the corresponding requirement unassessed. Parent-copy accepted outputs to `/out`.

1. Normalize requirements into `requirements.jsonl` with `{id,requirement,source,priority:"must"|"should"|"nice",evidence_needed}`.
2. Inventory evidence deterministically where possible; log unreadable files. For each requirement, classify `MET`, `PARTIAL`, `GAP`, or `UNASSESSED`; `MET` requires a path and quoted evidence.
3. Have a fresh reviewer challenge every MET/PARTIAL claim. Revise once from accepted findings; unresolved claims become UNASSESSED, never MET.
4. Deliver `ENTERPRISE-GAP-ANALYSIS.md`, `gap-backlog.json`, `evidence-map.jsonl`, and `SUMMARY.md`. Backlog items include owner, acceptance test, dependency, effort, buyer impact, and source requirement.

Require agent and deterministic-script capabilities at launch; map capability names to the active host. No code landing.
