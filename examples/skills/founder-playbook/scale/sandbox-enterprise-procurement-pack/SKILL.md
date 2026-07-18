---
name: sandbox-enterprise-procurement-pack
status: alpha
description: "Prepare evidence-tagged enterprise procurement materials and an observability hardening plan. Do not make account-specific promises or implement the plan."
---

Deliver `README.md`, `readiness-scorecard.md`, `AUDIT.md`, `checklist-refreshed.md`, `pack/`, `plans/`, and `metadata.json` in `/out`.

## Control contract

At runtime enumerate `/in`; identify one repository/current-state mount and one operations-evidence mount by manifest inspection. Record actual paths in `/workspace/intake.md`; stop with `INPUT_AMBIGUITY.md` if either role is ambiguous. Launch mounts from absolute host paths; basenames determine `/in/<basename>`.

Use documented tool signatures. Omit `model` unless the host supplies a valid concrete supported model. Use deterministic scripts with a valid image selected from the repository toolchain and `egress: none`; allow package-manager egress only for a named missing dependency. Run nested stages synchronously. Require `succeeded`, exit code 0, and nonempty declared artifacts; retry once, then quarantine the stage with status/error/missing files and mark dependent scorecard rows `UNASSESSED`. Parent-copy accepted child artifacts into its own `/out`.

## Procedure

1. Inventory existing DPA, subprocessors, SLA, monitoring, incident, audit-log, and dependency-audit evidence. Write `inventory.json` and `skipped.md`; all quantitative claims must be traceable.
2. Refresh a requirements checklist only when current external requirements matter. Research workers receive the baseline in their prompt because private mounts are unavailable to research sandboxes. Validate the returned checklist before use.
3. Draft product documentation, support playbook, SLA, questionnaire prefill, and both observability plans. Every SLA number is `[OBSERVED]` with an evidence path or `[ASPIRATIONAL]`; every questionnaire answer is `HAVE`, `PARTIAL`, `GAP`, or `UNASSESSED` with evidence.
4. A fresh auditor downgrades unsupported claims, verifies each aspirational SLA has an enforcement item, and emits the scorecard. Redraft affected files once only; residual gaps remain explicit.
5. Deliver a pack index, scorecard counts, residuals, and metadata. `observability-hardening-plan.md` and `compliance-drift-reconciliation.md` are designs for later implementation, not code.

Mount absolute repository/current-state and operations-evidence paths. Require agent, optional research, and deterministic-script capabilities; map them to the active host at launch.
