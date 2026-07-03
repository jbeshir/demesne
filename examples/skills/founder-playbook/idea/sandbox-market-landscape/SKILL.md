---
name: sandbox-market-landscape
status: alpha
description: Drive an Idea-stage market-landscape research pass through a demesne pipeline — a slow-tier orchestrator profiles the founder's hypothesis, fans out open-web `sandbox_research` children (one per competitor tier plus market-sizing, trends, and analogous-markets), runs a fresh adversarial pressure-test stage that steelmans each tier's threat and attacks every sizing assumption, then a compiler synthesises one landscape report with three sections — a four-tier competitor map, a pressure-tested TAM/SAM/SOM model with buyer landscape, and a two-year tailwind/headwind trend read. Apply when a founder needs to understand the competitive field, market size, and timing behind a hypothesis before committing to build. Triggers include "map the competitive landscape", "who are our competitors", "TAM SAM SOM", "market sizing", "is the market expanding or consolidating", "trend analysis", "tailwinds and headwinds", "market landscape research". Skip for mining competitor reviews for unresolved complaints (use sandbox-competitor-complaint-mining), sharpening a raw observation into a hypothesis or hunting disconfirming evidence (use sandbox-hypothesis-stress-test), building interview question sets (use sandbox-interview-kit-design), and single open-web questions (call `sandbox_research` directly).
allowed-tools: Read, Glob, Grep, Bash, Write, Edit, mcp__demesne__sandbox_agent, mcp__demesne__sandbox_research
---

Drive an Idea-stage market-landscape research pass through a demesne pipeline. You (the host session) author one orchestrator prompt, launch a single slow-tier `sandbox_agent`, and that orchestrator fans out parallel open-web `sandbox_research` children (competitors by tier, market sizing, trends, analogous markets), runs a fresh adversarial pressure-test stage over their findings, then a slow-tier compiler synthesises one three-section landscape report. The deliverable is markdown in `/out`; there is no code landing. This skill merges the playbook's three landscape activities — tiered competitor mapping (with a per-tier threat case), TAM/SAM/SOM sizing, and two-year trend analysis — into one pass. It does **not** mine competitor reviews (that is `sandbox-competitor-complaint-mining`).

**Watch out (cross-cutting):** The failure mode this pipeline exists to counter is *competitor neglect* — dismissing every tier's threat with the easiest rebuttal — so the threat-case children must be fresh adversaries, never the researcher scoring its own map. `sandbox_research` children run in a **fresh private workspace with no `/in` mounts and cannot read `/workspace`**, so the hypothesis and target market must be embedded verbatim in every research prompt; a child sent without them returns a generic industry survey unanchored to this founder. Every sizing number and capability claim must carry a URL + quoted excerpt; the compiler demotes uncited figures to "unverified assumption" rather than promoting them into the model.

## Procedure

