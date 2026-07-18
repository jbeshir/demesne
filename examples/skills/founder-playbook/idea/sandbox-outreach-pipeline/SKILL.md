---
name: sandbox-outreach-pipeline
status: alpha
description: Prepare a review-ready, personalized outreach batch and tracking file from an interview kit and seed list. Do not send messages, schedule meetings, or modify external accounts.
---

Use native Demesne sandbox capabilities; omit `model` unless the host supplies a concrete schema-valid value.

## Procedure

1. Enumerate `/in` excluding `/in/previous-jobs`. Identify exactly one interview kit and one seed list by manifest/expected content; record their paths in `/workspace/intake.json`. Accept seed CSV or JSON only. On missing, ambiguous, or unsupported input, write `/out/FAILURE.md` and stop.
2. Process seeds in exact batches of 20. Dispatch background research/drafting children with unique DNS-1123 names. A child prompt names only its child-local destination: research `/out/evidence.jsonl`, drafter `/out/drafts.jsonl`, auditor `/out/verdicts.jsonl`, tracking builder `/out/tracking.csv`.
3. At every barrier, re-poll `running` jobs. Accept only `status == succeeded`, `exit_code == 0`, and the declared file. Retry each failed/cancelled/missing-output job once with a unique `-r2` name; on second failure, write job details to `/out/FAILURE.md` and stop.
4. Require each draft to cite at least one seed-specific evidence field. An auditor marks `PASS` only when it contains a named recipient, one cited evidence field, a single clear ask, and no unsupported claim; otherwise it marks `REVISE` with a reason. Dispatch `outreach-audit-r2` for revised drafts; its verdict, not r1, is authoritative.
5. Dispatch `build-package` after final audit. It writes `/out/outreach/<prospect_id>.md`, `/out/cadence.md`, `/out/tracking.csv`, `/out/audit.md`, and `/out/evidence.jsonl`; gate every output.
6. Parent-copy the accepted package into its own `/out`; verify every output and match tracking rows to prospect files. Write `/out/SUMMARY.md`; hand off to a separately authorized sending workflow.

## Output contract

Deliver `/out/outreach/<prospect_id>.md`, `/out/cadence.md`, `/out/tracking.csv`, `/out/audit.md`, `/out/evidence.jsonl`, and `/out/SUMMARY.md`; or `/out/FAILURE.md` on an aborted batch. Each draft carries recipient/channel/subject metadata and its cited personalization evidence.
