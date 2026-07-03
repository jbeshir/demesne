---
name: sandbox-feedback-loop-ops
description: Run the recurring weekly feedback loop over a mounted inbox of raw user feedback (bug reports, feature requests, session notes, support emails, churn messages as loose files in any format). An orchestrator ingests the inbox, classifies every item into a closed taxonomy (bug / feature-request / confusion / churn-signal / praise / escalate-human), fans out one handler child per class — bugs get a repro sketch and severity, feature requests are scored against the mounted MVP scope doc — then a reducer produces a FACTUAL weekly raw synthesis and STOPS. A human must review that raw roll-up before any cross-cycle pattern analysis runs; pattern analysis is a separate, host-gated mode. The pipeline also emits structured outreach/scheduling drafts for a host-side finishing step. Apply when a founder wants to process a batch of accumulated user feedback on a cadence — "triage this week's feedback", "run the weekly feedback synthesis", "process the bug and feature inbox", "what came in from users this week", "roll up the support notes". Skip when: designing the triage tree or sprint cadence itself rather than running it (sandbox-product-ops-system); cold-prospect discovery or interview recruiting (sandbox-outreach-pipeline); pressure-testing one proposed feature against scope (sandbox-mvp-scope-guardrail); diagnosing PMF from usage/survey data (sandbox-pmf-diagnostic); a generic same-op-over-corpus pass with no triage (sandbox-corpus-map-reduce).
allowed-tools: Read, Glob, Grep, Bash, Write, Edit, mcp__demesne__sandbox_agent, mcp__demesne__sandbox_script
---

Run the ongoing discovery/feedback operational loop as a recurring weekly pipeline. The host writes one orchestrator prompt and launches a single slow-tier `sandbox_agent`; that orchestrator ingests a mounted inbox of raw feedback, classifies each item into a closed taxonomy, fans out per-class handler children, and a reducer rolls the handled records into a **factual** weekly synthesis. The pipeline has two modes and a hard gate between them: **INTAKE mode** ends at `/out/raw-synthesis.md` and stops; **PATTERN mode** (cross-cycle trend analysis) is a *separate* run the host launches only after a human has reviewed and annotated that raw synthesis — this gate is the playbook's rule (a human reviews the raw synthesis before any automated pattern-analysis on top of it, L262–263). No code lands. Outreach drafts, scheduling requests, and contact-list updates are emitted as structured output for the `## Host-side finishing` step; the sandbox never sends anything.

**Watch out (cross-cutting):** (1) INTAKE mode must never infer cross-item themes, trends, or priorities — that collapses the human-review gate; the reducer produces counts, tables, and verbatim quotes only. (2) Copy handler and synthesis output into the orchestrator's own `/out` with `cp` in the orchestrator's process — a child writes only to `/out/child/<name>/` and delegating the copy to a `sandbox_script` child strands the files. (3) Never fabricate a recipient email/handle and never auto-send: outreach is drafted in-sandbox and sent host-side, founder-reviewed.

## Procedure

**Modes.** A weekly run is INTAKE mode (steps 1–6). PATTERN mode (step 7) is a distinct later invocation, gated on the human review; do not run it in the same pass.

1. **INTAKE.** The orchestrator reads every file under `/in/<inbox>/` (emails, exported tickets, Slack/session notes, plaintext dumps — messy real-world formats expected) and writes `/workspace/queue.jsonl`, one object per line: `{id, path, preview, raw_type}`, where `path` is the absolute `/in/<inbox>/…` path and `preview` is the first ~500 chars. Anything unreadable or empty is logged with `raw_type:"unreadable"` so it routes to `escalate-human` rather than vanishing.

2. **TAXONOMY.** Write `/workspace/classes.md` as the closed set below before classifying — the taxonomy is fixed by the playbook, but the file carries each class's handler spec, record schema, and confidence threshold: **bug**, **feature-request**, **confusion** (user misread/friction with something that already exists), **churn-signal** (dissatisfaction or intent-to-leave), **praise**, and mandatory **escalate-human** (mixed/ambiguous/policy-sensitive/unparseable — no handler, straight to quarantine). Items with mixed content route to their dominant signal; genuinely split items go to `escalate-human`.

3. **CLASSIFY.** Spawn `name=classify01`, medium tier. It reads `/workspace/queue.jsonl` and `/workspace/classes.md` and writes `/workspace/routes.jsonl`: `{id, class, confidence, reason}` per line — `class` a slug from the taxonomy, `confidence` a 0–1 float, `reason` 1–2 auditable sentences. DNS-1123 child names throughout (lowercase letters, digits, interior hyphens, ≤40 chars). If the inbox exceeds ~150 items, shard into `queue-shard-NN.jsonl`, dispatch classifier children `background: true`, poll `sandbox_wait` (≤8 in flight), and `cat` the `routes-shard-NN.jsonl` back together; each shard's prompt embeds the full taxonomy.

