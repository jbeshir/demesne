---
name: sandbox-solution-concept-pressure-test
status: alpha
description: Pressure-test a proposed solution concept against supplied discovery evidence and alternatives. Use after discovery; it does not perform a market-landscape study.
---

Use native Demesne sandbox capabilities. Omit `model` unless the host supplies a concrete schema-valid value. Every named child is a unique DNS-1123 label. A completed child is available to a sibling only when that sibling is spawned later, under `/in/previous-jobs/<name>/`.

## Procedure

1. Enumerate `/in` excluding `/in/previous-jobs`. Identify exactly one discovery-evidence mount by declared contents and copy it to `/workspace/discovery.md`. If zero or multiple match, write `/out/FAILURE.md` and stop.
2. If a supplied alternatives/landscape report is found, copy and normalize it to `/workspace/alternatives.md` and plan to deliver it as `/out/alternatives.md`. If none is supplied, run bounded alternatives research only: direct alternatives, workaround alternatives, and adjacent substitutes. This is not a full market-landscape claim.
3. Dispatch background attackers for every required evidence/alternative stream. Each writes only its own `/out/attack.md`. Poll every job until terminal; accept only `status == succeeded`, `exit_code == 0`, and an existing declared file. Retry once with a unique `-r2` name. On a second failure, write job ID, status, exit code, and preserved stderr location to `/out/FAILURE.md`, then stop.
4. After accepted attackers complete, dispatch `concept-reducer`; it reads their completed-sibling files and writes `/out/verdict.md`, `/out/concept.md`, `/out/assumptions.md`, and `/out/attacks/`. It selects exactly three load-bearing assumptions, each with `what_must_be_true`, `cheap_test`, and `blast_radius`. Apply the same success/file gate and retry policy.
5. The parent copies accepted reducer outputs and `alternatives.md` when present into its own top-level `/out`, verifies them, and writes `/out/SUMMARY.md`. A parent or a dedicated collector child may copy files, but all deliverables must land in the parent run's top-level `/out`.

## Output contract

Deliver `/out/verdict.md`, `/out/concept.md`, `/out/assumptions.md`, `/out/attacks/`, `/out/metadata.json`, `/out/SUMMARY.md`, and `/out/alternatives.md` when supplied. Verdict is `PROCEED-TO-PROTOTYPE|REVISE-CONCEPT|RETURN-TO-DISCOVERY`; every attacker finding includes statement, severity, evidence citation or `SPECULATION`, and exposed assumption. Do not implement product code.
