---
name: sandbox-hypothesis-stress-test
description: Sharpen a vague founder observation into a specific, testable problem hypothesis, then adversarially hunt disconfirming evidence for it — an orchestrator runs a capped sharpen→audit loop that fills the four slots WHO/HOW-OFTEN/HOW-SEVERE/WHAT-THEY-DO-TODAY (each tagged evidence vs assumption), then fans out open-web `sandbox_research` children each tasked to find ONLY negative evidence (negative market signals, failed competitors who tried this, user behaviour that contradicts the premise, structural adoption obstacles), and a fresh compiler synthesises the strongest counter-case plus a "what would change my mind" discovery-test list. Apply at the Idea stage before customer interviews, when the request is "is this problem real", "sharpen my idea into a hypothesis", "turn this observation into something testable", "argue against my idea", "find disconfirming evidence", "stress-test my problem statement", "steelman the case against building this". Skip when you want a full competitor map / TAM / trend read (use sandbox-market-landscape), to mine competitor reviews for complaints (use sandbox-competitor-complaint-mining), to attack a SOLUTION concept rather than a problem premise (use sandbox-solution-concept-pressure-test), or to build the interview questions this feeds (use sandbox-interview-kit-design).
allowed-tools: Read, Glob, Grep, Bash, Write, Edit, mcp__demesne__sandbox_agent, mcp__demesne__sandbox_research, mcp__demesne__sandbox_script
---

Turn a fuzzy problem observation into a hypothesis sharp enough to disprove, then point the pipeline *against* it. You (the host session) author one orchestrator prompt and launch a single slow-tier `sandbox_agent`; that orchestrator runs a sequential sharpen→audit loop to produce the hypothesis, then fans out parallel open-web disconfirmation hunters, then a fresh compiler assembles the counter-case and the discovery-test list. Deliverables are markdown in `/out` — `hypothesis.md`, `counter-case.md`, `discovery-tests.md`. There is no code landing; the output feeds customer discovery, it does not decide build/no-build.

**Watch out (cross-cutting):** The whole point is the confirmation-bias antidote from the playbook — AI finds supporting evidence for whatever direction you point it, so this pipeline points the other way; a hunter that hedges with reassuring evidence, or a compiler that softens findings into "but it's probably fine", has silently defeated the run. `sandbox_research` children have open egress but a FRESH private workspace with NO `/in` mounts and cannot read `/workspace/hypothesis.md` — embed the full finalised hypothesis text in each hunter's prompt or it researches nothing. The orchestrator must `cp` every deliverable into its own `/out/` itself; delegating that copy to a `sandbox_script` child strands the files under `/out/child/<name>/`.

## Procedure

