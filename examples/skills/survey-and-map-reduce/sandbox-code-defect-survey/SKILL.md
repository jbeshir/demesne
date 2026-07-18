---
name: sandbox-code-defect-survey
status: alpha
description: "Survey a mounted codebase for evidence-backed defect classes without changing it."
---

Produce a report-only survey of source-code defects. Do not edit, build, commit, or open a fix loop.

## Host agent

Launch one `sandbox_agent` with the repository in `directories`. Put the target domain and any prior quality work in its prompt. Do not name a model unless the host supplies a concrete allowed value; otherwise use the provider default.

## Orchestrator procedure

1. Enumerate `/in`; ignore `previous-jobs`; require exactly one intended repository directory. If selection is ambiguous or missing, write `/out/EXECUTIVE_SUMMARY.md` with `status: input-invalid` and stop. Copy it to `/workspace/repo`.
2. Spawn `research01` with `sandbox_research` to make a cited, domain-specific taxonomy. Wait until `status=succeeded` and `exit_code=0`; otherwise retry once as `research01-retry`. On a second failure, record `research` as a coverage gap and use only the mandatory `docs-code-alignment` axis.
3. Write `/out/TAXONOMY.md`. Spawn at most 12 `detect-<slug>` children, at most 8 in flight. Each reads `/workspace/repo`, confirms each finding from surrounding code, and writes `/out/REPORT.md` with `file:line`, excerpt, severity, confirmed/suspected status, and an improvement plan. A clean report is valid.
4. For every background job, call `sandbox_wait(job_id, timeout_seconds: 120)` until it is terminal. Accept it only when `status=succeeded`, `exit_code=0`, and `/out/child/<name>/REPORT.md` is nonempty. Retry a failed, cancelled, missing, or malformed report once under `<name>-retry`; then record that axis as a coverage gap. Do not claim coverage for it.
5. Copy each accepted child report from `/out/child/<name>/REPORT.md` to `/out/reports/<NN>-<slug>.md`. Write `/out/EXECUTIVE_SUMMARY.md` with taxonomy, per-axis counts, coverage gaps, cross-cutting themes, and a remediation order. State `DONE` only after every planned axis is accepted or recorded as a gap.

## Output contract

```
/out/TAXONOMY.md
/out/reports/NN-<slug>.md
/out/EXECUTIVE_SUMMARY.md
```