4. **HANDLE.** The orchestrator reads `/workspace/routes.jsonl` with `jq`, drops sub-threshold items and every `escalate-human` item to `/workspace/quarantine.jsonl` (never handled), and writes per-class slices to `/workspace/slices/<class>.jsonl`. Then it spawns **one handler child per populated non-escalate class** (`name=handle-bug`, `handle-feature-request`, `handle-confusion`, `handle-churn-signal`, `handle-praise`), medium tier, each dispatched `background: true` and polled with `sandbox_wait`, **≤8 in flight** — blocking calls are issued one per turn and run sequentially, so background dispatch is what runs them concurrently. Each handler reads its slice and the actual item files via the inherited `/in/<inbox>` mount and writes `/out/handled.jsonl` in its class schema (below) plus `/out/log.md` for anything it could not parse. The **feature-request** handler additionally reads the mounted scope doc at `/in/scope-doc/` if present and scores each request against it; if no scope doc is mounted it scores on the generic rubric and stamps `scope_verdict:"no-scope-doc"`.

5. **RAW SYNTHESIS (the gate).** After **all** handler jobs reach a terminal state (barrier: `sandbox_wait` on every handler `job_id` first — files appear at `/in/previous-jobs/handle-<class>/` only once that sibling completes), spawn `name=weekly-synth`, slow tier. It reads each handler's `/in/previous-jobs/handle-<class>/handled.jsonl` and produces `/out/raw-synthesis.md` — a **factual roll-up only**: volume by class, bug table by severity, feature-request list with scope verdicts, verbatim churn and praise quotes with source ids, quarantine count. It must NOT rank, theme, trend, or prioritise. Header stamp, verbatim: `RAW SYNTHESIS — HUMAN REVIEW REQUIRED before pattern analysis (PATTERN mode)`. The same child also writes `/out/outreach-queue.jsonl` (draft messages, schema below) from the churn-signal and praise records.

