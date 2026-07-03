---
name: sandbox-outreach-pipeline
status: alpha
description: Run customer-discovery outreach as an operational pipeline through a demesne orchestrator — a slow-tier orchestrator reads a target profile, fans out `sandbox_research` children to build a prospect list of real named individuals with public verifiable contact channels (source-cited, confidence-flagged, no guessed emails), fans out drafting children for a personalized 1:1 outreach message per prospect, plans the day-7 follow-up cadence, has a FRESH auditor strip spam/over-templating/uncited-personalization, and emits drafts + cadence + `tracking.csv` as structured output. HYBRID: the sandbox drafts and structures only; a `## Host-side finishing` step sends mail / creates calendar holds / updates the sheet via the host session's OWN connected tools after the founder reviews every draft. Apply when the user wants to "build a prospect list and reach out", "run interview outreach", "cold email discovery targets", "set up the outreach + follow-up cadence", "manage the reply/scheduling pipeline for customer interviews". Skip when you still need the target profile and interview questions (build those first with sandbox-interview-kit-design), when interviews are done and you are synthesising notes (sandbox-interview-synthesis), or when the ask is general open-web research with no outreach artefact (sandbox-product-research).
allowed-tools: Read, Glob, Grep, Bash, Write, Edit, mcp__demesne__sandbox_research, mcp__demesne__sandbox_agent, mcp__demesne__sandbox_script, mcp__demesne__sandbox_wait, mcp__demesne__sandbox_cancel
---

Run customer-discovery outreach as an operational pipeline (playbook activity 7). You (the host session) author one orchestrator prompt, launch a single slow-tier `sandbox_agent`, and it runs the pipeline autonomously: research a prospect list, draft one tailored message per prospect, plan the follow-up cadence, audit the drafts, and emit them as structured files. **The sandbox produces drafts and sheets only — it never sends anything.** The deliverable is `/out/outreach/` (per-prospect drafts + subjects), `/out/cadence.md`, and `/out/tracking.csv`; there is no code landing. A host-side finishing step (below) does the actual sending with the host's connected tools, and only after the founder reviews every draft.

**Watch out (cross-cutting):** This is 1:1 personalized research outreach, **not a bulk-mail engine** — cap the list (default ≤40 prospects) and refuse to scale past it. Two failure modes silently ruin a run: (1) a research child inventing a plausible email and presenting it as verified — every contact channel MUST carry a source URL + a confidence flag, and guessed addresses are marked `guessed`, never `verified`; (2) drafts that are template-with-mailmerge wearing a personalization costume — the auditor exists to catch exactly this.

## Procedure

1. **INTAKE** — orchestrator's own process. Read the target profile: if `sandbox-interview-kit-design` output is mounted at `/in/interview-kit/`, read its target-profile + reachability map; else work from the prompt prose. If the founder mounted a seed list (`/in/seeds/` — known prospects, warm intros, a LinkedIn export, a half-filled spreadsheet in whatever format), read it too; children must handle messy real-world files and **log** anything unparseable rather than skip it silently. Write `/workspace/profile.md` (job titles, company types, seniority, the per-persona reachability channels) and `/workspace/scope.md` (prospect cap, personas, consent note). No profile mounted and none in the prompt → stop and ask; the whole pipeline is targeting-blind without it.

2. **PROSPECT RESEARCH (fan-out — `sandbox_research`)** — one child per persona/segment, `name=prospect-<persona-slug>` (DNS-1123: lowercase, digits, interior hyphens, ≤40 chars; e.g. `prospect-eng-manager`, `prospect-ops-lead`). `sandbox_research` has open-web egress but a **FRESH private workspace with NO `/in` mounts** — embed the persona's profile slice, the channel rules, and the per-persona prospect quota directly in each prompt. Each child finds real *named* individuals matching the profile and returns, per prospect: name, role, company, and every **public, verifiable** contact channel (personal site, company contact page, public profile, conference bio) each with a source URL + `verified|likely|guessed` confidence flag. Hard rule in the prompt: never fabricate an email; if only a pattern is inferable, emit it as `guessed` with the reasoning. Output: `/out/child/<name>/prospects.jsonl`. Dispatch each with `background: true`, collect `job_id`s, poll `sandbox_wait` (`timeout_seconds: 120`), keep **≤8 in flight** — blocking calls are issued one per turn and run sequentially, so background dispatch is what makes the personas run concurrently; `sandbox_cancel` kills a stuck child.

