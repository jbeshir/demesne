---
name: sandbox-quality-improvement
description: Make behavior-preserving quality repairs in an existing repository.
---

# Quality improvement

1. Require a mounted repository, enumerate `/in` excluding `previous-jobs`, require one candidate, and stage it at `/workspace/repo`. Do not assume a basename. Omit `model` unless the host supplies a valid concrete model.
2. Run the baseline gate in a language-image script. Use `egress=package-managers` when the pinned gate must install dependencies; otherwise use `none`. Record the command and result in `BASELINE.md`.
3. If the gate fails, fix only a provably behavior-preserving cause. For any failure requiring behavior, feature, route, output, or wiring change, write `BACKLOG.md`, keep the gate red, and halt review.
4. Run auditors sequentially so this workflow does not depend on nested asynchronous control availability. Each auditor writes one nonempty report. After success and `exit_code=0`, the orchestrator copies `/out/child/<name>/<file>` into its own `/out`; a later auditor reads the completed sibling at `/in/previous-jobs/<name>/` or the collected copy. Retry a failed auditor once with a new name; then record `FAILURE.md` and stop dependent work.
5. Classify incomplete or unwired behavior as non-blocking backlog unless the repair is provably behavior-preserving. Never implement missing product functionality in this workflow. Run one independent re-review after each accepted repair, then rerun the gate.

## Outputs

Write `BASELINE.md`, `AUDIT.md`, `REVIEW.md`, `BACKLOG.md` or `FAILURE.md`, and `/out/repo`. The parent, not a producing child, copies the final repository and reports to `/out`.
