---
name: sandbox-enterprise-procurement-pack
status: alpha
description: Draft the written pack enterprise procurement demands (product docs, support playbooks, SLA drafts, a SIG-Lite/CAIQ-Lite questionnaire answer library) plus a design-only observability/hardening plan — a slow-tier orchestrator inventories what already exists, refreshes the procurement checklist from the open web, fans out one drafter per artefact grounded in the actual codebase and ops evidence, then a fresh auditor tags every SLA number OBSERVED-or-ASPIRATIONAL, every questionnaire answer HAVE/PARTIAL/GAP, and emits a readiness scorecard. Its centrepiece is a compliance-drift reconciliation design (claimed state vs live telemetry), because artefacts are cheap to produce and expensive to keep synchronized. Apply when the user wants to "get procurement-ready", "draft our SLAs", "prep for enterprise security questionnaires", "build the SOC 2 / vendor-review document pack", "write our support playbook and SLA", or "what will enterprise procurement ask us for". HYBRID: the ongoing support-ops layer (ticket routing, renewals, reporting cadence, running the drift reconciliation) is host-side finishing. Skip when the ask is per-named-account gaps (use sandbox-enterprise-gap-analysis), code-level SOC2/GDPR/HIPAA remediation (use sandbox-compliance-review), pre-launch code security audit (use sandbox-prelaunch-security-review), or actually building the observability code (hand the plan to sandbox-feature-work).
allowed-tools: Read, Glob, Grep, Bash, Write, Edit, mcp__demesne__sandbox_agent, mcp__demesne__sandbox_research, mcp__demesne__sandbox_script, mcp__demesne__sandbox_wait, mcp__demesne__sandbox_cancel
---

Draft the procurement document pack and the observability/hardening plan an enterprise buyer's review will demand. You (the host session) author one orchestrator prompt, launch a single slow-tier `sandbox_agent`, and it inventories existing artefacts, refreshes the requirement checklist from the open web, fans out one drafter per artefact, then a fresh auditor grounds every claim and scores readiness. The deliverable is markdown + a plan in `/out`; **there is no code landing** — this skill designs the observability layer, it does not build it (hand `plans/observability-hardening-plan.md` to `sandbox-feature-work`).

**Watch out (cross-cutting):** An SLA number with no observed backing is a breach-of-contract exposure the moment it's signed — every uptime/response/resolution figure must trace to inventory/evidence (`[OBSERVED]`) or be marked `[ASPIRATIONAL]`, never asserted bare. A questionnaire answer marked HAVE without an artefact behind it fails the instant procurement asks for the evidence — the auditor downgrades unbacked HAVEs to GAP.

## Procedure

