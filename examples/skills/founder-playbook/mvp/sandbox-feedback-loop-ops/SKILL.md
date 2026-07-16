---
name: sandbox-feedback-loop-ops
status: alpha
description: "Turn founder-reviewed feedback into a weekly, auditable learning loop. Use after human review; do not use it to send outreach or approve product changes."
allowed-tools: mcp__demesne__sandbox_agent, mcp__demesne__sandbox_script, mcp__demesne__sandbox_wait
---

Require the host to provide reviewed feedback. Required capabilities are child agents, deterministic scripts, waiting, and file copies; map capability names to the host before launch. Do not treat an allowlist as a safety boundary.

## Procedure

1. Discover mounts at runtime: enumerate `/in/*`, match each requested input by launch-supplied label and expected contents, and fail on zero or multiple matches. Record the resolved inbox, optional scope document, reviewed synthesis, and history paths in `/workspace/inputs.md`. Never assume a mount basename.
2. Normalize the inbox with a deterministic script into `/workspace/normalized.jsonl`; write rejected rows and reasons to `/workspace/normalize-log.md`. Set routing confidence to `0.60` unless the launch prompt supplies a value in `[0,1]`; record the value in `/workspace/classes.md`.
3. Dispatch classifiers as `classify-shard-01`, `classify-shard-02`, … (DNS-1123, unique) with `background:true`. Each writes `/out/classified.jsonl`. For every job, wait again while `running`; accept only `succeeded`, `exit_code == 0`, and an existing `classified.jsonl`. Retry once with a distinct `-r2` name. Otherwise write a failure record to `/workspace/quarantine.jsonl` and exclude that shard.
4. Build `/workspace/classes.md` from accepted shards. Dispatch one handler per class as `handle-<class-slug>`; each writes `/out/handled.jsonl`. Apply the same success, one-retry, and quarantine rule. Start `weekly-synth` only after every accepted handler file parses as JSONL; it may use an empty class only when that is recorded in the quarantine file.
5. Run `weekly-synth` in the background. Accept it only after the same gate and both `raw-synthesis.md` and `outreach-queue.jsonl` exist; retry once, then write `/out/FAILURE.md` and stop rather than claim completion.
6. In the orchestrator, create `/out/handled/<class>/` and copy each accepted child file from `/out/child/<name>/handled.jsonl`. Copy accepted synthesis files and normalization/classification logs into `/out`; write `/out/SUMMARY.md` with mounts, threshold, jobs, retries, quarantines, and counts.

The human-review requirement is absolute: handlers may classify, summarize, and draft only; they neither send outreach nor change scope.
