---
name: sandbox-solution-concept-pressure-test
status: alpha
description: Take a validated problem plus its discovery evidence, develop the solution concept into a crisp written form, then attack it from four genuinely distinct adversarial angles — unsolved gaps, alternatives a rational buyer would pick instead, what must be true at scale, and claims with no support in the evidence — isolate the THREE assumptions the design depends on most heavily (each with what would have to be true, the earliest cheap test, and the blast radius if it fails), and issue an evidence-cited verdict of proceed-to-prototype / revise-concept / return-to-discovery. Apply when the problem is validated and the founder asks "pressure-test my solution concept", "what are the three assumptions this depends on", "is this ready to prototype", "attack this design from every angle", "what would a rational buyer do instead". Skip when the problem hypothesis itself is still unvalidated (use sandbox-hypothesis-stress-test), when interview notes still need synthesising into evidence (use sandbox-interview-synthesis), when you need the competitor/market research itself (use sandbox-market-landscape), when the verdict is already PROCEED and you want the prototype built (use sandbox-prototype-sprint), or for a general non-concept decision between named options (use sandbox-debate-decision).
allowed-tools: Read, Glob, Grep, Bash, Write, Edit, mcp__demesne__sandbox_agent, mcp__demesne__sandbox_research
---

Pressure-test a solution concept before any prototype is built. You author one orchestrator prompt, launch a single slow-tier `sandbox_agent`, and that orchestrator develops the concept from the mounted discovery evidence, fans out four adversarial attackers with distinct priors, runs a fresh assumption analyst that isolates the three load-bearing assumptions, then a fresh judge that issues the verdict. The deliverable is `/out/verdict.md` plus the concept, assumption ledger, and attack transcripts; no code landing. The playbook framing this skill enforces: a prototype is easy to mistake for evidence — this pipeline is the pressure-testing that must happen first, and its verdict gates whether a prototype is even warranted.

**Watch out (cross-cutting):** Ungrounded attackers are the silent failure mode — an attacker that doesn't cite the mounted evidence produces plausible-sounding generic critique that survives to the verdict; require every finding to carry a file citation or be marked `SPECULATION`. The assumption analyst and judge must each be fresh contexts — the concept drafter ranking its own assumptions, or an attacker judging, converges on self-approval. The analyst must be spawned only after every attacker job is terminal (`sandbox_wait` on all attacker `job_id`s first): `/in/previous-jobs/attack-*/` mounts register at create but files appear only when that sibling finishes. The orchestrator `cp`s all deliverables into its own `/out/` itself — a `sandbox_script` child would strand them under `/out/child/<name>/`.

## Procedure

1. **INVENTORY.** The orchestrator copies the mounted discovery evidence into `/workspace/evidence/` and writes `/workspace/manifest.md`: every file, its apparent type (interview synthesis, hypothesis doc, complaint-mining report, landscape, raw notes), and a log of unparseable files — logged, never silently skipped; real founder corpora are messy. If the evidence mount is missing or empty, halt and report — attacking a concept with no evidence produces speculation dressed as a verdict.