1. **INTAKE** (orchestrator's own process). Write `/workspace/observation.md`: the founder's raw observation verbatim, any mounted context (notes, prior research, the founder's stated assumptions), and the four-slot target shape with the calibration example (below) so every downstream child calibrates to the same sharpness bar. If a founder-notes directory is mounted at `/in/<dir>`, read it and quote relevant lines; log anything unparseable rather than dropping it.

2. **SHARPEN** — sequential, *not* a fan-out (draft→audit→redraft is a loop; blocking children issued one per turn is correct here). Spawn one medium-tier `sandbox_agent` (`name=sharpener`) that reads `/workspace/observation.md` and writes `/out/hypothesis-draft.md`: each of the four slots filled — **WHO** exactly (job title, company type, team shape — not "companies"), **HOW OFTEN** the pain recurs, **HOW SEVERE** it is (time/money/risk, quantified), **WHAT THEY DO TODAY** (the current workaround it displaces) — and each slot tagged `[EVIDENCE]` (grounded in a mounted fact) or `[ASSUMPTION]` (the founder's guess).

3. **SHARPNESS AUDIT** — verifier separation: a fresh medium-tier `sandbox_agent` (`name=sharpness-auditor`), never the sharpener grading itself. It reads the draft at `/in/previous-jobs/sharpener/hypothesis-draft.md` and scores each slot specific-and-testable vs still-vague against the calibration bar, writing `/out/audit.md` with a per-slot verdict and a concrete fix for each failure. On any FAIL the orchestrator relays the fixes into a `sharpener-r2` blocking round. **Cap at 2 sharpen→audit rounds**; slots still failing after round 2 are recorded verbatim as `OPEN QUESTION` in the hypothesis — never fabricated into false specificity. The orchestrator then assembles final `/workspace/hypothesis.md` and `/out/hypothesis.md` in its own process from the passing slots plus open questions.

4. **DECOMPOSE** (orchestrator). Write `/workspace/disconfirm-avenues.json` — one avenue per playbook disconfirmation category, each a `research_question` that seeks *only* negative evidence: `negative-market-signals` (declining demand, shrinking budgets, adjacent products that died), `failed-competitors` (who tried this exact thing and folded or pivoted, and why), `contradicting-behaviour` (what the target user actually does — revealed preference — that contradicts the premise), `structural-obstacles` (procurement, regulatory, switching-cost, incentive misalignments that block adoption even if the pain is real), and optionally `good-enough-substitute` (why the status quo is tolerated because the pain sits below the switching threshold). Target 4–5 avenues; each object: `{name, disconfirmation_target, research_question, source_types, tool_call_budget, output_format}`.

5. **HUNT** — parallel fan-out via background dispatch. Dispatch each avenue as a `sandbox_research` child with `background: true`, collect its `job_id`, and poll `sandbox_wait` (`timeout_seconds: 120`) until every job is terminal; keep **≤8 in flight** (4–5 avenues all dispatch at once). Blocking calls are issued one per turn and run strictly sequentially, so background dispatch is the only thing that makes the hunters concurrent. Name each `disconfirm-<slug>` (DNS-1123: lowercase letters, digits, interior hyphens, ≤40 chars — e.g. `disconfirm-failed-competitors`, `disconfirm-structural-obstacles`). Each child has open egress, a fresh private `/workspace`, and no `/in` mounts — embed the **full finalised hypothesis text**, the avenue's disconfirmation mandate, and the citation rule in the prompt. Prosecution-only framing, stated explicitly: the child argues the *strongest* form of the case against the hypothesis that the evidence supports (not the easiest-to-dismiss version), is forbidden from balancing with supporting evidence or reassurance, requires ≥3 distinct source types and a URL + access date + quoted excerpt per claim, and reports "no disconfirming evidence found for X" as a finding rather than backfilling comfort. Output: `/out/child/disconfirm-<slug>/finding.md`.

6. **COMPILE** — barrier: only after every HUNT job reaches a terminal state. Spawn one slow-tier `sandbox_agent` (`name=counter-compiler`; fresh context, so it never scores its own hunts). It reads `/workspace/hypothesis.md` directly (`sandbox_agent` inherits `/workspace`) and all hunter findings at `/in/previous-jobs/disconfirm-*/finding.md`, then writes into its `/out`: **`counter-case.md`** — the strongest disconfirming case organised by category, every claim carrying its URL + excerpt; uncited scare-claims are dropped, not promoted (symmetric rigor — the antidote to confirmation bias is real counter-evidence, not manufactured doubt) — and **`discovery-tests.md`** — for each `[ASSUMPTION]`-tagged slot and each premise the counter-case threatens, one concrete "what would change my mind" test: the interview observation or cheap experiment that would confirm or disconfirm it, and which way each result cuts. The compiler issues **no** build/no-build verdict; it arms open-ended customer discovery.

7. **DELIVER** — in the orchestrator's own process, `cp` `counter-case.md` and `discovery-tests.md` from `/out/child/counter-compiler/` into `/out/`, copy each raw hunter finding to `/out/hunts/disconfirm-<slug>/finding.md`, and write `/out/metadata.json` (observation title, final slots, avenues, tiers, run date). Delegating this copy to a `sandbox_script` child strands the deliverables under `/out/child/`.

## Writing the orchestrator prompt

Brief it as a complete document; terse prompts produce a shallow hypothesis and a strawman counter-case.

1. **The observation** — the founder's words verbatim, the market/context, and the assumptions the founder is aware they are making. Mount any founder notes at `/in/<dir>` and tell the orchestrator to quote them.
2. **The four-slot shape + calibration example** — embed both. Vague → sharp: *"contract review takes too long"* becomes *"in-house legal teams at mid-market companies spend 3+ days per contract-review cycle because redlines are managed across email threads rather than a single version-controlled document."* Map it: **WHO** = in-house legal teams at mid-market companies; **HOW OFTEN** = every contract-review cycle; **HOW SEVERE** = 3+ days per cycle; **WHAT THEY DO TODAY** = redlines across email threads instead of one version-controlled document. Every slot must be as concrete and falsifiable as this one, or it is `[ASSUMPTION]`/`OPEN QUESTION`.
3. **The sharpen→audit contract** — sharpener drafts, a *separate* auditor grades each slot, cap 2 rounds, unresolved slots become `OPEN QUESTION`. State that the auditor must be a fresh child.
4. **The disconfirmation mandate** — the four (or five) avenues above; every hunter finds *only* negative evidence, in its strongest form, cited; reassurance and hedging are forbidden; missing evidence is reported as missing. This is the playbook's "point it the other way" step — say so.
5. **The pipeline contract** — the seven steps; emphasise background dispatch + `sandbox_wait` for HUNT (blocking children run sequentially), the barrier before COMPILE, that research children get the hypothesis embedded in-prompt (no `/in`), and the deliver-via-own-`/out` rule.
6. **Output contract** — the three deliverables and the section orders below; the compiler issues no verdict.

## Output contract

```
/out/
  hypothesis.md          # sharpened hypothesis — the deliverable that feeds discovery
  counter-case.md        # strongest disconfirming case, cited, by category
  discovery-tests.md     # "what would change my mind" test list for interviews
  metadata.json          # observation title, final slots, avenues, tiers, run date
  hunts/
    disconfirm-negative-market-signals/finding.md
    disconfirm-failed-competitors/finding.md
    disconfirm-contradicting-behaviour/finding.md
    disconfirm-structural-obstacles/finding.md
```

`hypothesis.md` sections in order: **Hypothesis** (one falsifiable sentence, written last, placed first), **The Four Slots** (WHO / HOW OFTEN / HOW SEVERE / WHAT THEY DO TODAY, each with its `[EVIDENCE]`/`[ASSUMPTION]` tag), **Open Questions** (slots unresolved after 2 rounds), **Load-Bearing Assumptions** (the `[ASSUMPTION]` slots the whole hypothesis rests on).

`counter-case.md` sections in order: **Verdict-Free Summary** (the strongest case against, in ≤300 words; explicitly no build/no-build call), **By Category** (one subsection per avenue — negative market signals, failed competitors, contradicting behaviour, structural obstacles, substitutes — each a cited case), **Uncited/Weak Claims Dropped** (what was cut for lacking a source, for transparency), **Cross-Cutting Threats** (patterns hitting ≥2 categories).

## Launching the orchestrator

- **`directories:`/`files:`** — mount the founder's observation notes / prior research if any exist; optional but the sharpener produces a richer, more `[EVIDENCE]`-grounded draft with them. Research hunters never see these mounts (fresh private workspace) — the hypothesis reaches them only through their prompt.
- **Tiers**: slow for the orchestrator and the counter-compiler; medium for the sharpener, sharpness-auditor, and the `sandbox_research` hunters.
- **Child-naming**: lowercase letters, digits, interior hyphens, ≤40 chars, unique within the parent — `sharpener`, `sharpener-r2`, `sharpness-auditor`, `disconfirm-failed-competitors`, `counter-compiler`; never `Sharpener_R2` or `disconfirm.failed`.

## Host-side landing

No code landing. Read `hypothesis.md` and `counter-case.md` together, then walk into customer discovery with `discovery-tests.md` in hand — the goal from the playbook is to reach interviews having already stress-tested the premise against its strongest counterarguments, so the conversations are genuinely open-ended rather than confirmation-seeking. Hand the sharpened hypothesis and target profile to `sandbox-interview-kit-design` to build the question sets.
