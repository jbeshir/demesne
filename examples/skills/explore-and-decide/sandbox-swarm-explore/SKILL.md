---
name: sandbox-swarm-explore
description: Explore a decision through independent, scored candidate proposals.
---

# Swarm exploration

1. Write `/workspace/spec.md` before dispatch. Define: distinct preambles differ in at least two named assumptions; consensus means ≥70% of valid proposals support the same conclusion; scrutinize every outlier scoring ≥20% above the median; rank candidates on evidence 40, feasibility 30, impact 20, risk 10; report coverage percentage and mean rubric score.
2. Spawn uniquely named workers with `background:true`; omit `model` for the configured default unless the host supplies a valid concrete model. Limit this workflow to eight proposals for quality/cost, not as a platform concurrency guarantee.
3. Re-wait while running. Consume only `succeeded` jobs with exit code 0 and a nonempty proposal. Retry once with a new name; then quarantine it and disclose the coverage loss.
4. Spawn the aggregator only after barriers pass. It applies only the criteria in `spec.md`, writes `REPORT.md`, and the parent copies it from `/out/child/aggregate/REPORT.md` to `/out/REPORT.md`.

Pass this procedure as the orchestrator prompt; do not restate it elsewhere.
