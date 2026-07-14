---
name: sandbox-bottleneck-stress-test
status: alpha
description: "Find founder-dependent workflows that fail during a one-week absence and deliver verified handoff designs. Do not use for workload inventory or code changes."
---

Produce `/out/bottleneck-stress-test.md`, `bottleneck-map.jsonl`, `SUMMARY.md`, `stress/`, `fixes/`, and `verify.md`. Do not edit product code.

## Execution rules

Discover mounts at runtime: enumerate `/in`, record each basename and purpose in `/workspace/intake.md`, and stop with `INPUT_AMBIGUITY.md` if evidence or a load audit cannot be identified unambiguously. Host `directories` are absolute source paths; their basenames become `/in/<basename>`.

Use only documented tool parameters. Omit `model` unless the host supplies a valid concrete value. Run nested children synchronously; use no unsupported nested job-control calls. A child is accepted only when its returned result is `succeeded`, `exit_code` is 0, and every declared nonempty output exists. Retry one failed or cancelled child once with a new invocation; then write `/workspace/quarantine/<stage>.md` with the invocation, status, error, and missing outputs. Never reduce quarantined output. The parent copies every accepted child artifact from its returned output directory into the declared parent `/out` path.

Use `sandbox_script` with `image: python` and `egress: none` for structured CSV, JSON, or ICS parsing; let the orchestrator inspect small unstructured notes directly. No child may alter the source mounts.

## Procedure

1. Identify founder evidence and an optional load-audit mount. Write `bottleneck-map.jsonl`, one record per item: `{workflow_id,name,type:"workflow"|"decision"|"approval",cadence,why_founder:"habit"|"authority"|"expertise"|"relationship",depends_on:[...],downstream:[...],load_audit_bucket}`. Log unparseable files. Group dependency-connected items into `/workspace/clusters/<id>.jsonl`.
2. For each cluster, obtain a fresh adversarial simulation. Require `stress.jsonl` records `{workflow_id,on_absence:"stalls"|"breaks"|"silently_degrades"|"unaffected",cascade_from:[...],time_to_impact,severity:1-5,who_would_notice,derail_risk,evidence}` and `week-narrative.md`. Set `derail_risk=true` only for a severity 4–5 stall/break or a silent degradation with no named detector. Copy them to `/out/stress/<cluster>/`.
3. After all accepted simulations, reduce only their copied artifacts into a ranked `derail-risk.jsonl`. If none are accepted, deliver an incomplete report naming every quarantine and stop.
4. For each ranked workflow, obtain `fix.md` specifying handoff trigger and owner, decision rules, escalation owner and threshold, exceptions, and `tighten-existing` or `build-new` destination. Copy to `/out/fixes/<workflow>/fix.md`.
5. After all accepted fixes, use a fresh verifier to emit `verify.md` with `PASS` or `RESIDUAL` per fix. Re-run only residual fixes once, then a fresh verifier. Report remaining residuals; do not create a third round.
6. Assemble the report with TL;DR, map, absence simulation, ranking, fixes, residual risks, and implementation order. Include counts for mapped workflows, accepted/quarantined simulations, fixes, and residuals.

## Launch

Mount founder evidence and, when available, a load-audit output using absolute host paths. Require capabilities for agent work, deterministic scripts, and parent artifact collection; map those capabilities to the active host rather than embedding provider-specific tool identifiers.
