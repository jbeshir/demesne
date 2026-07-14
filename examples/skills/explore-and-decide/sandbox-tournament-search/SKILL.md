---
name: sandbox-tournament-search
description: Compare candidate solutions in a bounded two-round tournament.
---

# Tournament search

## Constraints

Use unique DNS-1123 child names (≤40 characters). N=8 is this workflow's quality/cost cap; ensure the documented rootless-Podman prerequisite before larger fan-outs. The host must provide agent, script, wait, status, and cancel capabilities. Producing children cannot self-surface output; the parent may copy it directly or use a separate collection script child.

## Procedure

1. Generate candidates, then wait repeatedly while each job is running. Before any next stage require every input job to be `succeeded`, `exit_code=0`, and to contain its expected nonempty artifact. Retry once with a new name; otherwise stop and write `REPORT.md` with the failed stage.
2. Run judges for Round 1 and Round 2 only. `output_format` is advisory: parse each JSONL record and validate required fields, types, candidate identifiers, score range 0–100, and unique rank before pruning. Retry invalid output once; then abort and report it.
3. Deliver the ranked result and evidence to parent `/out/REPORT.md`. End after Round 2. Record deeper-search recommendations in the report and start another tournament only with explicit caller authorization.
