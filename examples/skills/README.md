# demesne example skills

> **Status: pre-alpha.** These skills are in principle ready to use, but most are untested in practice. Regard them as examples of what could be tried — starting points to adapt — rather than hardened, battle-tested tools. Expect rough edges, and read a skill before you run it.

These are ready-to-use skill definitions that drive demesne orchestration pipelines. Each is a self-contained directory holding a single `SKILL.md` — YAML frontmatter (a `name`, and a `description` that acts as the trigger signal your agent matches the request against) followed by the pipeline instructions. They are written for a host agent session with the demesne MCP server connected, and the orchestration is agent-agnostic: any agent that can load a `SKILL.md` and call demesne's MCP tools can run them.

To use one, make its directory visible wherever your agent discovers skills. Symlinking it in keeps this repo as the single source of truth — a `git pull` then updates the skill in place, and you can track which skills you've enabled by listing the links:

```bash
ln -s "$(pwd)/sandbox-feature-work" <your-agent-skill-dir>/sandbox-feature-work
```

Substitute `<your-agent-skill-dir>` for whatever location your agent loads skills from.

Every skill follows the same shape: the host authors one prompt, launches a single slow-tier `sandbox_agent` orchestrator, and that orchestrator fans the work out across child sandboxes (`sandbox_agent`, `sandbox_research`, `sandbox_script`). The orchestrator's children read each other's completed output at `/in/previous-jobs/<name>/` and surface results by copying them into `/out`. Skills name model **tiers** — slow, medium, fast — rather than specific models, so they run on whichever agent you've configured; the host maps each tier to a concrete model when it launches the run. See [the nested-sandboxes reference](../../docs/reference/nested-sandboxes.md) and [Develop demesne skills](../../docs/how-to/develop-demesne-skills.md) for the mechanics these skills are built on.

## Concurrent fan-out

Skills that fan work out across sibling children dispatch them **in the background**, not with blocking calls. The observed behaviour is that an orchestrator agent issues its child calls **one per turn** — it will not emit several as parallel tool calls in a single message, even when explicitly instructed to (this was tested directly, and it holds regardless of how forcefully the prompt asks). A blocking `sandbox_agent`/`sandbox_research`/`sandbox_script` call does not return until its child has finished, so blocking children issued one per turn run strictly one after another — sequential fan-out, however the prompt is written. Passing `background: true` returns immediately with `{job_id, status: "running"}` while the child runs detached, so the orchestrator dispatches the next child right away and they run at the same time; that is the only way to get siblings running concurrently. It also sidesteps the ~240s client tool-call timeout a long blocking child would trip.

The canonical fan-out loop every parallel stage uses:

1. **Dispatch** each child with `background: true`, collecting its `job_id`. Keep at most **8 in flight** — a host-resource guard, not an MCP limit (demesne enforces no cap). For N ≤ 8 dispatch all N; for N > 8 dispatch 8 and launch one more each time a job finishes (a rolling window).
2. **Poll** each `job_id` with `sandbox_wait` (`timeout_seconds: 120`), re-calling any still `running` until every job reaches a terminal state (`succeeded`/`failed`/`cancelled`); `sandbox_cancel` kills a stuck job and its subtree.
3. **Harvest** each child's output from `/out/child/<name>/` (siblings read a completed peer at `/in/previous-jobs/<name>/`).

Barriers still hold where a stage genuinely needs every prior result — a reducer over all map outputs, a judge over all candidates, debate round N+1 over round N: drain the whole in-flight set before dispatching the next stage. Steps that are **sequential by construction** — shared-`/workspace/repo` edit phases, a bisect probe loop — do not fan out at all and keep their blocking calls.

## The skills

**Build / land code on a branch** — the orchestrator commits to a branch in `/out/repo`; the host lands it with `git fetch` + ff-merge.

| Skill | What it does |
|-------|--------------|
| [`sandbox-feature-work`](sandbox-feature-work/) | One substantial change: research → plan → numbered phases → in-sandbox `make validate` gate → review/fix → branch. |
| [`sandbox-migration-sweep`](sandbox-migration-sweep/) | One specified edit applied to N similar files in parallel, each in its own git worktree, per-shard verify, failures quarantined. |
| [`sandbox-test-gen-from-spec`](sandbox-test-gen-from-spec/) | Backfill tests for existing undertested code; per-unit writers gated on coverage delta, tautologies dropped. |
| [`sandbox-quality-improvement`](sandbox-quality-improvement/) | Audit-and-fix loop against a deterministic gate. |

**Survey / map-reduce over a corpus or codebase** — report-only (or a structured store).

| Skill | What it does |
|-------|--------------|
| [`sandbox-code-defect-survey`](sandbox-code-defect-survey/) | Research a defect taxonomy, fan out one detector per type across the code, synthesise. |
| [`sandbox-prose-defect-survey`](sandbox-prose-defect-survey/) | The prose twin of the code survey — documentation, comments, and generated text. |
| [`sandbox-docs-quality`](sandbox-docs-quality/) | Map a fixed set of documentation-quality lenses over the docs tree. |
| [`sandbox-corpus-map-reduce`](sandbox-corpus-map-reduce/) | Apply the same extraction/scoring op to every item in a corpus, then reduce to a ranked answer. |
| [`sandbox-etl-document`](sandbox-etl-document/) | Parse → extract → classify → validate → load unstructured documents into a structured store, with a quarantine pile. |

**Explore a question / decision** — multiple attempts or perspectives on the same problem.

| Skill | What it does |
|-------|--------------|
| [`sandbox-product-research`](sandbox-product-research/) | Parallel open-web research avenues synthesised into a brief. |
| [`sandbox-tournament-search`](sandbox-tournament-search/) | Generate diverse candidates → judge → prune → refine → pick a winner (tree-of-thoughts). |
| [`sandbox-debate-decision`](sandbox-debate-decision/) | N specialist roles cross-critique a decision across rounds; a judge synthesises with dissent preserved. |
| [`sandbox-swarm-explore`](sandbox-swarm-explore/) | Many decoupled explorers with different seeds/lenses; an aggregator preserves outliers. |
**Targeted / sequential**

| Skill | What it does |
|-------|--------------|
| [`sandbox-routing-triage`](sandbox-routing-triage/) | Classify a heterogeneous batch and dispatch each item to a specialist sub-pipeline, low-confidence items quarantined. |
| [`sandbox-bisect-hunt`](sandbox-bisect-hunt/) | Binary-search the commit / file / flag / version that introduced a regression, fresh sandbox per probe. |
| [`sandbox-benchmark-runner`](sandbox-benchmark-runner/) | Sweep a parameter grid with deterministic `sandbox_script` runs, rank the configurations. |

## Adapting a skill

Treat each `SKILL.md` as a template, not a fixed recipe. The frontmatter `description` is the trigger signal your agent matches against; the body is the contract the orchestrator follows. Tune the in-flight concurrency cap, egress modes, images, and quarantine policy to your task — the constraints each skill calls out are the parts to keep. The background-dispatch fan-out loop (above) is the one mechanism to leave intact: blocking children do not run concurrently.
