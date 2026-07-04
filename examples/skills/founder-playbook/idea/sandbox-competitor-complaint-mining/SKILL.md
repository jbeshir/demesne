---
name: sandbox-competitor-complaint-mining
status: alpha
description: Mine competitors' customer reviews for the top complaints existing solutions have NOT resolved, then score the founder's problem hypothesis against each one. An orchestrator locks a per-complaint extraction schema, then EITHER map-reduces a mounted corpus of exported reviews OR fans out `sandbox_research` children that gather review evidence per competitor across G2/Capterra-style sites, app stores, Reddit/HN, and support forums (each complaint carrying a URL + verbatim quote); a reducer clusters and ranks the unresolved complaints, a fresh auditor refute-checks every top citation, and a compiler writes a report whose centrepiece scores whether the hypothesis addresses each top complaint. Apply when the user wants "free qualitative research on competitors' customers" — "what do competitors' customers complain about", "mine G2/Capterra/app-store reviews for [competitor]", "top unresolved complaints in [category]", "does my idea address real complaints about [competitor]", "synthesize competitor customer feedback for problem-solution fit". Skip when you want competitor tiering, TAM/SAM/SOM, or trend tailwinds (use sandbox-market-landscape — it deliberately leaves complaint mining to this skill), when hunting disconfirming evidence against your OWN hypothesis rather than competitors' reviews (use sandbox-hypothesis-stress-test), for a generic same-op-over-any-corpus with no review/hypothesis framing (use sandbox-corpus-map-reduce), or for broad feature/gap competitive analysis not centred on customer complaints (use sandbox-product-research).
allowed-tools: Read, Glob, Grep, Bash, Write, Edit, mcp__demesne__sandbox_agent, mcp__demesne__sandbox_research, mcp__demesne__sandbox_script, mcp__demesne__sandbox_wait, mcp__demesne__sandbox_cancel
---

Mine competitors' customer reviews for their top *unresolved* complaints and test the founder's hypothesis against them. You (the host session) author one orchestrator prompt, launch a single slow-tier `sandbox_agent`, and it runs the whole pipeline autonomously. The deliverable is markdown + data in `/out`; there is no code landing. This is the Idea-stage exercise of getting free qualitative research on competitors' customers — a complaint the incumbents haven't fixed and the hypothesis addresses is a problem-solution-fit signal.

**Watch out (cross-cutting):** Three failure modes silently ruin this run. (1) **Fabricated evidence** — `sandbox_research` children will invent plausible-sounding complaints; every complaint record needs a `source_url` + verbatim `quote`, and the auditor (a `sandbox_agent` with no web egress) consistency-filters each citation — a plausibility/coherence check, not live re-fetch — dropping the unsupported ones. (2) **Loud ≠ unresolved** — a heavily-upvoted rant the vendor fixed in a later release is not an unresolved complaint; `resolved_status` is judged from evidence, never inferred from volume. (3) **Generous matching** — the compiler must justify every "hypothesis addresses this" with the specific hypothesis clause and cited complaint records, not a vibe.

## Procedure

