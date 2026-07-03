---
name: sandbox-pivot-diagnostic
description: Run the stalled-PMF pivot diagnostic — after three or more iteration cycles
  without meaningful movement toward PMF benchmarks, an orchestrator profiles the founder's
  retention data deterministically, fans out three parallel analysis lenses (segment
  divergence; designed-value vs experienced-value gap, positioning or product; what would
  have to be true for the current product to reach genuine PMF), then runs an adversarial
  persevere-vs-pivot pair and a fresh judge that issues ADJUST / PIVOT (to a named pivot
  type, per segment evidence) / RETURN-TO-IDEA-STAGE with dissent preserved. Apply when
  "should we pivot", "PMF has stalled", "three cycles and retention hasn't moved",
  "adjust or pivot or start over", "is this a positioning problem or a product problem".
  Skip when you are still measuring whether PMF exists — run sandbox-pmf-diagnostic first
  and feed its verdicts in here; skip for pressure-testing one proposed feature
  (sandbox-mvp-scope-guardrail), for contested decisions without a PMF evidence corpus
  (sandbox-debate-decision), and once you are already back at the drawing board
  (sandbox-hypothesis-stress-test).
allowed-tools: Read, Glob, Grep, Bash, Write, Edit, mcp__demesne__sandbox_agent, mcp__demesne__sandbox_script
---

Decide what a stalled PMF search means: adjust, pivot, or return to the Idea stage. You author one orchestrator prompt, launch a single slow-tier `sandbox_agent`, and it runs the pipeline autonomously against mounted founder evidence — retention/usage exports, the user-feedback corpus, and the ORIGINAL problem hypothesis. The deliverable is `/out/verdict.md` plus lens findings and advocate briefs; report only, no code or branch landing.

**Watch out (cross-cutting):** the founder running this almost always wants to hear ADJUST — the pipeline exists to resist that pull, so the advocates' opposed priors go in verbatim via `preamble` (an empty preamble produces a generic agent) and the judge must be a fresh context that produced none of the analysis. Every quantitative claim traces to the `data-profile` tables — a lens doing its own retention arithmetic silently ruins the run. Advocates dispatch only after ALL lens jobs are terminal: `/in/previous-jobs/<name>/` files appear only when that sibling completes.

## Procedure