1. **INTAKE** (orchestrator's own process). Write `/workspace/hypothesis.md`: the founder's hypothesis in the playbook's shape — WHO exactly, HOW OFTEN, HOW SEVERELY, WHAT THEY DO TODAY — plus the product concept, the target market/geography, and any competitor names the founder already knows. If `/in/hypothesis/` is mounted (output of `sandbox-hypothesis-stress-test`, including its counter-case), read it and carry its disconfirming signals forward as threat-case seeds. Then write `/workspace/avenues.json`: the research-avenue list (default 7 below; the orchestrator may add avenues for a crowded space, never exceeding 10).

2. **RESEARCH** (fan-out, medium-tier `sandbox_research`, background dispatch). Default seven avenues, each its own child: the four competitor tiers — `research-tier-direct`, `research-tier-indirect`, `research-tier-acquirers`, `research-tier-adjacent` — plus `research-sizing` (TAM/SAM/SOM data + market direction + buyer landscape), `research-trends` (regulatory/technological/demographic), and `research-analogous` (markets where this problem was already solved). Dispatch each with `background: true`, collect its `job_id`, and poll with `sandbox_wait` (`timeout_seconds: 120`) until all reach a terminal state — blocking calls are issued one per turn and run strictly sequentially, so background dispatch is the only thing that makes the avenues run concurrently. Keep **≤8 in flight**. Each child has open egress but a fresh private `/workspace` and no `/in` mounts; embed the full avenue object, the hypothesis, and the citation rule in its prompt. Each writes `/out/child/<name>/finding.md`: a cited evidence table (≥3 distinct source types, URL + access date + quoted excerpt per claim, publication dates flagged when older than ~12 months) plus a first-pass draft for its avenue.

3. **PRESSURE-TEST** (fan-out, medium-tier `sandbox_agent`, background dispatch). Dispatch **only after every RESEARCH job is terminal** — these children read completed siblings at `/in/previous-jobs/<research-name>/finding.md`, and that mount is empty until the sibling finishes. Fresh adversaries, none scoring its own research: one threat-case child per tier — `threat-direct`, `threat-indirect`, `threat-acquirers`, `threat-adjacent` — each reading its tier's finding and writing the strongest honest case for *why this tier genuinely threatens us* (steelman, not the dismissible version); plus `sizing-critic`, which attacks every TAM/SAM/SOM assumption (is the segment count real and cited? is the price/ACV grounded? does the SAM→SOM capture rate survive scrutiny?) and flags each as sound / shaky / unsupported; plus `trend-critic`, which challenges each trend's tailwind-vs-headwind call for THIS hypothesis over two years with an explicit counter-scenario. Each writes `/out/child/<name>/critique.md`. Same background loop, ≤8 in flight.

4. **COMPILE** (single slow-tier `sandbox_agent`, `name=compiler`). Dispatch only after every PRESSURE-TEST job is terminal. It reads `/workspace/hypothesis.md` and all findings + critiques at `/in/previous-jobs/*/` and synthesises `/out/child/compiler/landscape.md` (three sections, structure below) plus `appendices/appendix-<n>-<avenue>.md` (each avenue's finding + its critique, verbatim). It integrates each tier's threat case beside that tier's map, presents the sizing model with per-assumption confidence flags from `sizing-critic`, and issues the final tailwind/headwind verdict per trend incorporating `trend-critic`'s counter-scenarios. Conflicting evidence is flagged with a verification action, not silently resolved.

5. **DELIVER** (orchestrator's own process). `cp -r /out/child/compiler/. /out/` and write `/out/metadata.json` (hypothesis title, tiers/avenues, tiers used, run date). Do **not** delegate this copy to a `sandbox_script` child — that child writes only to its own `/out/child/<name>/` and strands the report.

**Fix cap:** if the compiler marks any section under-evidenced, the orchestrator may re-dispatch that single research avenue once with a sharpened prompt, then re-run COMPILE — **cap two research rounds total**, no more.

## Writing the orchestrator prompt

Brief it as a complete document; terse prompts produce shallow landscapes.

1. **The hypothesis** — WHO/HOW OFTEN/HOW SEVERE/WHAT-THEY-DO-TODAY, product concept, target market and geography, known competitors. Mount `/in/hypothesis/` if the founder ran `sandbox-hypothesis-stress-test`.
2. **The four tiers** (definitions the research and threat children use verbatim): **direct** — solving the same problem the same way; **indirect** — solving the same problem a different way (including "spreadsheet + email" and in-house builds); **potential acquirers** — incumbents who would buy in rather than build; **adjacent players who could move in** — companies one product decision away from the space. Each tier's threat case must counter *competitor neglect*: argue the genuine threat, not the version easiest to dismiss.
3. **Sizing method** — TAM/SAM/SOM built bottom-up from cited public data (segment population × realistic ACV), each layer showing its arithmetic and sources; classify market direction as **expanding / consolidating / mature** with evidence; map the buyer landscape — who holds **budget**, who **influences** the decision, and whether they are the **same person** (the last is a go-to-market fact, not a footnote).
4. **Trends** — exactly three external trends, one each **regulatory / technological / demographic**, each judged **tailwind or headwind for THIS hypothesis over the next two years** with a reason; plus analogous-market extraction — a market where a similar problem was solved, what worked, what didn't, and what transfers.
5. **Citation discipline** — every capability, competitor, and sizing figure carries URL + access date + quoted excerpt; "do not infer from general reputation — cite or omit." Note publication dates; flag sources older than ~12 months in fast-moving categories.
6. **Pressure-test framing** — the threat/sizing/trend critics are fresh adversaries; their job is to make the strongest opposing case, and the compiler must preserve unresolved disagreement as a flagged verification action rather than averaging it away.

## Output contract

```
/out/
  landscape.md                        # The deliverable
  metadata.json                       # hypothesis, tiers/avenues, tiers used, run date
  appendices/
    appendix-a-tier-direct.md         # finding + critique, verbatim
    appendix-b-tier-indirect.md
    appendix-c-tier-acquirers.md
    appendix-d-tier-adjacent.md
    appendix-e-sizing.md
    appendix-f-trends.md
    appendix-g-analogous.md
```

`landscape.md` sections in order: **TL;DR** (≤400 words; the sharpest threat, the SOM number with its confidence, and the timing verdict; written last, placed first), **Methodology** (avenues run, sources per avenue, known limitations, any avenue excluded and why), **A. Competitor Map** (the four tiers, each as a table of players + positioning followed by its steelmanned "why this tier genuinely threatens us" case), **B. Market Sizing** (TAM/SAM/SOM with per-layer arithmetic and sources, each assumption tagged sound/shaky/unsupported by `sizing-critic`; market direction; buyer landscape), **C. Trends & Timing** (the three trends each tagged tailwind/headwind over two years with counter-scenario, plus the analogous-market lesson and the overall timing read), **Appendix Index**.

## Launching the orchestrator

- **`directories:`** — mount `/in/hypothesis/` (the `sandbox-hypothesis-stress-test` output) if it exists; optional but it sharpens the threat cases. No founder-private data is required — this is open-web research — so the pipeline runs with the prompt alone if nothing is mounted.
- **Tiers** — **slow** for the orchestrator and the compiler; **medium** for the research and pressure-test children (the orchestrator sets this when spawning them).
- **Child naming** — DNS-1123 labels: lowercase letters, digits, interior hyphens, ≤40 chars (`research-tier-direct`, `threat-adjacent`, `sizing-critic`, `compiler` — never `Research_Tier_Direct` or `tier.direct`). Bad names produce invalid volume names and break sibling mounts silently.
- Tell the orchestrator to background-dispatch both fan-out stages and poll `sandbox_wait` (≤8 in flight); a blocking child does not return until it finishes, so blocking dispatch serialises the fan-out and defeats the whole design.

## Host-side landing

There is no code landing — the deliverable is `/out/landscape.md`. Read it, share it, act on it: the tier threat cases feed which competitor to watch, the SOM-with-confidence feeds the build/no-build call, and the timing verdict feeds *when*. Its natural downstream is customer discovery — hand the sharpest threat and the timing read into `sandbox-interview-kit-design`.
