---
name: sandbox-interview-kit-design
description: Turn a validated problem hypothesis into a ready-to-run customer-discovery interview kit — an orchestrator derives a precise target profile (job titles, company types, team structure, seniority) and per-persona personas, fans out a `sandbox_research` reachability map (where these people actually gather) alongside per-persona question drafters, then runs a FRESH adversarial auditor over every drafted question that flags leading / future-facing / too-broad framing and attaches a deflection follow-up probe to each survivor, capped at two draft→audit rounds. Apply when you have a stress-tested hypothesis and need discovery interviews that produce honest answers, not confirmation — triggers include "design customer interview questions", "customer discovery kit", "who do I interview and what do I ask", "audit my interview questions for bias", "build an interview guide", "target profile for user interviews". Skip when the hypothesis is still vague and needs sharpening or disconfirming (use sandbox-hypothesis-stress-test — its output is this skill's input), when you already have the kit and need to find and contact prospects (use sandbox-outreach-pipeline), or when interviews are already done and you're synthesising notes (use sandbox-interview-synthesis).
allowed-tools: Read, Glob, Grep, Bash, Write, Edit, mcp__demesne__sandbox_agent, mcp__demesne__sandbox_research
---

Build a customer-discovery interview kit from a validated problem hypothesis. You author one orchestrator prompt, launch a single slow-tier `sandbox_agent`, and it derives the target profile, fans out a reachability-research child and per-persona question drafters, then puts every drafted question through a fresh adversarial auditor before compiling the kit. The deliverable is `/out/KIT.md` (profile + reachability + audited per-persona question sets) plus the audit trail — **REPORT-ONLY**: no code landing, and no outreach or scheduling (that is `sandbox-outreach-pipeline`'s job). The whole point is the audit: the playbook's Idea stage fails on confirmation bias, so questions that presuppose the problem or invite socially-desirable answers are the enemy, not a polish item.

**Watch out (cross-cutting):** The auditor MUST be a different child than the drafter for a persona — a drafter grading its own questions rationalises its leading phrasings and the bias survives into real interviews. The reachability child is `sandbox_research` (open egress) with a FRESH private workspace and NO `/in` mounts, so the full target profile must be embedded verbatim in its prompt — it cannot read `/workspace`. And the orchestrator must `cp` deliverables into its own `/out` itself; a child's files live at `/out/child/<name>/` and delegating the copy to a helper child strands them.

## Procedure

1. **FRAME** (orchestrator's own process). Copy the mounted hypothesis into `/workspace/hypothesis.md`. Write `/workspace/profile.md` — the (a) deliverable: for each persona, precise job titles, company type + size band, team structure, seniority, budget authority, AND an explicit **exclusion list** (who looks adjacent but is NOT the target). Precision beats a long list — a profile that admits everyone screens no one. Write `/workspace/personas.json`: 1–3 persona objects (`slug`, `role`, `one-line who`). Split personas only where the hypothesis genuinely spans distinct roles with different daily reality (e.g. the buyer vs the daily user); do not manufacture personas to pad the kit.

2. **REACHABILITY + DRAFT** (one background wave, ≤8 in flight — see fan-out below). Dispatch together:
   - `reach-research` — a medium-tier `sandbox_research` child. Because it has no mounts, paste all of `profile.md` into its prompt. It returns the (b) deliverable: a **prioritized** reachability map — named subreddits, Slack/Discord communities, LinkedIn groups, industry conferences/meetups, and referral paths where THIS profile actually congregates — each with why they're there, access effort, and a priority tier. Writes `/out/reachability.md` (lands at `/out/child/reach-research/`).
   - one `drafter-<persona-slug>` per persona (medium-tier `sandbox_agent`, e.g. `drafter-inhouse-counsel`, `drafter-legal-ops`). Each reads `/workspace/hypothesis.md` + its persona from `profile.md` and drafts a question set grounded in **past and present concrete behaviour** — what they do today, how often, how severe, what it costs, what workarounds they've built — never the founder's solution. Writes `/out/questions.md`: numbered questions, each tagged with what it aims to learn. These do not depend on reachability, so they run concurrently with it.

3. **AUDIT** (barrier: after all drafters reach a terminal state — `reach-research` may still be running). Per persona spawn a FRESH medium-tier `auditor-<persona-slug>` (never the same child as the drafter). It reads its drafter's set at `/in/previous-jobs/drafter-<slug>/questions.md` and classifies EVERY question against the playbook's three failure modes: **leading** (presupposes the problem or telegraphs the wanted answer), **future-facing/hypothetical** ("would you use…", "would you pay…" — people are optimists about their own future behaviour), **too broad** (invites an idealised self-description instead of a real instance). Verdict per question: keep / revise / cut. For each kept-or-revised question it writes the neutral rewrite (anchored to a recent specific instance — "walk me through the last time you…") and a **deflection follow-up probe**: the redirect to use when the interviewee answers in generalities or gives a socially-flattering answer. Writes `/out/audit.jsonl` (locked schema, one line per question) and `/out/questions-clean.md`.

4. **REVISE** (capped, conditional). If an auditor cut or flagged-for-revision ≥ ~⅓ of a persona's questions, run ONE more round for that persona only: `drafter-<slug>-r2` reads the audit at `/in/previous-jobs/auditor-<slug>/` and produces a corrected set; `auditor-<slug>-r2` (fresh) re-audits. **Hard cap: two draft→audit rounds total** — after the second audit, keep the surviving questions and move on; a third round chases diminishing returns. Names must be unique per parent (the `-r2` suffix); reusing a name errors and poisons later sibling mounts.

5. **COMPILE** (barrier: after all auditors AND `reach-research` are terminal). One medium-tier `compiler` child reads `/in/previous-jobs/reach-research/reachability.md` and each `/in/previous-jobs/auditor-<slug>*/questions-clean.md`, plus `/workspace/profile.md`, and assembles `/out/KIT.md` in interview-ready flow order (see contract). It does not re-audit — it sequences and formats.

6. **DELIVER** (orchestrator's own process). `cp` each deliverable into the orchestrator's own `/out` — do NOT route through a child:
   - `cp /out/child/compiler/KIT.md /out/KIT.md`
   - `cp /out/child/reach-research/reachability.md /out/reachability.md`
   - `cp /workspace/profile.md /out/profile.md`
   - concatenate the per-persona `audit.jsonl` files into `/out/audit-trail.jsonl` and copy each `questions-clean.md` under `/out/questions/<slug>.md`.

**Fan-out loop** (step 2): dispatch every child with `background: true`, collect `job_id`s, poll each with `sandbox_wait` (`timeout_seconds: 120`) until terminal, keep ≤8 in flight. Blocking calls are issued one per turn and run strictly sequentially — using them here serialises the wave and lets later drafters see earlier siblings, losing independence. `reach-research` + up to 3 drafters = 4 jobs, well inside the window.

## Writing the orchestrator prompt

Brief it as a complete document; terse prompts produce shallow, leading questions.

1. **The validated hypothesis** — the specific WHO / HOW OFTEN / HOW SEVERE / WHAT-THEY-DO-TODAY statement (from `sandbox-hypothesis-stress-test` if that ran). Vague hypotheses yield vague profiles and unfocused questions.
2. **The personas** — either the founder's persona split or an instruction to derive 1–3 from the hypothesis, plus the precision-beats-a-list rule and the requirement to write an exclusion list per persona.
3. **The three audit failure modes, verbatim**, with a fix rule for each: leading → strip the presupposition, ask what they actually do; future-facing → convert to a recent real instance ("the last time…"), never intent or willingness-to-pay; too broad → force specificity (frequency, time, money, the concrete last occurrence). State that good discovery questions dig into past/present behaviour and that NO solution pitch, demo, or "would you like it if…" belongs in the set — a prototype is a prop for conversation, not evidence.
4. **The deflection-probe requirement** — every surviving question carries a follow-up for when the answer goes generic ("usually we…" → "the most recent time specifically — when was it, what did you do?") or socially-desirable ("we're pretty on top of it" → "what did the last slip cost you?").
5. **The audit.jsonl schema**, required via each auditor's `output_format`:
   ```
   {"persona":"inhouse-counsel","q_id":3,"original":"Don't you find contract review frustrating?","modes":["leading","too-broad"],"verdict":"revise","rewrite":"Walk me through the last contract review you ran end to end — what did each step take?","probe":"If they generalise, ask: 'the most recent one specifically — when was it?'","learns":"current process + cycle time"}
   ```
6. **The pipeline contract** — the six steps; DNS-1123 child names (lowercase, digits, interior hyphens, ≤40 chars — `drafter-legal-ops`, not `Drafter_LegalOps`); background dispatch + `sandbox_wait` for the wave; auditor is a fresh child, never the drafter; two-round cap; `reach-research` gets the profile pasted into its prompt because it has no mounts.
7. **REPORT-ONLY** — assemble documents only; do not attempt to contact anyone, and note that outreach/scheduling is handled downstream by `sandbox-outreach-pipeline`.
8. **The output contract** below — include it verbatim.

## Output contract

```
/out/
  KIT.md               # The interview kit — see section order below
  profile.md           # (a) precise target profile + exclusions, per persona
  reachability.md      # (b) prioritized reachability map
  questions/
    <persona-slug>.md  # (c) audited, interview-ready question set per persona
  audit-trail.jsonl    # every question's verdict, modes, rewrite, probe (concatenated)
```

`KIT.md` sections in order: **Target Profile** (per persona: titles/company/team/seniority + exclusions), **Where To Find Them** (reachability map, priority-tiered), then per persona an **Interview Guide** — warm-up/context questions, then the current-behaviour core (frequency, severity, workarounds, spend), then a close (referrals, "what didn't I ask about?") — every question printed with its deflection probe beneath it, and a one-line **How to run this** reminder: past-behaviour only, no pitching, listen for the concrete instance.

## Launching the orchestrator

- **`directories:` (mandatory)** — mount the validated hypothesis (a `sandbox-hypothesis-stress-test` `/out`, or a founder-written brief). Copy it into `/workspace` in FRAME; do not edit the mount. If it is forgotten the pipeline has no target and every persona and question is invented from nothing — refuse to proceed and ask for it. Optionally mount any founder notes on known personas or communities.
- **Tier:** slow for the orchestrator; medium for `reach-research`, the drafters, the auditors, the r2 children, and `compiler`.
- **Child names:** DNS-1123 labels — `reach-research`, `drafter-inhouse-counsel`, `auditor-inhouse-counsel`, `drafter-legal-ops-r2`, `compiler`. No underscores, no uppercase, unique within the parent.
