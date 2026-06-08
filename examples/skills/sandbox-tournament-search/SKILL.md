---
name: sandbox-tournament-search
description: Explore the design space for a problem with multiple plausible approaches — generate N candidate solutions in parallel across distinct thinking frames, score them against an explicit rubric, prune the weak, refine the survivors, and re-score to pick a winner. Apply when approach choice is genuinely uncertain and picking wrong is costly: algorithm design, API shape, schema design, prompt design, headline copy, naming, public-statement framing. Triggers include "explore alternatives", "compare approaches", "tournament search", "score candidates against rubric", "tree of thoughts", "which design wins", "generate options and rank them", "find the best design". Skip when the best approach is already decided (use sandbox-feature-work to build it), when the task is researching what comparables do (use sandbox-product-research), or when the problem has an obvious single answer.
allowed-tools: Read, Glob, Grep, Bash, Write, Edit, mcp__demesne__sandbox_agent
---

Explore a design space with a two-round tournament: generate N candidates in parallel across distinct thinking frames, score them against an explicit rubric, prune the weak, refine the survivors, and re-score to pick a winner. You author one orchestrator prompt; a single slow-tier `sandbox_agent` runs the tournament autonomously. The deliverable is a markdown winner with a full search trace — not deployed code, not a committed branch.

**Watch out:** Each judge must be a different context from the generators — never ask a generator to score its own or another's work. Within each batch, all generators must be spawned in a single tool-call message; a `for` loop within a batch serialises what should be parallel.

## Procedure

1. **Frame.** Write `/workspace/problem.md`: the question, success criteria, hard constraints, soft preferences, and an explicit evaluation rubric — 3–6 named criteria, each with a 1–10 scale definition and a statement of which direction is better. A judge without a rubric invents criteria per-run, making scores incomparable across rounds and breaking the search signal.

2. **Round 1: Generate.** Spawn N=5–8 medium-tier `sandbox_agent` children in parallel. Name each with a DNS-1123 label — lowercase letters, digits, interior hyphens, ≤40 chars (`gen-greedy`, `gen-dp`; not `gen_dp`, `Round1Gen`; invalid names produce bad volume names and poison sibling mounts). Each child receives the full problem statement in its `prompt` and a distinct thinking frame in its `preamble`. Use `preamble` for frames, not `prompt` — `preamble` is the whole-session role-setter seen before the task; frames buried in `prompt` become suggestions the generator can ignore.

   A strong frame is concrete and operational, e.g. for algorithms: "You approach every problem through a dynamic-programming lens: identify the optimal substructure, define the recurrence first, and fall back to a simpler method only if DP cannot apply." For non-algorithm domains, frame by design philosophy: REST-first / event-driven / minimal / ergonomics-first. Vague single-word frames ("creative", "simple") produce weak diversity.

   Each child writes exactly `/out/candidate.md`. Parallel generators are safe — each has its own private `/out`. The orchestrator-level collision risk is different: if the orchestrator also writes to `/workspace` while children are running, stagger those writes. Cap N at 8; beyond 8, frames produce near-duplicate candidates.

   If N>4, spawn in two sequential batches: all generators in batch 1 in one tool-call message, wait for completion, then all generators in batch 2 in one tool-call message. Four concurrent is a recommended batch size; demesne enforces no cap, but larger batches increase MCP keepalive pressure on nested sandboxes.

3. **Round 1: Judge.** After all generators complete, spawn a single medium-tier `sandbox_agent` named `judge-r1`. It reads `/in/previous-jobs/gen-<frame>/candidate.md` for each generator and the rubric, then writes `/out/scores.jsonl`:
   ```
   {"candidate":"gen-dp","frame":"dp","scores":{"correctness":9,"simplicity":5,"edge_cases":8},"total":22,"rationale":"...","rank":1}
   ```
   Spawn the judge only after all generators finish. The `/in/previous-jobs/<name>` mount registers when the sibling sandbox is created, but the file inside it doesn't exist until the generator completes; a judge spawned concurrently with the last generator risks reading an incomplete candidate. Enforce the schema via the judge's `output_format` parameter — free-form prose forces unreliable natural-language parsing by the orchestrator.

