---
name: sandbox-debate-decision
description: Produce a bounded evidence-backed decision through structured debate.
---

# Debate decision

1. If the caller explicitly requests research and supplies a question, spawn `research-1`; it writes `/out/research.md`. Wait until `succeeded`, `exit_code=0`, and nonempty output, retry once with `research-2`, then abort on failure. The orchestrator copies `/out/child/research-1/research.md` (or retry path) to `/workspace/research.md` before Round 1.
2. Spawn no more than five debate roles. Use unique valid names; invalid or duplicate names reject the spawn call. The five-role cap is a workflow choice, not a platform concurrency claim.
3. After every wait barrier, advance only on `succeeded`, exit 0, and the contracted artifact. Retry once under a new name; then cancel dependents and write `FAILURE.md`. Completed siblings are mounted only for children spawned after completion at `/in/previous-jobs/<name>/`.
4. Run exactly two rounds and write `/out/REPORT.md` with claims, evidence, dissent, and decision. Do not add an undefined convergence branch. The parent copies child outputs into its own `/out`.