2. **DEVELOP.** Dispatch two children with `background: true`, collect `job_id`s, poll `sandbox_wait` until both terminal (blocking calls are issued one per turn and run sequentially — never use them for fan-out):
   - `concept-drafter` (medium tier, `sandbox_agent`): reads `/workspace/evidence/` and the founder's concept sketch if one was mounted; writes `/out/concept.md` with the locked structure: **Target user** (the validated profile, verbatim from evidence), **Core value mechanism** (the one thing it does that the status quo can't), **Workflow fit** (where it enters the user's existing day), **Deliberately not doing**, **Scale story** (how it works at 1,000 users, not 10), **Why now**. Every section cites the evidence file it rests on.
   - `research-alternatives` (medium tier, `sandbox_research` — open egress, but a FRESH private workspace and NO `/in` mounts: it cannot read `/workspace/evidence/`, so embed the problem hypothesis and target profile verbatim in its prompt): researches what these buyers actually use today — named competitors, incumbent tools, spreadsheet-and-email workarounds — and writes `/out/alternatives.md` with URLs. Skip this child only if a `sandbox-market-landscape` report is already in the evidence mount.

   Harvest: `cp /out/child/concept-drafter/concept.md /workspace/concept.md` and `cp /out/child/research-alternatives/alternatives.md /workspace/alternatives.md` before the attack fan-out — attackers read the shared `/workspace` copies.

3. **ATTACK.** Dispatch all four attackers (medium tier) with `background: true` and poll `sandbox_wait` until all terminal — four fit the ≤8 in-flight window. Inject each prior verbatim via the `preamble` parameter (demesne's role-setter; priors buried in `prompt` become ignorable suggestions). Attackers are spawned simultaneously and do not see each other — independent attacks, by design. Each reads `/workspace/concept.md`, `/workspace/evidence/`, `/workspace/alternatives.md` and writes `/out/attack.md`: numbered findings, each with a one-sentence statement, severity (`CONCEPT-BREAKING` / `MAJOR` / `MINOR`), an evidence citation (file + quote) or the literal tag `SPECULATION`, and **the assumption this exposes** — the unstated thing the concept needs to be true for this finding not to kill it.

   | Child | Prior (genuinely distinct — identical priors produce an echo chamber) |
   |-------|------|
   | `attack-gaps` | Assumes the concept under-solves the validated problem; hunts mismatches between what the discovery evidence says users suffer and what the concept actually delivers. |
   | `attack-alternatives` | Plays the rational buyer with a budget and a deadline; for each alternative in `alternatives.md` (including "do nothing" and "build it in-house"), argues the strongest honest case for picking it over this concept. |
   | `attack-scale` | Assumes what works for ten hand-held users breaks at a thousand; interrogates unit economics, support load, technical ceilings, and distribution — what would have to be true at scale. |
   | `attack-evidence` | The confirmation-bias auditor; flags every claim in `concept.md` with no supporting record in `/workspace/evidence/`, citing the absence — the concept must address the problem validation revealed, not the one the founder assumed. |

4. **ISOLATE.** Barrier: only after every attacker job is terminal, dispatch `assumption-analyst` (slow tier, `background: true`, poll `sandbox_wait`). It reads `/workspace/concept.md` and all `/in/previous-jobs/attack-*/attack.md`, merges the attackers' exposed assumptions with its own extraction into a deduplicated ledger, scores each on **load** = (how much of the concept collapses if it's false) × (how untested it is in the evidence), and cuts to exactly THREE. For each of the three it writes: **what would have to be true** for the assumption to hold, **the earliest cheap test** — executable within a week without building product (an interview question, a landing-page test, a pricing conversation, a concierge run) so it feeds straight back into customer discovery or the prototype sprint's reaction test — and **the consequence if it fails**: which concept sections die, and whether that failure means revise or return to discovery. Output `/out/assumptions.md`, with the full ledger below the cut so the top-three selection is auditable.

5. **VERDICT.** After the analyst completes, dispatch `judge` (slow tier, fresh context — never the drafter, an attacker, or the analyst). It reads concept, all attacks, and assumptions via `/in/previous-jobs/`, discards `SPECULATION`-tagged findings from verdict weight (they may appear as open questions only), and applies this decision rule verbatim: **PROCEED-TO-PROTOTYPE** — no `CONCEPT-BREAKING` finding survives and each of the three load-bearing assumptions has at least some direct supporting evidence in the corpus; the cheap tests refine, they don't establish existence. **REVISE-CONCEPT** — `CONCEPT-BREAKING`/`MAJOR` findings exist but are repairable design choices while the validated problem still holds; the verdict must list each specific revision and the finding it addresses. **RETURN-TO-DISCOVERY** — any load-bearing assumption has zero support or is contradicted in the evidence: the gap is in validation, not design. Severity disagreements between attackers are preserved in a Dissent section, never averaged away. Writes `/out/verdict.md`.

6. **DELIVER.** In the orchestrator's own process — never delegated to a child — copy `verdict.md`, `concept.md`, `assumptions.md`, each `attack.md`, and `alternatives.md` (if produced) into `/out/` per the contract below, and write `/out/metadata.json` (concept title, evidence files used, roles, tiers, run date).

## Writing the orchestrator prompt

Brief it as a complete document:

1. **The validated problem hypothesis** verbatim, in the playbook's shape — WHO exactly, HOW OFTEN, HOW SEVERE, WHAT THEY DO TODAY — plus where in the evidence mount its validation lives.
2. **The concept seed** — the founder's sketch if one exists, or an instruction to develop the concept purely from evidence — and the six-section `concept.md` structure from step 2 verbatim.
3. **The four attacker priors** from the table verbatim, each to be passed as that child's `preamble`. State that priors must stay genuinely distinct and that softening an attack to be agreeable is a failure.
4. **The finding schema** — statement, severity scale, citation-or-`SPECULATION`, assumption exposed — required in every attacker's `output_format`.
5. **The load formula and the rule of THREE** — exactly three load-bearing assumptions, cheap tests executable within a week without building product, blast radius stated per assumption. More than three dilutes focus; fewer hides load.
6. **The verdict decision rule** from step 5 verbatim, including the three-way verdict vocabulary and the dissent-preservation requirement.
7. **The pipeline contract** — background dispatch + `sandbox_wait` for every fan-out (blocking children are issued one per turn and run sequentially), the analyst-after-all-attackers barrier, DNS-1123 child names, ≤8 in flight, orchestrator-performed final copy.
8. **The output contract** below, verbatim.

Terse prompts produce shallow attacks. Over-specify the problem and the evidence map; never hint at which verdict you want.

## Output contract

```
/out/
  verdict.md            # The deliverable
  concept.md            # The developed concept, as attacked
  assumptions.md        # The three load-bearing assumptions + full auditable ledger
  alternatives.md       # Research child's findings (absent if landscape was mounted)
  attacks/
    attack-gaps.md
    attack-alternatives.md
    attack-scale.md
    attack-evidence.md
  metadata.json         # concept title, evidence files used, roles, tiers, run date
```

`verdict.md` sections in order: **Verdict** (one of PROCEED-TO-PROTOTYPE / REVISE-CONCEPT / RETURN-TO-DISCOVERY, one paragraph, written last, placed first), **The Three Load-Bearing Assumptions** (per assumption: what must be true / earliest cheap test / blast radius), **Attack Summary** (surviving findings per angle with severity), **Required Revisions** (non-empty iff REVISE-CONCEPT; each revision names the finding it addresses), **Evidence Trail** (every verdict-supporting claim mapped to an evidence file or attack finding), **Dissent** (unresolved severity disagreements between attackers, verbatim).

## Launching the orchestrator

- **`directories:`** — the discovery-evidence directory is **mandatory**: interview-synthesis output, the stress-tested hypothesis, complaint-mining and landscape reports if they exist, raw notes in whatever format the founder has. If it's forgotten, the orchestrator must halt at step 1 and report — running anyway yields attackers speculating and an `attack-evidence` child with nothing to audit. **`files:`** — the founder's concept sketch, optional.
- **Tiers**: slow for the orchestrator, `assumption-analyst`, and `judge`; medium for `concept-drafter`, `research-alternatives`, and all four attackers.
- **Child names**: DNS-1123 labels — lowercase letters, digits, interior hyphens, ≤40 chars, unique within the run (`concept-drafter`, `attack-gaps`, `assumption-analyst`, `judge`; never `Attack_Gaps` or `judge.v2` — invalid names break sibling mounts silently).

## Host-side landing

No code lands — read `/out/verdict.md` and act on it. On REVISE-CONCEPT, apply the Required Revisions and re-run this skill with the revised concept mounted as the sketch; there is deliberately no in-run fix loop (cap: zero), because a pipeline that revises and re-attacks its own revision drifts toward self-approval. On PROCEED-TO-PROTOTYPE, hand `concept.md` and the three cheap tests to `sandbox-prototype-sprint`. On RETURN-TO-DISCOVERY, the failed assumption's cheap test is the next interview question — take it back through `sandbox-interview-kit-design`.