6. **DELIVER.** In the orchestrator's own process, `cp` `weekly-synth`'s outputs into `/out`, then write `/out/triage.jsonl` (every item's `{id, class, confidence, reason, status}` with `status` ∈ `handled|quarantined|escalated`), `/out/quarantine.jsonl`, and `/out/SUMMARY.md` (counts, handler errors, whether a scope doc was mounted). If >20% of items land in `escalate-human`, say so in `SUMMARY.md` — the inbox is noisy or the intake step under-parsed. Print `DONE — awaiting human review of raw-synthesis.md before PATTERN mode`.

7. **PATTERN mode (separate gated run).** Launch only after the founder has reviewed and annotated `raw-synthesis.md`; the host mounts that reviewed file at `/in/reviewed-synthesis/` and prior weeks' outputs at `/in/history/`. Spawn `name=pattern-analysis`, slow tier, which reads the reviewed synthesis plus history and writes `/out/pattern-analysis.md`: recurring themes, iteration-cycle deltas (persisting / worsening / resolved vs prior weeks), the top 3 priorities with cited item ids, and feed-forward hooks (feature-request clusters that amount to a scope-amendment case → route to `sandbox-mvp-scope-guardrail`; churn clusters). It cites `id`s; it never re-triages raw items.

## Writing the orchestrator prompt

Brief it as a complete document:

1. **The inbox** — path (`/in/<inbox>`), what the founder dropped in, formats to expect, the week/cycle label, and that children must handle messy files and log unparseable ones to `escalate-human`, never silently skip.
2. **The closed taxonomy, verbatim** — the six classes of step 2, dominant-signal routing, and the mandatory `escalate-human`. No invented classes.
3. **Per-class handler ops and schemas** — `handle-bug`: `{id, source_path, summary, repro_steps, expected, actual, severity, affected_area}`, `severity ∈ blocker|major|minor|cosmetic`. `handle-feature-request`: `{id, source_path, request, underlying_need, scope_verdict, amendment_check, evidence_strength, requester}`, `scope_verdict ∈ in-scope|out-of-scope|amendment-candidate|no-scope-doc`; `amendment_check` applies the scope doc's feature-amendment criteria — **genuine user signal vs founder enthusiasm dressed as product thinking**; `evidence_strength ∈ recurring|weak|single-anecdote`. `handle-confusion`: `{id, source_path, what_confused, feature_exists, probable_gap}`, `probable_gap ∈ docs|onboarding|ux|discoverability`. `handle-churn-signal`: `{id, source_path, user, signal_quote, urgency, suggested_action}`, `urgency ∈ urgent|watch`. `handle-praise`: `{id, source_path, user, quote, testimonial_candidate, consent_needed}`.
4. **Confidence thresholds** — a global floor (e.g. 0.6) or per-class values; sub-threshold items quarantine, never dispatch.
5. **The gate, stated twice** — INTAKE mode ends at the factual raw synthesis; the reducer must not theme/trend/prioritise; PATTERN mode is a separate run the host triggers only after human review. This is a playbook hard rule, not a stylistic choice.
6. **`raw-synthesis.md` section order** — Header + gate stamp; Volume by class; Bugs by severity (blockers first); Feature requests (with scope verdicts); Churn signals (verbatim quote + urgency); Praise / testimonials; Quarantine & escalate-human (count + reasons); Outreach queue (pointer to `outreach-queue.jsonl`).
7. **`outreach-queue.jsonl` schema** — `{id, trigger_class, recipient, channel_hint, draft_subject, draft_body, suggested_action, needs_founder_review:true}`, `suggested_action ∈ email-reply|schedule-session|request-testimonial|log-contact`. `recipient` only if a real contact appears in the source item — never fabricated; unknown contact → `recipient:null`, action `log-contact`.
8. **The fan-out contract** — steps 1–7, DNS-1123 names, background-dispatched handlers ≤8 in flight via `sandbox_wait`, the barrier before `weekly-synth`, and the orchestrator-copies-its-own-`/out` rule.

Terse prompts produce a reducer that editorialises (breaking the gate) and handlers that drop messy files. Over-specify the taxonomy, the schemas, and the gate.

## Output contract

```
/out/
  raw-synthesis.md          # INTAKE mode: factual weekly roll-up, HUMAN-REVIEW gate stamp
  outreach-queue.jsonl      # draft messages/scheduling for host-side finishing (founder-reviewed)
  triage.jsonl              # audit trail: {id, class, confidence, reason, status} per item
  quarantine.jsonl          # sub-threshold + escalate-human items, for human review
  SUMMARY.md                # counts per class, handler errors, scope-doc-mounted flag
  handled/
    <class>/handled.jsonl   # per-class structured records copied from each handler child
  pattern-analysis.md       # PATTERN mode ONLY: cross-cycle trends, priorities, feed-forward
```

`raw-synthesis.md` and `pattern-analysis.md` never coexist from one invocation — the second is a separate, gated run.

## Launching the orchestrator

- **`directories: ["<abs path to inbox>"]` is mandatory.** It mounts as `/in/<inbox>` for the orchestrator and every child; omit it and the pipeline wakes with nothing to triage.
- **`/in/scope-doc/`** (optional but recommended) — the MVP scope doc from `sandbox-mvp-scope-guardrail`. Without it, feature requests are scored generically and stamped `no-scope-doc`.
- **PATTERN mode adds** `/in/reviewed-synthesis/` (the human-annotated `raw-synthesis.md`, mandatory for that mode) and `/in/history/` (prior weeks' `/out` — enables iteration-cycle deltas).
- Tiers: **slow** for the orchestrator, `weekly-synth`, and `pattern-analysis`; **medium** for `classify01` and the handler children.
- Child names: `classify01`, `handle-bug`, `handle-feature-request`, `handle-confusion`, `handle-churn-signal`, `handle-praise`, `weekly-synth`, `pattern-analysis` — all DNS-1123, ≤40 chars.

## Host-side finishing

The sandbox cannot reach Gmail, Calendar, a CRM, or a sheet — those are host-authenticated. After the founder reviews `raw-synthesis.md` (and optionally runs PATTERN mode), the host agent executes `outreach-queue.jsonl` with its OWN connected tools:

- **Contact-list upkeep** — append new reporters/interviewees (rows where `suggested_action:"log-contact"`) to the founder's contact store, e.g. a connected Sheets/CRM MCP tool.
- **Outreach sends** — for `email-reply` and `request-testimonial` rows, send `draft_subject`/`draft_body` to `recipient`, e.g. a connected Gmail MCP tool.
- **Session scheduling** — for `schedule-session` rows (typically churn-signal saves and high-value interview candidates), create calendar holds and invites, e.g. a connected Calendar MCP tool.

Confirm the actual tool names against what is connected in your session; if nothing is connected, hand `outreach-queue.jsonl` to the founder to execute manually. **The founder reviews every draft before it sends** — this is personalized 1:1 follow-up to people who gave feedback, not bulk mail (`needs_founder_review` is `true` on every row). For cold-prospect discovery outreach and interview recruiting, use `sandbox-outreach-pipeline` instead; the recurring cadence that schedules these weekly runs is wired by `sandbox-product-ops-system`.
