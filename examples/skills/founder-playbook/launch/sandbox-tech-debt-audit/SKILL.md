---
name: sandbox-tech-debt-audit
status: alpha
description: "Measure technical debt in a mounted repository and produce a report-only remediation plan and architecture-decision draft. Do not change code."
---

# Technical debt audit

Give one orchestrator this complete procedure plus stack, test command, coverage image, and repository basename. Omit `model` unless the host provides a valid concrete model.

## Procedure

1. Enumerate `/in`, excluding `previous-jobs`; resolve exactly one directory matching the supplied repository basename. On zero or ambiguity write `/out/INCOMPLETE.md` and stop. Copy it with `cp -a "$MOUNT"/. /workspace/repo`.
2. Discover `AGENTS.md` and `CLAUDE.md` at repository/project roots. Read every present file and record the canonical target document, or use neutral `ARCHITECTURE-DECISIONS.md` when none exists.
3. Detect toolchain. For Go, Node, Python, or Anaconda run the host-supplied locked coverage command in the matching image and write `/workspace/coverage.txt`. For any other toolchain, write `UNMEASURED: unsupported toolchain; <reason>`; never estimate coverage.
4. Dispatch `audit-structural`, `audit-coverage`, `audit-refactor`, and `audit-decisions` in background, maximum eight. Each writes its `AUDIT-<dim>.md` with `file:line`, rationale, and S/M/L effort. Audit decisions skips already documented decisions.
5. At every child barrier repeat `sandbox_wait` while `running`. Require `succeeded`, `exit_code == 0`, and a nonempty declared artifact at `/out/child/<name>/...`. Preserve failure diagnostics, retry once under a fresh `-r2` name, then cancel dependents and write `INCOMPLETE.md`. Do not sequence incomplete audits.
6. Harvest the four audits and coverage to the orchestrator `/out`. Run `sequence-plan-r1`, then fresh `verify-plan-r1`. On `CHANGES_NEEDED`, run `sequence-plan-r2` against the critique and `verify-plan-r2`; two rounds maximum. Harvest the selected plan and final review. Run `backfill-decisions` and harvest `ARCHITECTURE-DECISIONS.md`.
7. Write `EXECUTIVE_SUMMARY.md` and verify all required files are nonempty before `DONE`.

## Output contract

```text
/out/EXECUTIVE_SUMMARY.md
/out/REMEDIATION-PLAN.md
/out/PLAN-REVIEW.md
/out/ARCHITECTURE-DECISIONS.md
/out/coverage.txt
/out/AUDIT-structural.md
/out/AUDIT-coverage.md
/out/AUDIT-refactor.md
/out/AUDIT-decisions.md
```

The plan assigns every finding to `must-fix-before-next-release`, `can-wait-a-sprint`, or `acceptable-ongoing-debt`, with evidence, effort, risk, sprint, and execution route. Architecture decisions are a draft for human review, never a landed change.

## Launch inputs

Pass `directories: ["<absolute repository path>"]`. State its basename, stack, locked test/coverage command, image, and product context. This audit is report-only; route approved work to the appropriate implementation workflow.
