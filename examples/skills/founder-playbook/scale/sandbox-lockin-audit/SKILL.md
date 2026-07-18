---
name: sandbox-lockin-audit
status: alpha
description: "Assess evidence-backed customer switching costs and integration lock-in, then produce a retention plan and build backlog. Do not implement the backlog."
---

Deliver `/out/REPORT.md`, `profiles.jsonl`, `backlog.json`, `audit.md`, `customers.jsonl`, and `SUMMARY.md`.

## Control contract

Discover the customer-evidence mount by enumerating `/in` and recording the selected actual path in `/workspace/intake.md`; stop with `INPUT_AMBIGUITY.md` if selection is ambiguous. Host `directories` are absolute paths whose basenames determine mounts. Use only documented parameters; omit `model` unless the host supplies a valid concrete value. Use `egress: none` for archive staging unless a named missing dependency requires package-manager egress.

Run nested stages synchronously. Admit a stage only when it returns `succeeded`, exit code 0, and nonempty declared artifacts. Retry once; on a second failure/cancellation/missing artifact write `/workspace/quarantine/<stage>.md`, list the affected customer, and mark it unprofiled. Do not treat terminal as successful. The parent copies accepted child artifacts to its own `/out`.

## Procedure

1. Inventory files and assign an item to a customer only with an authoritative account identifier or two corroborating identifiers. Otherwise log it as unassigned with candidate IDs and reason. Write `customers.jsonl`; use one profile invocation per customer.
2. Each accepted profile produces `profile.json` and `log.md`. Its locked schema is `{customer_id,source_paths,automations:[{name,what_it_automates,built_on}],integrations:[{external_system,direction:"inbound"|"outbound"|"bidirectional",mechanism:"native"|"api"|"webhook"|"manual_export",criticality}],team_workflows:[{workflow,teams_involved,frequency,product_dependency}],switching_cost:{rebuild_effort:{rating:"low"|"medium"|"high",rationale},data_migration:{rating,rationale},retraining:{rating,rationale},workflow_disruption:{rating,rationale},overall:"low"|"medium"|"high",dominant_component},lock_in_depth:"surface"|"moderate"|"deep",evidence_gaps}`.
3. Reduce accepted profiles into `REPORT.md` and `backlog.json`; explicitly list unprofiled customers and schema failures. Backlog items are `{capability,type:"integration"|"api"|"webhook"|"sdk",deepens_customers:[...],unlocks_new_lockin,effort:"S"|"M"|"L",lock_in_gain:"low"|"medium"|"high"}`.
4. A fresh auditor reads the reducer draft and profiles, writes `audit.md` with `upheld|downgraded|unsupported` for every deep claim, then one distinct revision stage incorporates accepted downgrades. If audit or revision is quarantined, deliver the draft marked incomplete; never fabricate audit coverage.
5. Summarize profiled, unprofiled, unassigned, schema-drift, downgraded, and quarantined counts.

Mount an absolute customer-evidence directory. Require agent and deterministic-script capabilities; resolve host tool names and a concrete model, if needed, at launch. A human translates `backlog.json` before handing work to another skill.
