---
name: sandbox-gtm-engine
description: Build a company's foundational go-to-market engine through a demesne-orchestrated
  pipeline — a slow-tier orchestrator grounds itself in mounted company evidence and two
  open-web research passes, segments the market across the playbook's four audiences
  (individual users, investors, enterprise buyers, Wall Street analysts), runs a
  generate→judge messaging tournament per audience, fans out per-audience playbook builders
  (user content pipeline, investor metrics narrative, sales playbook, analyst-relations
  strategy) plus a tactical-ops designer (outbound sequences, CRM hygiene, pipeline
  reporting, PR cadence), then a coherence judge forces all four audiences to tell one
  consistent story before delivery. Deliverable is a package of operating docs in /out plus
  an infrastructure build backlog handed to sandbox-feature-work. Apply at Scale stage when
  founder-led growth is flattening and the ask sounds like "build a GTM engine", "we need a
  go-to-market strategy", "messaging architecture", "sales playbook", "analyst relations
  strategy", "investor narrative", "how do we talk about the product per audience". Skip
  when the need is per-named-account procurement gaps (sandbox-enterprise-gap-analysis),
  the procurement doc pack itself — product docs, support playbooks, SLAs
  (sandbox-enterprise-procurement-pack), customer-discovery interview outreach
  (sandbox-outreach-pipeline), competitor/TAM research (sandbox-market-landscape), or
  defining product usage metrics (sandbox-metrics-framework).
allowed-tools: Read, Glob, Grep, Bash, Write, Edit, mcp__demesne__sandbox_agent, mcp__demesne__sandbox_research, mcp__demesne__sandbox_wait, mcp__demesne__sandbox_status
---

Build the foundational GTM engine of playbook activity 25. You (the host session) author one orchestrator prompt, launch a single slow-tier `sandbox_agent`, and it runs the pipeline autonomously: ground → segment → per-audience messaging tournament → per-audience playbook fan-out → coherence gate → deliver. The deliverable is operating docs in `/out` — no code landing; the only build output is `infra-backlog.md`, formatted for a later `sandbox-feature-work` run. This is a HYBRID skill: wiring the recurring motion into live tools is host-side finishing.

**Watch out (cross-cutting):** The two ways this run silently fails are marketing mush and audience drift. Mush: messaging with no traceable proof points reads plausible and is worthless — every proof point must cite a mounted-evidence file or carry an `ASPIRATIONAL` tag, and the eye-roll criterion in the rubric exists to kill it. Drift: four audience tracks written in parallel converge into four different companies — the coherence judge is the barrier that catches it, so never skip it to save a phase. And as always: parallel stages must use `background: true` + `sandbox_wait`; blocking child calls are issued one per turn and run sequentially.

## Procedure

