---
name: sandbox-product-research
status: alpha
description: Drive a product / competitive research pass through a demesne-orchestrated pipeline — a slow-tier orchestrator profiles the target, scans for comparables on the open web, fans out per-avenue subagents in parallel, then a compiler agent synthesises a prioritised executive report with per-avenue appendices identifying feature, documentation, and publishing gaps. Apply when the user wants to understand how a product compares to alternatives and what polished, complete, or competitive looks like for it. Triggers include "product research", "competitive research", "gap analysis", "identify our gaps vs competitors", "what's missing vs competitors", "what does a polished/published version of X look like", "competitive landscape research". Skip for code work (use sandbox-feature-work), internal-codebase audits (use sandbox-quality-improvement), and single open-web questions (call `sandbox_research` directly).
---

Drive a product / competitive research pass through a demesne pipeline. You (the host session) author one orchestrator prompt, launch a single slow-tier `sandbox_agent`, and that orchestrator fans out parallel isolated open-web subagents — one per research avenue — then a compiler agent synthesises a prioritised executive report with per-avenue appendices. The deliverable is markdown in `/out`; there is no code landing.

**Watch out (cross-cutting):** Every capability claim must cite a URL + quoted excerpt; the compiler flags uncited claims rather than promoting them.

## Procedure

1. **INTAKE** — Write `/workspace/target.md`: a one-page profile of the product (what it is, who uses it, what it competes on, the user's initial gap hypotheses). If `/in/<repo>` is mounted, read it; otherwise work from the prompt prose.

2. **COMPARABLES** — Spawn one medium-tier `sandbox_research` child (`name=comparables`; DNS-1123: lowercase letters, digits, interior hyphens, ≤40 chars) to source 5–8 plausible comparables — adjacent categories, ecosystem peers, "what would a buyer compare us to" — with one paragraph of rationale each. `sandbox_research` has open-web egress and a fresh private `/workspace` with no `/in` mounts; embed everything it needs in the prompt. Output: `/workspace/comparables.md`. The orchestrator may adjust the list; the verification avenue (step 3) catches missed peers.

3. **DECOMPOSE** — Write `/workspace/avenues.json`. Hybrid dimension-first rule: one avenue per dimension (features, documentation, publishing, community/ecosystem, governance/sustainability) plus one **verification avenue** that cross-checks the comparables list for missed peers. Target 5–8 avenues; never more than 10. Effort scaling: 1 subagent for fact-finding, 2–4 for comparisons, 5–10 for a full landscape. Each avenue object: `{name, scope, comparables, dimension, research_question, tool_call_budget, output_format}`. The compiler's Methodology section must name any comparables that were excluded from analysis and give a one-line reason for each — a reader needs to see the analysis boundary.

4. **AVENUES** — Dispatch each `sandbox_research` child with `background: true` (collect its `job_id`) and poll with `sandbox_wait` until all finish — blocking calls are issued one per turn and run sequentially, so background dispatch is what makes the avenues run concurrently. Keep **≤8 in flight** (a host-resource guard, demesne enforces no cap); for N > 8, launch a replacement as each finishes. Name each `avenue-<slug>` (DNS-1123). Each child runs with open egress, a fresh private `/workspace`, and no `/in` mounts; embed the full avenue object and all needed context in its prompt. Each avenue prompt must require: ≥3 *distinct* source types (GitHub, package registry, docs site, community forum, primary website, etc.), a tool-call budget of 10–25, and per-claim citation — URL + access date + quoted excerpt; omit rather than infer uncited claims. For fast-moving categories, note publication dates and flag sources older than ~12 months. Output: `/out/child/<avenue>/finding.md` (per-comparable table + gap summary).

5. **COMPILE** — Spawn one slow-tier `sandbox_agent` compiler. It reads `/workspace/target.md`, `/workspace/comparables.md`, and all avenue findings at `/in/previous-jobs/<avenue>/finding.md` (the sibling mount registers at compiler creation; files become visible once each sibling completes). It synthesises `report.md` plus `appendices/appendix-<n>-<avenue>.md` (findings copied verbatim) into its `/out`. Every claim in `report.md` must cite its appendix. Conflicting findings are flagged with a recommended verification action — not silently resolved. Flag if >50% of sources across appendices come from one domain.

6. **DELIVER** — In the orchestrator's own process, `cp -r /out/child/<compiler-name>/. /out/`. Do not delegate to a `sandbox_script` child; that child writes only to `/out/child/<name>/` and would strand the report. Write `/out/metadata.json`: target, comparables list, avenue names, models used, run date. The orchestrator must `cp` the compiler's output into its own `/out` directly — delegating the copy strands the report.

## Writing the orchestrator prompt

Brief it as a complete document:

1. **The target** — what the product is, the user it serves, initial gap hypotheses, and any comparables the user wants treated as fixed. Mount `/in/<repo>` if there is one.
2. **The pipeline contract** — the six steps above; emphasise the parallel fan-out in step 4 and the deliver-via-own-`/out` rule.
3. **The decomposition rule** — hybrid dimension-first with a verification avenue; 5–8 avenues, never >10. Embed the avenue-object schema.
4. **Frameworks to apply where relevant** (not mechanically):
   - **[Diátaxis](https://diataxis.fr/)** for documentation — tutorial / how-to / reference / explanation.
   - **Kano model** for feature classification — must-have / performance / delighter.
   - **Jobs-to-be-done** to broaden the competitor set ("any tool that accomplishes this job competes").
   - **[OSSF Scorecard](https://scorecard.dev/)**, **GitHub community standards**, **[opensource.guide](https://opensource.guide/)** for OSS publishing/community checklists.
5. **Prioritisation** — every recommendation in `report.md` must include an **ICE score** (Impact × Confidence × Ease, each 1–10), a one-line effort estimate, and a suggested owner role. Escalate to RICE if real usage data is available; use MoSCoW to categorise within ICE rankings.
6. **Subagent prompts** — embed the avenue object plus: the ≥3-distinct-source-types rule, tool-call budget, structured-output requirement, and an explicit "do not infer capabilities from general reputation; cite URL + quoted excerpt or omit the claim" instruction.
7. **Compiler prompt** — five-section report structure below; forbid claims not cited to an appendix.

## Output contract

```
/out/
  report.md                       # The deliverable
  appendices/
    appendix-a-<avenue-name>.md   # One per avenue
  metadata.json                   # target, comparables, avenues, models, run date
```

`report.md` sections in order: **TL;DR / Executive Summary** (≤500 words; the 3–5 prioritised recommendations as bullets; written last, placed first), **Methodology** (which comparables, why each, dimensions assessed, known limitations), **Cross-Cutting Findings** (patterns across ≥2 avenues; not a list of facts), **Prioritised Recommendations** (ICE-scored table + per-item narrative with owner + effort), **Appendix Index** (link to each appendix).

## Launching the orchestrator

- **`directories: ["<abs path>"]`** if the target is a repo on disk. Optional.
- **Slow** tier for the orchestrator and compiler; **medium** tier for all subagents.
- Tell the orchestrator to dispatch avenue subagents with `background: true` and poll with `sandbox_wait` (≤8 in flight) — blocking calls are issued one per turn and run sequentially, defeating the parallel fan-out.
## Host-side landing

There is no code landing — the deliverable is `/out/report.md`. Read it, share it, decide on the recommendations. Optionally copy `/out/` into the repo or a docs store. No `make validate`, no branch fetch, no commit — this skill produces a research artefact, not a code change.
