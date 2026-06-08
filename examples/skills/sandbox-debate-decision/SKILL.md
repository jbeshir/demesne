---
name: sandbox-debate-decision
description: Run a high-stakes design decision through a structured adversarial debate — a slow-tier orchestrator frames the question and assigns N distinct roles (3–5), fans out parallel Round-1 `sandbox_agent` debaters each writing a position, then fans out parallel Round-2 debaters that cross-critique the Round-1 positions, then a single slow-tier judge synthesises a verdict with dissenting opinions explicitly preserved. Apply when a decision needs adversarial scrutiny from genuinely opposing priors: architecture choices, RFC review, vendor selection, hiring rubric, contract clause, prompt-strategy tradeoffs. Triggers include "debate this decision", "get multiple perspectives on", "stress-test this choice", "devil's advocate", "adversarial review", "what's the strongest case against", "N-of-M consensus on this". Skip for factual research (use sandbox-product-research), code changes (use sandbox-feature-work), and decisions the user has already finalised.
allowed-tools: Read, Glob, Grep, Bash, Write, Edit, mcp__demesne__sandbox_agent, mcp__demesne__sandbox_research, mcp__demesne__sandbox_script
---

Put a contested design decision through a structured adversarial debate. You author one orchestrator prompt, launch a single slow-tier `sandbox_agent`, and that orchestrator frames the decision, fans out N debaters in parallel (Round 1), fans out N cross-critiquers in parallel once Round 1 completes (Round 2), then runs a single slow-tier judge that synthesises a verdict with dissenting opinions recorded — not silenced. The deliverable is `/out/verdict.md` plus round transcripts; no code landing.

**Watch out (cross-cutting):** Round 2 must be spawned only after Round 1 fully completes — the `/in/previous-jobs/<name>/` mount registers at child create but files appear only when that sibling finishes writing; Round-2 debaters started too early read empty mounts. The orchestrator must `cp` all deliverables into its own `/out/` itself; delegating that copy to a `sandbox_script` child strands the files under `/out/child/<copy-step>/`.

## Procedure

1. **FRAME.** The orchestrator writes `/workspace/decision.md` (the question; every option under consideration — minimum two, with "do nothing" as an explicit option when the user offers only one candidate; evaluation criteria; and hard constraints debaters must treat as fixed) and `/workspace/roles.json` (an array of N role objects: `name`, `preamble`, `prior`). Roles must have genuinely distinct optimisation targets — identical priors produce an echo chamber and a false-confidence verdict. If debaters need live documentation or current pricing, spawn a `sandbox_research` child now (open egress; fresh private `/workspace`; no `/in` mounts — it cannot read `/workspace/decision.md`), write its findings into `/workspace/` before the round fan-outs start; `sandbox_agent` children have no web access.

2. **ROUND 1.** Spawn N medium-tier `sandbox_agent` children in a **single tool-call message** (all N simultaneously). Use `name=debater-<role>` (e.g. `debater-pragmatist`; lowercase letters, digits, interior hyphens only, ≤40 chars — bad names produce invalid volume names and break sibling mounts silently). Inject each role's prior verbatim from `roles.json` via the `preamble` parameter — that is the demesne mechanism for setting agent identity; a missing or empty preamble produces a generic agent regardless of the question. Each debater reads `/workspace/decision.md` and writes `/out/position.md`: proposed answer, reasoning, and per-criterion scores for each option. From the orchestrator's perspective, position files appear at `/out/child/debater-<role>/position.md`. Sequential dispatch (debaters spawned one at a time) multiplies wall-clock time by N and lets later debaters see earlier ones' outputs, losing round independence. Prefer N≤4: up to four debaters fit one independent parallel wave. For N=5, choose: spawn all five in one wave (exceeds the ≤4-concurrent recommendation — demesne enforces no cap, but keepalive pressure rises beyond four) or split 4+1 (the fifth, spawned after the first four complete, sees their `/in/previous-jobs/` positions and loses Round-1 independence). Cap at 5 roles total — beyond that, judge context fills with positions and synthesis quality degrades.

3. **ROUND 2.** After all Round-1 children complete, spawn N medium-tier `sandbox_agent` children in a **single tool-call message**. Use `name=debater-<role>-r2` — names must be distinct from Round-1 names; reusing the same name returns an error that poisons the sibling-mount chain for all subsequent spawns. Each reads all `/in/previous-jobs/debater-*/position.md` files (Round-1 siblings are completed, so their files are present) and writes `/out/position.md`: revised position, explicit cross-critique of each Round-1 disagreement, and updated per-criterion scores. Round-2 debaters do not see each other — spawned simultaneously, their sibling mounts for one another are registered but empty; this is intentional (independent revision, not a cascading live debate that could produce premature convergence). If convergence between Round-2 debaters is needed, spawn them sequentially (sacrificing parallelism and independence).