4. **Prune.** Parse `judge-r1`'s `scores.jsonl`, retain the top K=2–3 candidates by total score, and write `/workspace/elim-round1.md` listing each eliminated candidate, its score, and a one-line reason. Use K=2 for a tight run; K=3 when the top tier is clustered within 2–3 points. Cap K at 3.

5. **Round 2: Expand.** Spawn K medium-tier `sandbox_agent` children in a single tool-call message. Each child's `preamble` includes the same thinking frame as Round 1 plus the judge's `rationale` for its candidate. Its `prompt` embeds the surviving candidate and instructs it to address identified weaknesses without discarding strengths. Each child writes `/out/refined.md`.

6. **Round 2: Judge.** After all refiners complete, spawn a single medium-tier `sandbox_agent` named `judge-r2` using the same rubric and schema as Round 1. It reads `/in/previous-jobs/refine-<frame>/refined.md` for each survivor and writes `/out/scores.jsonl`. Pick the highest-scoring entry as winner. If the problem needs deeper search, checkpoint the Round 2 winner to `/workspace` and spawn a separate follow-up tournament — a third round adds as much work as the first two combined for diminishing returns.

7. **Deliver.** Run `cp` in the orchestrator's own session — do not delegate to a child. A `sandbox_agent` child writes to `/out/child/<name>/`, not the orchestrator's `/out`; routing the copy through a child strands the deliverable invisibly:
   - `cp /out/child/refine-<winner>/refined.md /out/WINNER.md`
   - `cp /out/child/judge-r1/scores.jsonl /out/scores-r1.jsonl`
   - `cp /out/child/judge-r2/scores.jsonl /out/scores-r2.jsonl`
   - Write `/out/trace.md` — both rounds' full elimination log.
   - Write `/out/REPORT.md` — rubric, search tree with per-round scores, final ranking, and a one-paragraph narrative on why the winner won.

   This skill is **REPORT-ONLY**: do not commit code, push branches, or apply the winning design. The host decides whether and how to use it.

## Writing the orchestrator prompt

Brief it as a complete document:

1. **The problem statement** — what is being decided and why multiple approaches are plausible. Concrete problems produce distinguishable candidates.
2. **The frame list** — all N frame names with a one-sentence description each. Provide these explicitly; do not leave frame selection to the orchestrator.
3. **The rubric** — 3–6 criteria with explicit 1–10 scale definitions and direction of "better". Embed it; do not ask the orchestrator to design it.
4. **The pipeline contract** — the seven steps above; DNS-1123 child names, ≤40 chars; each batch spawned in one tool-call message; judges spawned after their inputs complete.
5. **The scores.jsonl schema** verbatim — require it as `output_format` for both judge prompts.
6. **The prune threshold** — K=2 for tight runs, K=3 when candidates are clustered.
7. **"REPORT-ONLY"** — do not commit code, push branches, or apply the winning design.
8. **The output contract** below — include it verbatim.

## Output contract

```
/out/
  WINNER.md           # The winning refined candidate
  REPORT.md           # Rubric, search tree with per-round scores, final narrative
  trace.md            # Full elimination log for both rounds
  scores-r1.jsonl     # Round 1 judge scores (one JSON object per line)
  scores-r2.jsonl     # Round 2 judge scores (one JSON object per line)
```

## Launching the orchestrator

- `directories:` — mount any relevant background material (existing designs, specs, constraints). Optional; the problem statement can live entirely in the prompt.
- Tier: **slow** for the orchestrator; **medium** for all generator, refiner, and judge children — the orchestrator sets these when spawning.
- Child names: DNS-1123 labels — `gen-greedy`, `gen-dp`, `judge-r1`, `refine-graph`, `judge-r2`. No underscores, no uppercase.