3. **MERGE + DEDUPE (deterministic — `sandbox_script`)** — `name=merge-prospects`, `image=python`, `egress=none`. After every research job reaches a terminal state (barrier: `sandbox_wait` on all `job_id`s first — sibling files appear at `/in/previous-jobs/prospect-*/prospects.jsonl` only once each completes), it reads all `prospects.jsonl`, dedupes by (name, company), drops rows with no channel at all, enforces the prospect cap from `scope.md`, and writes `/workspace/prospects.jsonl` under a **locked schema** (`prospect_id, name, role, company, persona, channels:[{type,value,source_url,confidence}], notes, personalization_hook`). This is deterministic set work — never an LLM agent.

4. **DRAFT FAN-OUT (fan-out — `sandbox_agent`, medium tier)** — batch `/workspace/prospects.jsonl` into groups of ~8, one child per batch, `name=draft-batch-01`, `draft-batch-02`, …. `sandbox_agent` children inherit `/workspace`, so each reads its assigned `prospect_id`s and `/workspace/profile.md` directly (no web). Per prospect the child writes ONE tailored message keyed off that prospect's specific role/context/`personalization_hook` (playbook: personalized per individual, not per persona), plus a subject line and the single ask (a 20-minute problem-discovery call, per the interview kit). Output: `/out/child/<name>/<prospect_id>.md` (front-matter: prospect_id, to-channel, subject; body below). Background-dispatch ≤8 in flight, same loop as step 2.

5. **CADENCE + AUDIT (barrier, then a FRESH auditor)** — after all draft batches finish:
   - Orchestrator writes `/workspace/cadence.md`: the send → **day-7 follow-up for non-responders** (one short bump, references the original, restates the ask) → close-out after the second non-reply. State max two touches per prospect; a third is spam.
   - Spawn one medium-tier `sandbox_agent` `name=outreach-audit` — a **fresh context, never a drafter scoring its own work**. It reads every draft at `/in/previous-jobs/draft-batch-*/` and scores each against the rubric in the orchestrator prompt: spam/salesy tone, over-templating (near-identical bodies across prospects), a personalization claim not traceable to that prospect's `notes`/`source_url`, missing/soft ask, length >150 words. It writes `/out/child/outreach-audit/audit.md` with a per-draft `keep|revise` verdict and the reason. One revise round only: the orchestrator hands failing drafts back to a single `draft-fix` child, then the auditor re-checks; **cap at 2 rounds** and ship what passes, listing any still-flagged draft in the summary.

6. **BUILD TRACKING + DELIVER** — `sandbox_script` `name=build-tracking` (`image=python`, `egress=none`) reads `/workspace/prospects.jsonl` + the audit verdicts and writes `/out/tracking.csv` (columns: `prospect_id,name,company,persona,channel,confidence,draft_status,sent_date,followup_due,responded,interview_booked,notes` — rows pre-filled through `draft_status`, the rest blank for the founder/host to fill). Then, **in the orchestrator's own process**, `cp` every `<prospect_id>.md` into `/out/outreach/`, `cp /workspace/cadence.md /out/cadence.md`, and copy `audit.md` to `/out/audit.md`. Do **not** delegate this copy to a `sandbox_script` child — its `/out` is `/out/child/<name>/` and the files would be stranded. Write `/out/SUMMARY.md` (prospects found, verified vs guessed channels, drafts written, drafts still flagged, personas run) and print `DONE`.

