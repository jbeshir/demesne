---
name: sandbox-competitor-complaint-mining
status: alpha
description: Mine public competitor complaints into a cited, deduplicated report. Use for discovery evidence; do not contact users or modify product code.
---

Use native Demesne capabilities: `sandbox_agent`/`sandbox_research`, `sandbox_wait`, `sandbox_cancel`, and a parent-side file copy. Do not require provider-specific tool aliases. Omit `model` unless the host supplies a concrete schema-valid model; never pass a tier label as `model`.

## Procedure

1. Enumerate `/in`, excluding `/in/previous-jobs`; identify the single competitor list or brief by its declared manifest/content. If zero or multiple candidates match, write `/out/FAILURE.md` and stop. Copy the selected input to `/workspace/input.md`.
2. Normalize each competitor name for a child name: lowercase; replace every non-alphanumeric run with `-`; trim edge hyphens; truncate the base to 32 characters; append `-` plus the first 7 hex characters of SHA-256 of the original name. Reject an empty result. Record original name, normalized name, and job ID in `/workspace/jobs.jsonl`.
3. Dispatch one background research/gather child per competitor. A child writes only `/out/extracted.jsonl` and `/out/log.md`; the parent sees these at `/out/child/<name>/`. Keep at most 8 jobs in flight.
4. Wait for every job with `sandbox_wait(job_id)` using its long default; repeat only if it returns `running`. Accept a result only when `status == succeeded`, `exit_code == 0`, and both declared files exist. Retry a failed, cancelled, or missing-output job once with a new unique `-r2` name. On a second failure, append `{name, job_id, status, exit_code, reason}` to `/workspace/quarantine.jsonl`; do not treat it as evidence.
5. If any requested competitor is quarantined, deliver `/out/FAILURE.md` and `/out/quarantine.jsonl`; stop. Otherwise dispatch a compiler after all accepted children complete. It writes `/out/REPORT.md`, `/out/data.jsonl`, `/out/ranked.jsonl`, `/out/target.md`, and `/out/SUMMARY.md`.
6. Gate the compiler. The parent copies those five outputs unchanged into its own `/out`, verifies all exist, and parses every JSONL line.

## Output contract

Deliver `REPORT.md`, `data.jsonl`, `ranked.jsonl`, `target.md`, and `SUMMARY.md`. Each data record requires `complaint_id`, `competitor`, `source`, `source_url`, `quote`, `complaint_theme`, `resolved_status`, and `severity_note`; uncited records are excluded. The report scores each ranked complaint `directly|partially|does-not|orthogonal` against an exact hypothesis clause. End at delivery.