1. **INTAKE (orchestrator's own process).** Write `/workspace/target.md`: the founder's hypothesis **verbatim** (the final stage scores against its exact wording), the competitor list, and the review sources of interest. Write `/workspace/schema.md` with the locked per-complaint JSONL schema (below) — finalise it before any child runs; divergent keys produce unmerge-able records. Then pick the mode:
   - **Mode A (corpus).** A directory of exported reviews is mounted at `/in/<reviews>` (CSV/JSON/xlsx/txt dumps from review sites, app stores, support tickets — messy real-world exports). List it → `/workspace/manifest.jsonl` (`{path,size_bytes,type}`), noting unidentifiable items.
   - **Mode B (research).** Only competitor names are given. No corpus; the web is the source.
   (Both may run — a mounted corpus as primary evidence plus a research top-up — but default to one.)

2. **GATHER — fan out (background dispatch, ≤8 in flight; blocking calls issued one per turn run sequentially, so `background:true` is what runs siblings concurrently).**
   - **Mode A pre-process (deterministic → `sandbox_script`, `image=python`, `egress=package-managers`, `name=normalize`):** parse the messy exports into normalised JSONL and stage per-shard chunks under `/workspace/shards/<NN>/`. Log every unparseable file rather than dropping it silently. Then **MAP** one medium-tier `sandbox_agent` per shard (`name=map-01`, `map-02`, …): each reads its shard + the full `/workspace/schema.md`, emits `/out/extracted.jsonl` (one record per complaint, schema-locked) and `/out/log.md` (items skipped, parse errors) — a child that silently skips items fakes completeness.
   - **Mode B research (open web → `sandbox_research`):** one medium-tier child per competitor (`name=reviews-acme`, `reviews-globex`, …). `sandbox_research` children have open egress but a FRESH private workspace and NO `/in` or `/workspace` mounts — embed the competitor name, the four source-type list, and the full schema verbatim in the prompt. Each must cover **≥3 distinct source types** (G2/Capterra-style review sites, app stores, Reddit/HN threads, vendor support forums; add Trustpilot/others), a tool-call budget of 12–25, and emit for each complaint a real `source_url` + verbatim `quote` — **omit any complaint it cannot cite**. Output: `/out/extracted.jsonl` + `/out/log.md`. For a competitor with heavy volume, split into per-source-type children (`reviews-acme-g2`, `reviews-acme-appstore`) to stay inside budget.

   **Locked schema** (every record, both modes):
   ```json
   {"complaint_id":"acme-001","competitor":"Acme","source":"g2|capterra|app-store|reddit|hn|support-forum|trustpilot|other",
    "source_url":"https://…","quote":"verbatim excerpt","quote_date":"YYYY-MM|unknown",
    "complaint_theme":"kebab-case-label","frequency_signal":"e.g. 14 reviews mention this / recurring across 3 threads",
    "resolved_status":"unresolved|vendor-responded|fixed-in-later-version|unclear","severity_note":"impact on the reviewer"}
   ```
   In Mode A, `source_url` may be `corpus:<file>#<row>` when the export has no live URL. `complaint_theme` labels must be reused consistently — the reducer clusters on them.

3. **REDUCE (slow-tier `sandbox_agent`, `name=reducer`) — barrier: dispatch only after every gather job is terminal** (`sandbox_wait` on all `job_id`s first; sibling files appear at `/in/previous-jobs/<name>/extracted.jsonl` only once that sibling completes — spawn early and shards go missing). It concatenates all `extracted.jsonl` into `/workspace/all.jsonl` (flagging, not dropping, schema-divergent records), clusters records by `complaint_theme` **across competitors**, and ranks the top ~10–12 **unresolved** complaints by frequency × severity × un-resolvedness (records whose `resolved_status` is `fixed-in-later-version` or `vendor-responded` are down-weighted, not counted as open). Writes `/workspace/ranked.jsonl` (top complaints with their supporting `complaint_id`s + representative quote) and `/out/data.jsonl` (all records). Uncited or single-source complaints are flagged low-confidence here.

4. **AUDIT (medium-tier `sandbox_agent`, `name=auditor`, fresh context — verifier as result filter, no fix loop, one pass).** Reads `/workspace/ranked.jsonl` directly (the auditor shares `/workspace`, where the reducer wrote it). For each top complaint it refute-checks: does the cited `quote` plausibly exist at `source_url` (Mode B) / in the corpus record (Mode A), and is `resolved_status` justified by the evidence rather than by volume? Marks each `verified | flagged | dropped` with a reason → `/out/audited.jsonl`. It is not the producer of any evidence, so it has no stake in keeping complaints.

5. **COMPILE (slow-tier `sandbox_agent`, `name=compiler`, fresh context) — after the auditor is terminal.** Reads `/workspace/target.md` (the verbatim hypothesis) and `/in/previous-jobs/auditor/audited.jsonl` (verified/flagged complaints only; dropped ones excluded). Writes `/out/REPORT.md` whose centrepiece scores the hypothesis against each top complaint (rubric below). Every "addresses" claim cites the hypothesis clause + the complaint's `complaint_id`s.

6. **DELIVER (orchestrator's own process).** `cp` `REPORT.md` from `/out/child/compiler/`, `data.jsonl` from `/out/child/reducer/`, and the audited ranked list from `/out/child/auditor/audited.jsonl` (rename to `ranked.jsonl`) into the orchestrator's own `/out`. Each file comes from the child that produced it — the compiler writes only `REPORT.md`. Do **not** delegate this copy to a `sandbox_script` child — its `/out` is `/out/child/<name>/` and the files would strand. Also write `/out/target.md` and `/out/SUMMARY.md` (mode, competitors covered, source-type mix, records gathered/skipped/dropped, children spawned). Print `DONE`.

## Writing the orchestrator prompt

Brief it as a complete document:

1. **The hypothesis (verbatim) and competitors** — paste the founder's exact hypothesis string, the competitor names, and which review sources matter. State the mode; if `/in/<reviews>` is mounted, name it.
2. **The locked schema** — the JSON block above, reproduced in full. Emphasise consistent `complaint_theme` labels and mandatory `source_url` + `quote`.
3. **Gather discipline** — Mode A: `normalize` script logs unparseable files; map children write `extracted.jsonl` + `log.md`, never skip silently. Mode B: research children cover ≥3 source types, cite URL + verbatim quote per complaint, **omit uncitable complaints**, and run in a fresh workspace so everything they need is in the prompt.
4. **The four source types** to require in Mode B: G2/Capterra-style review sites, app stores, Reddit/HN, vendor support forums (+ Trustpilot/others). Under-covering a type biases the ranking.
5. **Reduce + audit rule** — reducer barrier after all gather jobs; cluster on theme, rank by frequency × severity × un-resolvedness, down-weight resolved records; auditor is a fresh refute-only filter (verified/flagged/dropped), no fix loop.
6. **Hypothesis-match rubric** (the deliverable's point). For each top unresolved complaint: `addresses = directly | partially | does-not | orthogonal`; `mechanism` (HOW the hypothesised solution would resolve it, or why it wouldn't); `evidence` (the hypothesis clause + supporting `complaint_id`s); `confidence`. Then an overall verdict: how many of the top-N unresolved complaints the hypothesis directly addresses → the problem-solution-fit signal, with the explicit caveat that addressing complaints is one signal, not validation certainty.
7. **Background dispatch** — gather children with `background:true`, collect `job_id`s, poll `sandbox_wait` (`timeout_seconds:120`), ≤8 in flight; reducer/auditor/compiler are single sequential children, not fan-out.
8. **Output contract** — the files below; report-only, no edits or commits.

## Output contract

```
/out/
  REPORT.md          # the deliverable; hypothesis-match is the centrepiece
  data.jsonl         # every complaint record, schema-locked (from the reducer, pre-audit)
  ranked.jsonl       # top unresolved complaints, post-audit (auditor's audited.jsonl: verified/flagged/dropped verdicts)
  target.md          # the hypothesis (verbatim) + competitor list as run
  SUMMARY.md         # mode, competitors, source mix, records gathered/skipped/dropped, children
```

`REPORT.md` sections in order: **TL;DR** (how many of the top-N unresolved complaints the hypothesis addresses + the headline verdict; written last, placed first), **Methodology** (mode, competitors, source types covered, records gathered/skipped, audit drops, source-imbalance flags), **Top Unresolved Complaints** (ranked table: rank, theme, competitors affected, frequency signal, resolved_status, representative quote + citation), **Hypothesis Match** (the per-complaint rubric scoring — the centrepiece), **Coverage & Anomalies** (thin-evidence competitors, single-source or flagged complaints, dropped citations).

## Launching the orchestrator

- **Mode A: `directories: ["<abs path to reviews export dir>"]` is mandatory.** Forget it and every map child wakes with no reviews and the pipeline produces nothing. Mode B needs no mount — research children fetch the web themselves.
- Tiers: **slow** for orchestrator, reducer, compiler; **medium** for map/research gather children and the auditor; the Mode A `normalize` step is a `sandbox_script` (`image=python`), not an LLM.
- Child names (DNS-1123: lowercase letters, digits, interior hyphens, ≤40 chars): `normalize`, `map-01`, `map-02`, `reviews-acme`, `reviews-globex`, `reviews-acme-g2`, `reducer`, `auditor`, `compiler`.
- Tell the orchestrator to dispatch gather children with `background:true` and poll `sandbox_wait` (≤8 in flight) — blocking calls issued one per turn run sequentially, defeating the parallel fan-out.

There is no code landing: the deliverable is `/out/REPORT.md`. Read it, and if the hypothesis addresses one or more top unresolved complaints, carry that signal into customer discovery.
