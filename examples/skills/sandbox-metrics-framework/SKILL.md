---
name: sandbox-metrics-framework
description: Define a product's measurement framework BEFORE the first user arrives — the orchestrator frames the product's single core value action, business model, and natural usage cadence, fans out parallel generators that each propose a 3–5 metric set through a distinct lens (activation, retention, engagement-depth, monetization, referral), runs a sandbox_research child that grounds category benchmarks (activation rates, D7/D30 retention) in cited comparables, a fresh judge scores every candidate against an actionability/vanity-resistance rubric and selects the metrics that matter, an adversarial child builds the false-positive signature table (signups-without-activation, revenue-without-retention, enthusiasm-without-repeat-usage) each paired with the honest metric that unmasks it, and a compiler emits the framework plus an instrumentation checklist. Apply when a founder is pre-launch and needs metrics fixed in advance so they cannot be chosen after the fact to fit whatever happened — "which metrics matter for my product", "define activation and retention targets", "set D7/D30 benchmarks", "what does a false positive look like", "build the measurement framework before launch", "what events should I instrument". Skip when users already exist and you want the PMF verdict (use sandbox-pmf-diagnostic, which consumes this framework's signatures and targets), when you want the instrumentation built (hand INSTRUMENTATION.md to sandbox-feature-work), when routing live feedback (sandbox-feedback-loop-ops), or when the rubric-judged artefact is a general design rather than a metric set (sandbox-tournament-search).
allowed-tools: Read, Glob, Grep, Bash, Write, Edit, mcp__demesne__sandbox_agent, mcp__demesne__sandbox_research
---

Fix a product's metrics before launch so they measure real value, not momentum. You author one orchestrator prompt, launch a single slow-tier `sandbox_agent`, and it frames the product, fans out lens-specific metric generators plus one benchmark-research child, runs a fresh judge to select the 3–5 metrics that matter, an adversarial child to build the false-positive table, and a compiler to assemble the framework and an instrumentation checklist. The deliverable is `/out/METRICS.md` + `/out/INSTRUMENTATION.md` — a report and a spec, **not** landed code; the checklist is handed to `sandbox-feature-work`, the signatures and targets feed `sandbox-pmf-diagnostic`.

**Watch out (cross-cutting):** The retention window must match the product's natural usage cadence — D7/D30 on a product people genuinely use monthly is noise, and a benchmark anchored to the wrong cadence poisons every downstream verdict. Vanity metrics survive when the context that proposed a metric also judges it, so the judge scores candidates it did not write and the false-positive attack runs in a fresh context. Benchmark numbers with no citation are hypotheses, not baselines — the research child drops uncited figures rather than inventing them.

## Procedure

1. **FRAME** — orchestrator's own process. Write `/workspace/product.md`: one-sentence product, the SINGLE core value action (the one thing a user does that delivers the value), target user, business model (free / freemium / subscription / usage-based), growth motion, and — load-bearing — the natural usage cadence (daily / weekly / monthly / episodic), which decides *which retention window is even meaningful*. If the founder mounted no product doc, fill `product.md` from the intake questionnaire (below). Write `/workspace/rubric.md`: the five metric-selection criteria with 1–10 scales (below). A `product.md` with no explicit core action makes every generator fall back to generic dashboard vanity metrics.

2. **RESEARCH benchmarks** — dispatch now with `background: true`. One `sandbox_research` child, name `benchmark-research`, medium tier, open egress. It has a FRESH private workspace and NO `/in` mounts, so embed the product category, business model, and usage cadence *directly in its prompt* — it cannot read `/workspace/product.md`. Task: gather cited activation-rate and cadence-appropriate retention baselines (D7/D30 for daily/weekly products; the right window otherwise) for comparable products, with median-vs-good-vs-poor bands where the source gives them. Require a URL + figure per baseline; drop anything uncited. It writes `/out/benchmarks.md`. Runs concurrently with the generators.

3. **GENERATE candidate metric sets** — fan-out, medium tier. Dispatch N=4–5 `sandbox_agent` children with `background: true`, keep ≤8 in flight, poll `sandbox_wait`. Names are DNS-1123 labels — `gen-activation`, `gen-retention`, `gen-engagement`, `gen-revenue`, `gen-referral` (lowercase, digits, interior hyphens, ≤40 chars; never `gen_revenue` or `Gen-Referral`, which break sibling mounts). Put the lens in `preamble`, not `prompt` — a lens buried in `prompt` becomes a suggestion the generator ignores. Each reads `/workspace/product.md` and writes `/out/candidate.md`: 3–5 metrics, each with name, precise definition, why it matters for THIS product, leading-or-lagging, and a proposed target hypothesis. Blocking dispatch would serialise them one per turn — use background.

4. **JUDGE** — fresh context, slow tier, name `judge`. Barrier: spawn only after every generator AND `benchmark-research` reaches a terminal state (`sandbox_wait` on all `job_id`s) — `/in/previous-jobs/<name>/` mounts register at child-create but files appear only when the sibling finishes. It reads each `/in/previous-jobs/gen-*/candidate.md` and `/in/previous-jobs/benchmark-research/benchmarks.md`, scores every candidate metric against the rubric, and selects the final **3–5 metrics that matter** across frames — the set MUST include at least one activation metric and at least one retention metric (the playbook names both). Writes `/out/scores.jsonl` (one row per candidate metric) and `/out/chosen.md` (the selected set, per-metric rationale, benchmark target attached from research or marked "unbenchmarked — hypothesis target"). Schema, enforced via `output_format`:
   ```
   {"candidate":"gen-retention","metric":"week-4 core-action retention","scores":{"actionability":9,"leading_power":8,"vanity_resistance":9,"specificity":8,"instrumentability":7},"total":41,"selected":true,"rationale":"..."}
   ```

5. **FALSE-POSITIVE ATTACK** — fresh context, slow tier, name `fp-attacker`, spawned after `judge` completes. It reads `/in/previous-jobs/judge/chosen.md` and `/workspace/product.md` and, for THIS product, instantiates the playbook's three vanity signatures — **signups-without-activation**, **revenue-without-retention**, **enthusiasm-without-repeat-usage** — plus any product-specific ones, each as a row: vanity signal, why it misleads, the HONEST metric that unmasks it, and the threshold separating real from false. Writes `/out/false-positives.md`. A fresh context is mandatory: the metric's author rationalises its own blind spot.

6. **COMPILE** — slow tier, name `compiler`. Reads `/in/previous-jobs/judge/chosen.md`, `/in/previous-jobs/benchmark-research/benchmarks.md`, `/in/previous-jobs/fp-attacker/false-positives.md`. Writes `/out/METRICS.md` (section order below) and `/out/INSTRUMENTATION.md` — per chosen metric: the event(s) to emit, their properties, the aggregation/cohort computation (spell out the D7/D30 cohort definition against the product's cadence), and a done-criterion — as a checklist for `sandbox-feature-work`. This skill emits the spec; it does NOT write instrumentation code.

7. **DELIVER** — orchestrator's own process, `cp`, never a child (a `sandbox_agent`/`sandbox_script` child writes only to `/out/child/<name>/`, stranding the files). `cp` each child's deliverable into the orchestrator's own `/out/` per the contract, and write `/out/metadata.json` (product, chosen metrics, run date). REPORT-ONLY: no branch, no landed code.

## Writing the orchestrator prompt

Brief it as a complete document; terse prompts produce generic dashboards:

1. **The product** — one sentence, the single core value action, target user, business model, growth motion, and natural usage cadence. If the founder has no doc, pose the **intake questionnaire**: What is the one action that delivers the value? Who does it and in what context? What does "activated" mean — the moment a user has clearly gotten the value at least once? How often would a happy user naturally return (daily/weekly/monthly/episodic)? How does the product make money, and what is the growth motion?
2. **The five frames** verbatim — activation (the aha/first-value moment), retention (N-day return + cohort curves), engagement-depth (breadth/frequency of the core action), monetization (conversion, expansion, willingness-to-pay), referral (word-of-mouth loops). Provide all five; do not leave frame choice to the orchestrator.
3. **The rubric** — five 1–10 criteria, higher is better: *actionability* (does a number move a specific decision?), *leading power* (does it predict week-6/12 PMF early, not only confirm it late?), *vanity-resistance* (how hard to inflate without delivering value — signups/pageviews score low, repeat core-action scores high), *product-specificity* (tied to THIS core action, not a generic metric), *instrumentability* (measurable pre-launch with reasonable event tracking).
4. **The pipeline contract** — the seven steps; background dispatch + `sandbox_wait` for the generators and research (blocking children run one per turn, sequentially); the judge barrier after all generators + research; the judge scores candidates it did not author; the FP attack runs fresh after the judge.
5. **The three false-positive signatures** verbatim as the required floor for `fp-attacker`, each needing an honest-metric unmasker and a threshold.
6. **The scores.jsonl schema** verbatim as `output_format` for the judge.
7. **The output contract** below — include it verbatim, and state 3–5 metrics with ≥1 activation and ≥1 retention.

## Output contract

```
/out/
  METRICS.md            # the framework — main deliverable
  INSTRUMENTATION.md    # per-metric event/cohort checklist for sandbox-feature-work
  benchmarks.md         # cited comparable-product baselines
  false-positives.md    # vanity signal -> honest metric -> threshold
  chosen.md             # judge's selected metric set with rationale
  scores.jsonl          # judge scores, one JSON object per candidate metric
  metadata.json         # product, chosen metrics, run date
  candidates/
    gen-activation/candidate.md
    gen-retention/candidate.md
    gen-engagement/candidate.md
    gen-revenue/candidate.md
    gen-referral/candidate.md
```

`METRICS.md` sections in order: **Product & core value action** (one paragraph + the natural cadence, stated), **The metrics that matter** (3–5, each: definition, why it matters, leading/lagging), **Benchmark targets** (activation criteria + cadence-appropriate retention targets, each with its cited baseline or flagged "hypothesis — unbenchmarked"), **False-positive signatures** (the table: vanity signal → why it misleads → honest metric → threshold), **Instrumentation** (pointer to INSTRUMENTATION.md), **Feeds** (how `sandbox-pmf-diagnostic` consumes these targets and signatures downstream).

## Launching the orchestrator

- **`directories:`/`files:`** — optional: mount a product spec, PRD, or repo to ground the core-action definition. Optional but recommended; with nothing mounted the intake questionnaire is the sole input and a thin `product.md` yields generic metrics.
- **Tiers** — slow for the orchestrator, `judge`, `fp-attacker`, and `compiler`; medium for the generators and `benchmark-research` (the orchestrator sets these when spawning).
- **Child names** — DNS-1123 labels: `gen-activation`, `gen-retention`, `benchmark-research`, `judge`, `fp-attacker`, `compiler`. No underscores, no uppercase, ≤40 chars, unique within the run.