1. **INTAKE** (orchestrator's own process). Write `/workspace/company.md`: product, stage/ARR band, current traction, existing customers, competitors, and an inventory of every mounted evidence file. Evidence arrives messy (exports, notes, decks in mixed formats) — log unparseable files to `/workspace/unparsed.md` rather than silently skipping them. If prior sibling outputs are mounted (`/in/market-landscape`, `/in/metrics-framework`, `/in/pmf-diagnostic`), summarise what they establish.

2. **GROUND** — two medium-tier `sandbox_research` children, dispatched with `background: true`, polled with `sandbox_wait`. They have open egress but a FRESH private workspace and NO `/in` mounts — embed the full company brief in each prompt; harvest from `/out/child/<name>/`. Each claim needs URL + quoted excerpt; uncited claims are omitted.
   - `research-voice` → `/out/child/research-voice/finding.md`: how each of the four audiences actually talks about this category — analyst category names and adjacent Quadrants/Waves, 3–5 competitor messaging teardowns (roof claim, pillars, proof style), user vocabulary from communities, investor framing of the space.
   - `research-benchmarks` → `/out/child/research-benchmarks/finding.md`: current-year refresh of the investor benchmark table and outbound/CRM practice norms embedded below (the embedded figures are directional 2025 values; the refresh supersedes them where cited).

3. **SEGMENT** (orchestrator). Read both findings from `/out/child/`, write `/workspace/segments.md`: market segmentation (2–4 segments with firmographics and buying trigger), then the four-audience map — for each audience, who specifically they are for THIS company, their vocabulary, and their evaluation criteria (spec in the prompt section).

4. **TOURNAMENT: generate.** Four medium-tier `sandbox_agent` children, `msg-gen-users`, `msg-gen-investors`, `msg-gen-enterprise`, `msg-gen-analysts` (DNS-1123: lowercase, digits, interior hyphens, ≤40 chars), dispatched with `background: true` (4 in flight, under the ≤8 window). Each receives `/workspace/company.md` + `segments.md` + the relevant research excerpts embedded in its prompt, plus the three thinking frames in its `preamble`, and writes `/out/candidates.md`: three complete message houses (structure in the prompt section), one per frame.

5. **TOURNAMENT: judge.** Only after all four generators reach a terminal state, dispatch four fresh medium-tier judges `msg-judge-users` … `msg-judge-analysts` (`background: true`). A judge is never a generator scoring its own or a sibling's work. Each reads `/in/previous-jobs/msg-gen-<aud>/candidates.md` (sibling files appear there only after that sibling completes) and writes `/out/verdict.md`: per-candidate rubric scores (1–10 per criterion), a winner, and a must-fix list the builder folds in — no separate refine round.

6. **BUILD** — after all judges are terminal, dispatch five medium-tier children with `background: true` (5 in flight). Each playbook builder reads its judge's verdict at `/in/previous-jobs/msg-judge-<aud>/verdict.md`, produces the final `messaging.md` (winner with must-fixes applied) plus its audience playbook per the specs in the prompt section:
   - `play-users` → `/out/messaging.md`, `/out/content-pipeline.md`
   - `play-investors` → `/out/messaging.md`, `/out/metrics-narrative.md`
   - `play-enterprise` → `/out/messaging.md`, `/out/sales-playbook.md`
   - `play-analysts` → `/out/messaging.md`, `/out/ar-strategy.md` (briefing logistics live here, not in ops)
   - `ops-layer` reads all four verdicts → `/out/outbound-sequences.md`, `/out/crm-hygiene.md`, `/out/pipeline-reporting.md`, `/out/pr-cadence.md`
   Meanwhile the orchestrator writes `/workspace/infra-backlog.md` itself: the product-marketing infrastructure list (prompt section §7) scored exists / partial / missing against the mounted repo and docs, sequenced, formatted as a `sandbox-feature-work` handoff.

7. **COHERENCE GATE** — after all five builders are terminal, spawn one slow-tier fresh judge `coherence-judge`. It reads every builder output at `/in/previous-jobs/play-<aud>/` and `/in/previous-jobs/ops-layer/`, plus `/workspace/segments.md`, and writes `/out/coherence-report.md`: PASS/FAIL per check — (a) all four roofs are audience-dressings of ONE core narrative, not four stories; (b) no cross-audience contradiction (e.g. the investor narrative claims a retention figure the user messaging undercuts); (c) every proof point cites a mounted-evidence path or is tagged `ASPIRATIONAL`; (d) no invented customer quotes; (e) ops-layer cadences reference the actual messaging, not placeholders. On any FAIL, the orchestrator dispatches targeted medium-tier `fix-<aud>` children and re-runs the judge on the touched files — every re-dispatch under a **fresh round-suffixed name** (`fix-<aud>-r2` on round 1, `fix-<aud>-r3` on round 2; the re-run judge is `coherence-judge-r2`, then `-r3`), because reusing the round-1 `fix-<aud>` or the `coherence-judge` name errors and poisons the sibling-mount chain. The re-run `coherence-judge-r2` reads the fixed files at `/in/previous-jobs/fix-<aud>-r2/`. Cap: 2 fix rounds; whatever still fails is listed under `OPEN ITEMS` in the report, never silently passed.

8. **DELIVER** (orchestrator's own process — a `sandbox_script` child would write only to its own `/out/child/<name>/` and strand the files). `cp` each builder's files from `/out/child/<name>/` into the `/out` tree per the output contract, `cp` both research findings to `/out/research/`, **renaming on copy** to the contract names (`research-voice/finding.md` → `research/voice.md`, `research-benchmarks/finding.md` → `research/benchmarks.md`), move `infra-backlog.md` and `segments.md` in, then write `/out/GTM-ENGINE.md` (section order below) and `/out/metadata.json` (audiences, child names, tiers, run date, evidence files used).

## Writing the orchestrator prompt

Brief it as a complete document:

1. **Company brief** — product, stage, ARR band, traction, known competitors, what evidence is mounted where, and any messaging the founder considers fixed.
2. **The four-audience axis** (the organizing spine — encode verbatim): *individual users* — care about ease of use and their specific pain; usually influencer not buyer, the low-risk audience for message testing. *Enterprise economic buyers* — care how much money it makes or saves, not how sophisticated the technology is; want ROI artefacts. *Investors/boards* — evaluate entirely on subscription-quality financial metrics (§5b table). *Wall Street / industry analysts* — value trajectory and category narrative over feature lists; a briefing is a one-way narrative-sharing session, not a sales pitch. Operating principle: one message for all four resonates with none — but it is one core narrative in four vocabularies, never four narratives.
3. **Message-house structure** (every candidate uses it): roof = one umbrella value proposition in the audience's vocabulary; 3–4 pillars = beliefs the audience should hold, NOT features (features go under pillars); foundation = proof points, each typed quantitative / named-customer / third-party and each citing a mounted-evidence path or tagged `ASPIRATIONAL`.
4. **Generator frames** (one candidate per frame, set in `preamble`): *pain-led* — open on the audience's felt problem, product enters as relief; *category-narrative* — open on where the market is going, product as the inevitable answer; *proof-led* — open on the strongest verified result, work backwards to the claim.
5. **The tournament rubric** (1–10 each, embed verbatim in both generator and judge prompts): **eye-roll test** — would a skeptical member of this audience roll their eyes? unsubstantiated superlatives and borrowed jargon score low; **vocabulary match** to `research-voice`; **proof verifiability** — share of proof points tracing to evidence; **differentiation** — names the real competitive alternative (often a spreadsheet or the status quo, not the rival); **core-narrative consistency** with the other audiences' likely story.
6. **Playbook specs** (embed all four; the benchmarks child refreshes figures where cited):
   - *Sales playbook (enterprise)* — a working 70% draft beats a polished one: ≤5 pages; ICP; 3 core talk tracks keyed to the pillars; top-5 objections with responses; qualification staging = BANT (budget, authority, need, timeline) to screen top-of-funnel, graduating to MEDDIC (metrics, economic buyer, decision criteria, decision process, identify pain, champion) for qualified opportunities; a single named owner and a quarterly review date — an ownerless playbook goes stale within one product cycle.
   - *Investor metrics narrative* — a table: metric / definition / company's current value (from mounted data, else `MISSING-INSTRUMENT` with how to measure) / benchmark band / one narrative sentence. Directional 2025 bands for the $1–10M ARR range: NRR 100–110%; CAC payback 18–24 mo acceptable, <12 best-in-class, >24 a red flag; burn multiple ~1.2 median at Series A, >2.5 a problem; Magic Number target 1.0+; LTV:CAC >3:1; gross margin 75%+; Rule of 40 as the headline composite. Close with the Series A gate: proven unit economics, NRR >100%, a credible efficiency path.
   - *Analyst-relations strategy* — founder-owned until there's a CMO; start with free vendor briefings (no client contract needed) to 1–2 Big Three firms plus the category's boutiques (often more accessible and more valuable early); do NOT chase Magic Quadrant/Wave inclusion in year one — realistic timeline 12–36 months; consistent quarterly touches beat a burst of briefings followed by silence (analysts read inconsistency as instability); interim wins: Cool Vendor / New Wave mentions and G2 category presence, which under Series C often moves deals more than Gartner placement. Briefing logistics: 30–60 min, narrative + trajectory + roadmap, no selling.
   - *User content pipeline* — most buyers self-educate before ever talking to sales: map the channels where this segment already learns, one content lane per messaging pillar, a weekly production cadence with owner, and a repurposing chain (long-form → social → sales enablement).
7. **Ops-layer + infrastructure specs**: outbound sequences — 7–13 touches over 2–4 weeks, multi-channel (email, phone, LinkedIn, video), front-load 4–5 touches in week one then taper, separate SDR (longer, higher-volume) and AE (fewer, higher-quality) variants, drafted in the enterprise messaging voice. CRM hygiene — 5–7 pipeline stages tied to buyer actions, never seller activity ("buyer confirmed budget and timeline", not "sent proposal"); written stage-exit criteria; time-in-stage limits with auto-recycle to nurture; mandatory next-step-with-date on every open deal; a weekly stale-deal review. Pipeline reporting — a weekly one-pager: coverage ratio, stage conversion, slipped deals, forecast vs actual. PR cadence — newsroom rhythm tied to the category narrative. Infrastructure backlog items to score exists/partial/missing: interactive demo (top-of-funnel), data-loaded sandbox tenant (deep technical evaluation — a different asset, not a variant), API reference, integration docs, technical one-pagers, plus the enterprise governance layer (SSO, RBAC, audit logs).
8. **The pipeline contract** — the eight steps above; background dispatch + `sandbox_wait` for every parallel stage (≤8 in flight); judges and builders only after their inputs are terminal; the coherence checks and the 2-round fix cap; every child name **unique within the run** — the iterative coherence loop re-dispatches `fix-<aud>` and the judge under fresh round-suffixed names (`fix-<aud>-r2`, `coherence-judge-r2`, then `-r3`) since a reused name errors and breaks the sibling mounts, and the re-run judge reads the `-r2` fix mounts; deliver via the orchestrator's own `cp`.
9. **The output contract** below — include it verbatim.

## Output contract

```
/out/
  GTM-ENGINE.md            # the front door — section order below
  segments.md              # segmentation + four-audience map
  audiences/
    users/       messaging.md  content-pipeline.md
    investors/   messaging.md  metrics-narrative.md
    enterprise/  messaging.md  sales-playbook.md
    analysts/    messaging.md  ar-strategy.md
  ops/
    outbound-sequences.md  crm-hygiene.md  pipeline-reporting.md  pr-cadence.md
  infra-backlog.md         # sequenced sandbox-feature-work handoff
  coherence-report.md      # per-check verdicts + OPEN ITEMS
  research/                # voice.md, benchmarks.md (cited findings)
  metadata.json
```

`GTM-ENGINE.md` sections in order: **The core narrative** (one paragraph — the single story all four audiences hear), **Audience map** (who each audience is here, their vocabulary, their evaluation criteria), **Per-audience summary** (roof + pillars + playbook pointer, four short blocks), **Operating cadence** (the recurring cycles — weekly content/pipeline, quarterly playbook review and analyst touch — that turn this into commercial motion), **Infrastructure backlog summary**, **Open items** (every `ASPIRATIONAL` tag and unresolved coherence failure, in one list).

## Launching the orchestrator

- `directories:` — mount (a) the product repo and/or docs (optional; without it the infra backlog can't be scored against reality and everything scores "missing"), and (b) an **evidence directory** — customer quotes, usage/retention exports, win/loss notes, case studies, financials. The evidence mount is effectively mandatory: forget it and every proof point in every message house comes out `ASPIRATIONAL`, which produces a structurally complete but commercially hollow engine. Prior sibling outputs (`sandbox-market-landscape`, `sandbox-metrics-framework`, `sandbox-pmf-diagnostic` reports) are high-value optional mounts.
- Tiers: **slow** for the orchestrator and `coherence-judge`; **medium** for research, generator, judge, builder, and fix children.
- Child names (DNS-1123, ≤40 chars): `research-voice`, `msg-gen-investors`, `msg-judge-analysts`, `play-enterprise`, `ops-layer`, `fix-users`. No underscores, no uppercase.
- This is a long run (≈16–18 children); launch the orchestrator itself with `background: true` and poll `sandbox_wait`.

## Host-side finishing

The sandbox cannot reach authenticated services, so the pipeline ships designs; the host wires the motion. (1) Hand `/out/infra-backlog.md` to `sandbox-feature-work`, one run per sequenced item. (2) Load the CRM hygiene rules, pipeline stages, and outbound sequences into whatever GTM tools the session actually has connected — e.g. a connected CRM or mail MCP tool; confirm the real tool names against your session, and if nothing is connected, hand the docs to the founder to configure manually. (3) Schedule the recurring cycles from GTM-ENGINE.md's Operating cadence (weekly pipeline review, quarterly playbook review, quarterly analyst touch) via a connected calendar/scheduler tool, or give the founder the cadence list to book. The founder reviews all outbound copy before anything is ever sent — this skill designs sequences; it does not send them.