## Writing the orchestrator prompt

Brief it as a complete document:

1. **The target** — the profile source (`/in/interview-kit/` if mounted, else prose), the personas, the prospect cap, and the seed-list mount (`/in/seeds/`) if any. State the consent/scope guardrail explicitly: personalized 1:1 outreach, founder reviews every draft before anything sends, hard cap on volume.
2. **The pipeline contract** — the six steps; emphasise the two barriers (all research done before merge; all drafts done before audit) and the deliver-via-own-`/out` rule.
3. **Channel rules for research children** — public + verifiable only; source URL per channel; `verified|likely|guessed` flag; **never present a guessed email as verified**; log unreachable personas rather than inventing contacts.
4. **The draft rubric** (the message the founder would be proud to have sent):
   - Opens with the specific reason *this person* was chosen (role, a public thing they said/shipped) — traceable to `notes`/`source_url`.
   - One sentence of context on the founder + the problem being explored (not the product being sold).
   - A single, low-friction ask: a 20-minute problem-discovery call — no pitch, no deck.
   - ≤150 words, plain text, no marketing voice, easy to say no to.
5. **The audit rubric** (step 5) verbatim, and the 2-round fix cap.
6. **The cadence** — send, day-7 single follow-up for non-responders, close after the second non-reply; max two touches.
7. **Output contract** — the file tree below; report/draft artefacts only, no sends inside the sandbox.

## Output contract

```
/out/
  outreach/
    <prospect_id>.md     # one tailored draft per prospect (front-matter: to-channel, subject)
  cadence.md             # send + day-7 follow-up + close-out rules
  tracking.csv           # prospect-status sheet, pre-filled through draft_status
  audit.md               # per-draft keep/revise verdicts + reasons
  SUMMARY.md             # prospects found, verified/guessed split, drafts flagged, personas run
```

## Launching the orchestrator

- **`directories:`** — mount `/in/interview-kit/` (the `sandbox-interview-kit-design` output) if it exists; mount `/in/seeds/` if the founder has a seed list. Neither is strictly mandatory, but with no profile mounted **and** none in the prompt the run is targeting-blind and step 1 will stop.
- Tier: **slow** for the orchestrator; **medium** for research, drafting, and audit children; `sandbox_script` for merge + tracking.
- Child names: `prospect-<persona>`, `merge-prospects`, `draft-batch-01…`, `outreach-audit`, `draft-fix`, `build-tracking` (all DNS-1123).
- Tell the orchestrator to background-dispatch every fan-out stage and poll `sandbox_wait` (≤8 in flight) — blocking calls run one per turn, sequentially, defeating the parallelism.

## Host-side finishing

The sandbox cannot reach Gmail, Google Calendar, or any interactively-authenticated service — it produced drafts and a sheet, nothing was sent. Back in the host session:

1. **Founder reviews every draft.** Read each `/out/outreach/<prospect_id>.md`. This gate is non-negotiable — it is what makes this 1:1 research outreach rather than spam. Cut or rewrite anything that reads generic, and delete any prospect whose contact channel is flagged `guessed` unless the founder is willing to stand behind it.
2. **Send** the approved drafts to each prospect's `to-channel`, **e.g. a connected Gmail/mail MCP tool** — confirm the actual tool name against what is connected in your session. Update `sent_date` and `followup_due` (= sent + 7 days) in `tracking.csv` as you go.
3. **Schedule** replies: when a prospect agrees, create the interview hold, **e.g. a connected Google Calendar/calendar MCP tool** (confirm the real name), and set `interview_booked` in the sheet.
4. **Day-7 follow-up:** for rows past `followup_due` with `responded` blank, send the single follow-up from `cadence.md`, then mark closed after the second non-reply.
5. **If nothing is connected** — hand `/out/outreach/`, `/out/cadence.md`, and `/out/tracking.csv` to the founder to send and track manually. The pipeline's job is done: it produced review-ready drafts, a cadence, and a live tracking sheet.
