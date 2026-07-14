---
name: sandbox-interview-kit-design
status: alpha
description: Build an audited customer-discovery interview kit from a validated hypothesis. Produce documents only; outreach and scheduling are out of scope.
---

Use native Demesne `sandbox_agent`, `sandbox_research`, `sandbox_wait`, and `sandbox_cancel` capabilities. Omit `model` unless the host supplies a concrete schema-valid model.

## Procedure

1. Enumerate `/in`, excluding `/in/previous-jobs`; identify exactly one hypothesis mount by expected contents. If it is absent or ambiguous, write `/out/FAILURE.md` and stop. Copy it to `/workspace/hypothesis.md`; derive 1–3 personas and write `/workspace/profile.md` and `personas.json`.
2. Dispatch `reach-research` and one `drafter-<slug>` per persona in the background, at most 8 at once. Research writes child-local `/out/reachability.md`; every drafter writes `/out/questions.md`.
3. For every barrier, poll each job until terminal and accept only `status == succeeded`, `exit_code == 0`, and its declared output. Retry once with a unique `-r2` name. On any second failure, write `/out/FAILURE.md` with job ID/status/exit code and stop.
4. After each accepted drafter, dispatch a fresh `auditor-<slug>`; it writes `/out/questions-clean.md` and `/out/audit.jsonl`. Each audit line has required keys and types: `persona:string`, `q_id:integer`, `original:string`, `modes:string[]` (members `leading|future-facing|too-broad`), `verdict:string` (`keep|revise|cut`), `rewrite:string|null`, `probe:string|null`, `learns:string`. A `cut` record has null rewrite/probe; other records have nonempty rewrite/probe.
5. If `3 * (cut_count + revise_count) >= total_question_count`, run one additional drafter/auditor round for that persona. Use `drafter-<slug>-r2` and `auditor-<slug>-r2`; two rounds is the maximum. For revised personas, compile only `auditor-<slug>-r2/questions-clean.md`; otherwise use r1. Preserve both audit rounds in the trail with a required `round: 1|2` field.
6. After accepted auditors and reachability complete, dispatch `compiler`. It writes `/out/KIT.md`. Gate it as in step 3. The parent creates `/out/questions`, copies final child-local files into its own `/out`, concatenates audit records in persona then round order, and verifies every copy/concatenation succeeded.

## Output contract

Deliver `/out/KIT.md`, `/out/profile.md`, `/out/reachability.md`, `/out/questions/<slug>.md`, and `/out/audit-trail.jsonl`. Questions target past/present behaviour and contain no solution pitch.