1. **INTAKE** (orchestrator's own process) — Write `/workspace/context.md`: what the product is, target buyer size (mid-market $20K–$100K ACV vs enterprise $5M+ — they weight differently: mid-market buys to fill security gaps, enterprise to satisfy compliance/incident-response), current compliance posture, and what monitoring/incident tooling exists today. Read the mounted repo at `/in/<repo>` and the ops-evidence dir at `/in/<evidence>` (see Launching). Children must handle messy real-world files (PDFs, screenshots, half-written docs) and **log unparseable inputs to `/workspace/skipped.md` rather than silently dropping them**.

2. **INVENTORY** (deterministic → one `sandbox_script`, `name=inventory`, `image=<repo lang>`, `egress=package-managers`) — Grep the repo + evidence dir for artefacts that already exist (DPA, subprocessor list, existing SLA text, monitoring/alerting config, incident runbooks, audit-log code) and run the ecosystem's dependency-vulnerability audit (`npm audit`, `pip-audit`, `govulncheck`) — a checklist item that is deterministic, never an LLM's guess. Write `/workspace/inventory.json`: `{artefact, present: true|false, path, note}` list + the dep-audit summary. This is the ground truth drafters use to tag OBSERVED vs ASPIRATIONAL.

3. **GROUND** (one medium-tier `sandbox_research`, `name=ground`) — Refresh the requirement checklist from the open web. `sandbox_research` has open egress but a **fresh private `/workspace` and no `/in` mounts** — embed the baseline checklist below in its prompt and task it to date/correct each item (thresholds and program versions drift: SSPA v11→v12 flipped ISO 27001 from "may be" to "will be required" for SaaS). Its output is harvested from `/out/child/ground/checklist.md`; drafters read it at `/in/previous-jobs/ground/` once it completes.

4. **PLAN** (orchestrator) — Write `/workspace/plan.md`: the five drafting children and the handoff contract (each reads `/workspace/context.md`, `/workspace/inventory.json`, `/in/previous-jobs/ground/checklist.md`, the mounts; each writes one file to its own `/out`).

5. **DRAFT** (fan-out; five medium-tier `sandbox_agent` children) — Dispatch all five with `background: true`, collect `job_id`s, poll `sandbox_wait` (`timeout_seconds: 120`) until every job is terminal — blocking calls are issued one per turn and run sequentially, so background dispatch is the only way they run concurrently. Five is ≤8 so dispatch all; keep ≤8 in flight. Names (DNS-1123: lowercase, digits, interior hyphens, ≤40 chars): `draft-product-docs`, `draft-support-playbook`, `draft-sla`, `draft-questionnaire`, `plan-observability`. Each writes to `/out/child/<name>/`. Content specs are in [Writing the orchestrator prompt](#writing-the-orchestrator-prompt).

6. **AUDIT + SCORECARD** (one slow-tier `sandbox_agent`, `name=audit-scorecard`; fresh context, never a drafter scoring its own work) — **Barrier: spawn only after all five drafters are terminal**, so their files exist at `/in/previous-jobs/<name>/`. It reads every draft, the checklist, and `inventory.json`, and: (a) verifies each `[OBSERVED]` SLA number actually traces to inventory/evidence — downgrades unbacked ones to `[ASPIRATIONAL]` and flags the known weak spots (credit caps of 15–25% of monthly fees, 5–15-day manual claim windows, response-vs-resolution tracked separately); (b) downgrades HAVE questionnaire answers with no artefact to GAP; (c) checks every `[ASPIRATIONAL]` SLA has a matching enforcement item in the observability plan — an aspirational SLA with no path to prove it is flagged; (d) emits `readiness-scorecard.md` (each checklist artefact → HAVE / PARTIAL / MISSING / ASPIRATIONAL, with the evidence pointer or the gap). Ends with `PASS` or `CHANGES_NEEDED`. On `CHANGES_NEEDED` the orchestrator re-dispatches only the affected drafter and re-audits — **cap 2 fix rounds**, then ship with residual flags listed in the scorecard.

7. **DELIVER** (orchestrator's own process) — `cp` each child's output into `/out` yourself: drafts → `/out/pack/`, the plan files → `/out/plans/`, scorecard + audit → `/out/`, checklist → `/out/`. A child's `/out` is `/out/child/<name>/`; delegating this copy to a `sandbox_script` child strands the files there. Write `/out/README.md` (pack index + top scorecard gaps) and `/out/metadata.json` (buyer size, artefacts drafted, residual flags, run date). Print `DONE`.

## Writing the orchestrator prompt

Brief it as a complete document. Embed this **baseline requirement checklist** (the `ground` child refreshes it; drafters cover it):

- **Security questionnaire**: SIG-Lite (~128 Qs) / CAIQ-Lite (71 Qs) pre-filled answer library, refreshed annually.
- **SOC 2 Type II** is the enterprise gate (Type I is design-only; bridge = "share Type I now, commit to Type II within 12 months"). **ISO 27001:2022** — EU price-of-admission, deflects 70–90% of questionnaires. **Pen test** — annual 3rd-party, ≥40 hrs, findings + remediation evidence.
- **Insurance**: Cyber / Tech E&O, $1M–$5M limits ($5M+ for F500), customer named Additional Insured.
- **DPA + subprocessor list**: Article 28 general-authorization model, public page with a "Last Updated" date and a transfer-mechanism column (SCCs / UK IDTA); AI-inference providers now heavily questioned. **Data-residency** statement.
- **Uptime SLA + credits**: 99.9% baseline (64% of $5M+ contracts; 99.95% next tier); credits cap 15–25% (or 50–100%) of monthly fees, manual claim within 5–15 days. **Support SLA** P1–P4 by **business impact, not urgency** — P1 ack ~15 min / resolve 1–2 hr, P4 next business day; **response and resolution are separate tracked commitments**; P1/P2 24×7, P3/P4 8×5; auto-escalate on breach (P2 idle 8 hr → P1) and one level up for enterprise tickets.
- **Breach-notification clause**: 24–48 hr contractual (67% of enterprise DPAs) — distinct from GDPR's 72 hr regulator clock and HIPAA's 60 days; trigger on **"suspicion," not "confirmation."** **BC/DR** plan with annual test cadence, stated RTO/RPO. **VPAT / ACR** to WCAG 2.1 AA for public-sector-adjacent deals.

Per-drafter briefs:

1. **`draft-product-docs`** → `product-documentation.md`: architecture overview, data-flow + subprocessor map, security overview, admin/onboarding guide. Diátaxis-aware (reference vs how-to). Ground every capability claim in the repo; don't overstate what the code does.
2. **`draft-support-playbook`** → `support-playbook.md`: P1–P4 tier definitions on an impact/urgency matrix, coverage windows, escalation-on-breach paths, and the response-vs-resolution distinction — the enforceability hinge (no timestamped state transitions ⇒ no provable breach ⇒ no defensible credit claim).
3. **`draft-sla`** → `sla-drafts.md`: uptime SLA + credit schedule, support-SLA response/resolution table, breach-notification clause, BC/DR summary. **Every number tagged `[OBSERVED]` (traces to `inventory.json`/evidence) or `[ASPIRATIONAL]` (a target needing observability to enforce).** Do not invent figures the ops evidence can't support.
4. **`draft-questionnaire`** → `questionnaire-prefill.md`: SIG-Lite/CAIQ-Lite answer library across the checklist domains, DPA + subprocessor list (public-page format), data-residency + insurance + SOC 2 / ISO / pen-test status. Each answer tagged **HAVE / PARTIAL / GAP** with an evidence pointer; a HAVE with no artefact is not allowed to stand.
5. **`plan-observability`** → `observability-hardening-plan.md` **and** `compliance-drift-reconciliation.md`: the design (not code) for the telemetry that makes SLAs enforceable — immutable, complete-and-unmodified audit logging (SOC 2 CC7.2; 6-yr retention for HIPAA; access-review evidence), uptime measurement backing every credit claim, incident-response tooling, ticketing with timestamped response/resolution state transitions. **Centrepiece — the drift reconciliation**: a recurring diff of *claimed state* (subprocessor list, SLA uptime figures, access-review dates, DR test dates, program versions) against *live telemetry* (IAM logs, ticketing SLA timestamps, monitoring uptime), flagging drift before a renewal or a SOC 2 Type II observation window catches it. Format both as an implementation backlog `sandbox-feature-work` can pick up.

Also brief: the auditor's four checks (above), the OBSERVED/ASPIRATIONAL and HAVE/PARTIAL/GAP tagging discipline, the 2-round fix cap, and the `skipped.md` logging rule.

## Output contract

```
/out/
  README.md                              # pack index + top scorecard gaps
  readiness-scorecard.md                 # each checklist artefact → HAVE/PARTIAL/MISSING/ASPIRATIONAL + evidence/gap
  AUDIT.md                               # PASS/CHANGES_NEEDED, downgraded claims, unenforceable-SLA flags
  checklist-refreshed.md                 # from the ground research child
  pack/
    product-documentation.md
    support-playbook.md
    sla-drafts.md                        # every number [OBSERVED] or [ASPIRATIONAL]
    questionnaire-prefill.md             # SIG-Lite/CAIQ-Lite + DPA/subprocessor + data-residency
  plans/
    observability-hardening-plan.md      # design-only → hand to sandbox-feature-work
    compliance-drift-reconciliation.md   # the claimed-vs-telemetry recurring diff design
  metadata.json
```

`README.md` order: **Readiness summary** (scorecard headline: how many HAVE / PARTIAL / MISSING), **What's ship-ready**, **Top gaps to close before signing** (ordered), **Aspirational SLAs and their enforcement path**, **Pack index**.

## Launching the orchestrator

- **`directories: ["<abs repo path>", "<abs evidence path>"]`.** The repo mount grounds SLA numbers and the observability plan in real capability; the evidence dir carries founder-private current state (existing docs, monitoring config, incident history, insurance certs, current uptime data, subprocessor contracts). **If the repo mount is forgotten, every SLA number is forced to `[ASPIRATIONAL]` and the observability plan is generic — say so and re-launch.** If the evidence dir is empty, the scorecard is mostly MISSING and that is an honest, useful result.
- Tier: **slow** for the orchestrator and the auditor; **medium** for the five drafters and the `ground` researcher; `sandbox_script` for INVENTORY.
- Tell the orchestrator to dispatch the five drafters with `background: true` and poll `sandbox_wait` (≤8 in flight), and to spawn the auditor only after all five are terminal — a blocking or premature auditor sees empty sibling mounts.

## Host-side finishing

The pipeline drafts the pack and designs the plan; the **ongoing support-ops layer cannot run inside a sandbox** (no access to your ticketing, mail, calendar, or scheduler sessions) — it is host work you drive with your own connected tools:

1. **Hand the observability plan to `sandbox-feature-work`** to actually build the logging/monitoring/ticketing instrumentation — this skill only designed it.
2. **Stand up the ongoing ops layer** — ticket routing + escalation workflows, renewal tracking, and the enterprise-CS reporting cadence — via whatever is connected to your session (e.g. a connected ticketing/CRM/scheduler MCP tool; confirm the actual tool names against your session; if nothing is connected, hand `pack/support-playbook.md` and the scorecard to the founder to wire manually).
3. **Schedule the drift reconciliation** from `plans/compliance-drift-reconciliation.md` to run on a recurring cadence (e.g. a host scheduler; else a documented manual monthly/pre-renewal check) — this is the durable value: claimed compliance state silently rots against live telemetry, and catching the drift before a named-account renewal or a Type II observation window is the whole point.
4. **Per-named-account** demands are out of scope here — route them to `sandbox-enterprise-gap-analysis`, mounting this pack as its current-state input.
