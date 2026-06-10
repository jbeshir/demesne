# Develop demesne skills

A "skill" here means a reusable instruction your top-level agent can invoke to drive a demesne pipeline. The principles below apply whether you encode the recipe as a Claude Code skill, a CLI wrapper, or a paragraph you keep around — there's no coupling to a specific skill format.

## Design the request

Before writing any tool calls, decide:

- What does the user's request map to? (One agent? A script? A fan-out?)
- Which tools does the orchestrator call, and which does each worker call?
- What is the final artefact, and where does it land?

Don't fan out where one well-prompted `sandbox_agent` call would do. Fan-out adds latency, cost, and coordination complexity — only reach for it when tasks are genuinely independent or when a fresh context per task materially improves quality.

## Compose phases

Split work into phases: research → plan → implement → verify. Each phase is a separate `sandbox_agent`, `sandbox_script`, or `sandbox_research` call against the same `/workspace`. Benefits:

- A fresh context per phase is cheaper than letting one window grow unbounded.
- Each phase can be retried independently if it fails.
- Phases compose naturally with the verifier/judge pattern.

## Plan and enforce the handoff

Decide what each phase produces, where, and in what format — then follow that contract in every phase:

| Phase output | Location |
|---|---|
| Plan / in-progress findings | `/workspace/<phase>.md` — shared scratch, visible to all siblings |
| Final artefacts | `/out/<name>` — the orchestrator's output dir; child results are copied here explicitly |
| Sibling outputs | `/in/previous-jobs/<name>/` — read-only mount of a completed sibling |

Spell the contract in the skill definition. A phase that writes to an undeclared path is likely to be silently lost.

## Verifier/judge patterns

A separate agent with a fresh context and no stake in the work catches errors the producer would rationalise away. There are two distinct uses — pick deliberately:

**Implementer–verifier cycle** — gate one produced artefact on correctness. The implementer writes its output; a verifier reads it (at `/in/previous-jobs/<worker-name>/`) and returns `PASS`/`FAIL` against explicit criteria; on `FAIL` a fix phase runs and the verifier re-checks. Cap the rounds (e.g. two attempts before escalating) — an uncapped loop on a hard task burns tokens. Use when there's a single thing to get right.

**Verifier as a result filter** — raise the precision of a *set* of results. A producer over-generates candidate findings (a survey's suspected bugs, a review's flagged issues); a verifier examines each — often one verifier per finding, prompted to *refute* it — and you keep only the ones that survive, dropping false positives. There's no fix loop: you're filtering the producer's output, not iterating it. Use when a false positive in the final report costs more than an extra check.

See [Spawning a verifier/judge child](../reference/nested-sandboxes.md#spawning-a-verifierjudge-child) for the tool-call shape.

## Effort calibration

Match the tool to the task:

- **Deterministic checks** (lint, tests, file comparisons) → `sandbox_script`. Fast, cheap, no LLM needed.
- **Classification or routing** → `sandbox_agent` with `model: "haiku"`. Lightweight reasoning.
- **General agentic work** → `sandbox_agent` with `model: "sonnet"` (default). Most tasks.
- **Hard synthesis or extended reasoning** → `sandbox_agent` with `model: "opus"`. Use sparingly.
- **The hardest synthesis** → `sandbox_agent` with `model: "fable"`. The most capable tier, above opus; reserve for tasks where opus is not enough.

Try a single well-prompted agent before fanning out. Three parallel workers each burning a sonnet context is only worthwhile if the tasks are genuinely independent and the quality gain justifies the cost.

## Checkpointing

For long pipelines, each phase writes its findings to `/workspace/<phase>.md` before it ends, and a fresh next-phase agent reads the checkpoint to pick up where the last one left off. Writing partial findings to `/out` early means progress survives interruption, and you can read intermediate results without waiting for the full pipeline to finish.

## Wiring it into your agent

Encode the recipe wherever your agent reads its skills or instructions: a Claude Code subagent definition, a slash command, or a saved paragraph in a system prompt. No hard coupling to any specific repo is needed — the recipe is just a prompt that describes the phase structure, the handoff contract, and the success criteria.

---

For the full layout and conventions of nested runs, see [Nested sandboxes reference](../reference/nested-sandboxes.md).