1. **INTAKE & TRIGGER CHECK.** The orchestrator reads the evidence mount and writes `/workspace/evidence-index.md` (per file: path, type, parse status — log unreadable files, never silently skip) and `/workspace/hypothesis.md` (the original problem hypothesis verbatim, plus the value the product was *designed* to deliver). Then confirm the trigger: evidence of 3+ iteration cycles without meaningful movement toward the PMF benchmarks (use the founder's benchmarks from `sandbox-metrics-framework` output if mounted). If the record shows fewer cycles or clear movement, write `/out/verdict.md` with verdict **KEEP-ITERATING** and stop — running the pivot diagnostic on one bad week manufactures a pivot.

2. **DATA PROFILE.** Spawn `sandbox_script` (`name=data-profile`, `image=python`, `egress=none`) to compute deterministic tables into `/workspace/tables/`: per-cohort retention curves, retention cut by every segment column the export offers (plan, persona, channel, company size), Sean Ellis tallies by segment where survey data exists (denominator = active users; drop the "no longer use it" bucket). Expect messy real-world files; whatever fails to parse is listed in `/workspace/tables/UNPARSEABLE.md` — lenses then work qualitatively on those items and must label every number they cite as computed or founder-claimed.

3. **LENSES.** Dispatch three medium-tier `sandbox_agent` children with `background: true`, collect `job_id`s, poll with `sandbox_wait` (blocking calls are issued one per turn and run strictly sequentially; ≤8 in flight — three fits easily). Names: `lens-segment`, `lens-value-gap`, `lens-realism` (DNS-1123: lowercase, digits, interior hyphens, ≤40 chars). Each is briefed with exactly one of the three questions (section below), reads `/workspace/hypothesis.md`, `/workspace/tables/`, and the raw evidence mount, and writes `/out/findings.md` with a locked schema: **Answer** (one paragraph), **Evidence** (every claim cited to a file or table), **Confidence** (high/medium/low with why), **What would change this answer**. From the orchestrator these land at `/out/child/lens-*/findings.md`.

4. **ADVERSARIAL SYNTHESIS.** After every lens job reaches a terminal state (this barrier must hold), dispatch two medium-tier advocates with `background: true`: `advocate-persevere` (prior: the current product and segment can reach PMF; must name the specific adjustments and the evidence they'd move) and `advocate-pivot` (prior: iteration is throwing good cycles after bad; must name ONE pivot type from the taxonomy below, the target, and what is preserved). Both read all three `/in/previous-jobs/lens-*/findings.md`; spawned simultaneously, they do not see each other — intentional, so neither softens toward the other. Each writes `/out/brief.md`.

5. **JUDGE.** After both advocates complete, spawn one slow-tier `sandbox_agent` (`name=judge`, fresh context, no stake in any prior output). It reads all five siblings under `/in/previous-jobs/`, plus `/workspace/tables/` and `/workspace/hypothesis.md`, applies the order-of-operations rule (below), and writes `/out/verdict.md`. Verdict is one of **ADJUST** / **PIVOT-\<type\>** / **RETURN-TO-IDEA-STAGE** — forbid the mushy middle: a "partial pivot" blend neither advocate argued defeats the pipeline. The losing advocate's strongest surviving argument goes in the Dissent section with a cheap disconfirming test to run before the founder commits.

6. **DELIVER.** In the orchestrator's own process — never delegated to a `sandbox_script` child, which would strand files under its own `/out/child/<name>/` — `cp` the verdict, tables, lens findings, and advocate briefs into the orchestrator's `/out/` per the contract below, and write `/out/metadata.json` (hypothesis title, cycles reviewed, verdict, models, run date).

## Writing the orchestrator prompt

Brief it as a complete document:

1. **Trigger context** — how many iteration cycles, what changed in each, and which PMF benchmarks (Sean Ellis %, D7/D30, activation) were the targets. Without this the trigger check in step 1 cannot run.
2. **The three questions, verbatim** — these are the lens briefs and the lenses answer nothing else: (a) *is there a segment in this data responding differently than the rest?* (b) *is the gap between designed value and experienced value a positioning problem or a product problem?* (c) *what would have to be true for the current product to find genuine PMF, and is that scenario realistic given what you're seeing?*
3. **Per-lens method** — `lens-segment`: classify each cohort's retention curve — declining-to-zero (no value realized), flatten-and-hold (the plateau cohort is the fit candidate; the LEVEL of the flatten is the measure of fit — consumer apps ~5–15% at 90 days, strong SaaS 40–60%+), or smile; hunt the divergent segment inside the "very disappointed" respondents; beware the tourist pattern — all cohorts identical, fast early churn, no floor means there is no hidden segment to zoom into. `lens-value-gap`: run positioning-first tests before conceding a product gap — the consistency test (the same user confusion appearing across ads, site, sales calls, onboarding, and retention points to positioning, not marketing execution); the alternative-set check (users comparing the product against spreadsheets or the status quo rather than the named rival manufactures phantom product deficiencies); the parity-burden check (the declared category sets baseline expectations — a mis-chosen category makes a fine product feel behind); and the pessimistic-product-thinking guard (claims that "the product is behind" require won/lost-deal evidence, not vibes). Verdict per gap: positioning / product / both, with the journey stage where the evidence sits. `lens-realism`: enumerate every condition that must hold for the current product and segment to hit the benchmarks (≥40% "very disappointed" among active users, retention flattening at a viable level, founder push effort giving way to user pull), then score each condition plausible / strained / contradicted against the tables.
4. **The pivot taxonomy** — the pivot advocate and the judge must use these names, nothing invented: zoom-in (one feature becomes the product), zoom-out (product becomes a feature of something larger), customer-segment (same problem, different segment), customer-need (same segment, different problem), platform, business-architecture (e.g. enterprise ↔ consumer), value-capture, engine-of-growth, channel, technology. Note that channel and technology are GTM/production levers and are rarely the answer to a PMF gap.
5. **Order-of-operations rule for the judge** — (1) rule out a positioning misdiagnosis first via the consistency test: never prescribe a product pivot for a positioning problem — repositioning is an ADJUST; (2) confirm the problem is structural, not perception: retention shape and push-vs-pull, from the tables; (3) only then match pivot type to root cause — divergent thriving segment → customer-segment or zoom-in on what that segment uses; right segment engaging but for a different job → customer-need; and PIVOT requires lens-segment or lens-value-gap evidence naming what is preserved. RETURN-TO-IDEA-STAGE only when lens-realism finds the core conditions contradicted AND no segment shows pull.
6. **The pipeline contract** — the six steps, background dispatch + `sandbox_wait` for both fan-outs, the two barriers (advocates after all lenses; judge after both advocates), the child-naming rule, and that the judge must preserve dissent rather than homogenise it.
7. **Output contract** — the file tree below, and the required `verdict.md` section order.

Terse prompts produce a diagnostic that confirms whatever the founder already believed. Over-specify the evidence and the tests; never state which verdict you expect.

## Output contract

```
/out/
  verdict.md                      # the deliverable
  metadata.json                   # hypothesis title, cycles reviewed, verdict, models, run date
  tables/                         # deterministic retention/segment tables (+ UNPARSEABLE.md if any)
  lenses/
    lens-segment/findings.md
    lens-value-gap/findings.md
    lens-realism/findings.md
  advocates/
    advocate-persevere/brief.md
    advocate-pivot/brief.md
```

`verdict.md` sections in order: **Verdict** (one paragraph: ADJUST / PIVOT-\<type\> with target / RETURN-TO-IDEA-STAGE — or KEEP-ITERATING if the trigger check failed; written last, placed first), **Trigger Check** (cycles reviewed, benchmark movement per cycle), **The Three Questions** (a, b, c — each answered with cited evidence), **Pivot Specification** (only if PIVOT: type, target segment/need, what is preserved, the first cheap validation step; otherwise one line per rejected pivot type and why), **Dissent** (the losing advocate's strongest surviving argument and the disconfirming test to run before committing; non-optional), **Next Steps** (route by verdict: ADJUST → the named adjustments, then re-run `sandbox-pmf-diagnostic` next cycle; PIVOT customer-segment/customer-need → `sandbox-interview-kit-design` against the new profile; RETURN-TO-IDEA-STAGE → `sandbox-hypothesis-stress-test` on the salvaged observation).

## Launching the orchestrator

- **`directories: ["<abs path to pmf-evidence>"]` is mandatory** — it mounts as `/in/<dir>` for the orchestrator and all children. It should contain the retention/usage export (CSV/JSON), the user-feedback corpus (any format — interview notes, support threads, survey exports; messy files expected, unparseable ones logged), the original problem hypothesis (the `sandbox-hypothesis-stress-test` output if it exists), and prior `sandbox-pmf-diagnostic` verdicts if available. Omitting the mount leaves the pipeline with nothing but opinion — the run degenerates into exactly the ungrounded pivot debate this skill exists to prevent.
- **Tiers**: slow for the orchestrator and the judge; medium for the three lenses and both advocates; `data-profile` is a script, no tier.
- **Child names**: `data-profile`, `lens-segment`, `lens-value-gap`, `lens-realism`, `advocate-persevere`, `advocate-pivot`, `judge` — lowercase letters, digits, interior hyphens only, ≤40 chars, unique within the run.