4. **JUDGE.** After all Round-2 children complete, spawn one slow-tier `sandbox_agent` (`name=judge`). It reads `/in/previous-jobs/debater-*-r2/position.md` and has access to Round-1 at `/in/previous-jobs/debater-*/position.md` for context — it reads sibling outputs directly, not from memory. The judge prompt must explicitly forbid dissent homogenisation: a verdict of "some prefer A, others B — we recommend a blend" that no role endorsed defeats the pipeline. Unresolved disagreements belong in the Dissenting Opinions section of `/out/verdict.md`, not merged away.

5. **DELIVER.** In the orchestrator's own process — not delegated to a child — `cp` the judge's verdict and each debater's round outputs into the orchestrator's own `/out/`. A `sandbox_script` child writes only to `/out/child/<name>/`, stranding the files there. Write `/out/metadata.json` (decision title, roles, models, run date).

## Default roles

The orchestrator defaults to these three when the user does not specify:

| Name | Prior |
|------|-------|
| `pragmatist` | optimises for time-to-ship and real-world execution constraints |
| `maintainer` | optimises for long-term operability and lifecycle cost |
| `adversary` | assumes Murphy's law; stress-tests proposals for failure modes |

Add `security` (attack-surface and blast-radius lens) or `economist` (budget and resource-allocation lens) for decisions with a strong safety or cost dimension.

## Writing the orchestrator prompt

The orchestrator starts cold. Treat the prompt as a complete briefing:

1. **The decision** — the specific question, every option (minimum two; include "do nothing"), and evaluation criteria (cost, risk, speed, compliance, team capability). Missing criteria cause debaters to invent their own, leaving the judge nothing to aggregate across.
2. **Hard constraints** — non-negotiable walls: regulatory requirements, fixed deadlines, skill-gaps that cannot be staffed around. Not criteria to optimise; debaters who violate them produce unacceptable positions.
3. **The roles** — specify N roles with their priors, or tell the orchestrator to select from the defaults plus any domain-specific lens. State that roles must have genuinely distinct priors.
4. **The pipeline contract** — the five steps above; emphasise the single-tool-call-message requirement for both round fan-outs, that Round 2 reads Round-1 siblings via `/in/previous-jobs/`, and that the judge must preserve dissent rather than homogenise it.
5. **Mounted context** — any RFC, design doc, constraint log, or code excerpt debaters need. Debaters with no context produce generic positions anchored only in their priors.
6. **Output contract** — `/out/verdict.md`, `/out/rounds/` (per-debater `position.md` + `transcript.jsonl`), and `/out/metadata.json`.

Terse prompts produce shallow debates. Over-specify the decision frame; under-specify which option should win.

## Output contract

```
/out/
  verdict.md                         # The deliverable
  metadata.json                      # decision title, roles, models, run date
  rounds/
    debater-pragmatist/             # Round 1
      position.md
      transcript.jsonl
    debater-maintainer/
      position.md
      transcript.jsonl
    debater-adversary/
      position.md
      transcript.jsonl
    debater-pragmatist-r2/          # Round 2
      position.md
      transcript.jsonl
    debater-maintainer-r2/
      position.md
      transcript.jsonl
    debater-adversary-r2/
      position.md
      transcript.jsonl
```

Each `rounds/` subdirectory is named for the child that produced it (matching its `/in/previous-jobs/<name>/` mount); the `-r2` suffix marks Round-2 children.

`verdict.md` sections in order: **Recommendation** (one paragraph; the synthesised answer; written last, placed first), **Criteria Scorecard** (table: option × criterion × per-role score × aggregate; show spread, not just the average — a wide spread is itself a finding), **Dissenting Opinions** (one subsection per unresolved disagreement; each with the dissenting role, its argument, and a concrete verification action to take before committing; non-optional and non-empty if any genuine disagreement persists), **Consensus Points** (what all roles agreed on regardless of final position), **Next Steps** (2–5 concrete actions given the verdict, including how to break any noted ties).

## Launching the orchestrator

- **`files:`/`directories:`** — mount any RFC, spec, constraint log, architecture diagram, or code debaters need. Optional but recommended; debaters with no context produce generic positions.
- **Model**: slow-tier for the orchestrator and the judge; medium-tier for Round-1 and Round-2 debaters (the orchestrator sets this when spawning children).
- **Child-naming rule**: lowercase letters, digits, interior hyphens only, ≤40 chars, unique within the parent (`debater-pragmatist`, `debater-maintainer-r2`, `judge` — never `Debater_Pragmatist` or `debater.r2`).
## Host-side landing

There is no code landing — the deliverable is `/out/verdict.md`. Read it, share it, act on the next steps. The Dissenting Opinions section is where the real value often sits: a unanimous verdict on a contested decision is a sign of insufficient role differentiation, not genuine consensus.
