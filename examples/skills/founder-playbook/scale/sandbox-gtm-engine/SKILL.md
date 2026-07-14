---
name: sandbox-gtm-engine
status: alpha
description: "Create an evidence-backed, coherent GTM engine for selected audiences and modules. Do not send outreach, configure external systems, or implement infrastructure."
---

Deliver `/out/GTM-ENGINE.md`, `segments.md`, `audiences/`, `ops/`, `infra-backlog.md`, `coherence-report.md`, `research/`, and `metadata.json`.

## Intake and control contract

Ask the requester to select one or more audiences (`users`, `investors`, `enterprise`, `analysts`) and modules (`messaging`, `playbooks`, `ops`, `infrastructure`, `host-activation`). Default to none; if no selection is supplied, write `INPUT_REQUIRED.md` and stop. Enumerate `/in` at runtime and record each actual mount in `/workspace/intake.md`; classify evidence, repository/docs, and optional prior reports from their manifests, never from a fixed basename.

Use documented tool signatures and omit `model` unless the host supplies a valid concrete value. Nested stages are synchronous. Before a downstream dispatch, require upstream `succeeded`, exit code 0, and nonempty expected artifacts. Retry once; then write a quarantine record with status, error, and missing outputs and exclude the unit. Parent-copy the latest accepted artifact into its declared `/out` path. A revision always uses a distinct invocation and replaces only the affected audience artifact; no third round.

## Procedure

1. Write `segments.md` with selected audiences, evaluation criteria, evidence inventory, and missing-evidence list. A proof point is evidence-linked or `ASPIRATIONAL`.
2. For each selected messaging audience, generate candidates from pain-led, category, and proof-led frames. Score 1–10 for eye-roll resistance, vocabulary match, proof verifiability, differentiation, and core-narrative consistency. Keep the highest total; break ties by evidence-linked proof count, then mark remaining tie unresolved.
3. Produce selected playbooks and ops modules from the winning message. Every deliverable states owner, review cadence, acceptance measure, and evidence gaps. Infrastructure items are `{item,current_state:"exists"|"partial"|"missing"|"unassessed",owner,acceptance_test,dependency,priority}`.
4. A fresh coherence reviewer checks each selected output for contradictory core claims, unsupported proof, missing owner/measure, and audience vocabulary mismatch. Write `coherence-report.md`. Revise failed units once and recheck only those units; open items stay in the report.
5. Assemble `GTM-ENGINE.md` with core narrative, audience map, per-audience summaries, operating cadence, infrastructure backlog summary, and open items. `metadata.json` lists selected scope, accepted/quarantined units, and unresolved items.

Launch with absolute evidence and optional repository/prior-report paths. Require agent and deterministic-script capabilities; resolve host tool names at launch. Host activation is a handoff only: do not send messages or mutate CRM/calendar systems.
