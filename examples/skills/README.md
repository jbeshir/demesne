# demesne example skills

> **Status: pre-alpha.** These skills are in principle ready to use, but most are untested in practice. Regard them as examples of what could be tried — starting points to adapt — rather than hardened, battle-tested tools. Expect rough edges, and read a skill before you run it.

These are ready-to-use skill definitions that drive demesne orchestration pipelines. Each is a self-contained directory holding a single `SKILL.md` — YAML frontmatter (a `name`, a `status` maturity marker, and a `description` that acts as the trigger signal your agent matches the request against) followed by the pipeline instructions. They are written for a host agent session with the demesne MCP server connected, and the orchestration is agent-agnostic: any agent that can load a `SKILL.md` and call demesne's MCP tools can run them.

## Maturity levels

Every skill's frontmatter carries a `status:` field. It is a per-skill maturity marker, distinct from the repo-wide pre-alpha note above:

- `alpha` — written but not yet run end-to-end against real inputs; read it before you run it. **All skills here are currently `alpha`.**
- `beta` — has been run successfully at least once against real inputs; rough edges expected.
- `stable` — repeatedly run, battle-tested, safe to trust without re-reading.

## Using a skill

To use one, make its directory visible wherever your agent discovers skills. Symlinking it in keeps this repo as the single source of truth — a `git pull` then updates the skill in place, and you can track which skills you've enabled by listing the links:

```bash
ln -s "$(pwd)/build-and-land/sandbox-feature-work" <your-agent-skill-dir>/sandbox-feature-work
```

Substitute `<your-agent-skill-dir>` for whatever location your agent loads skills from, and the leading `build-and-land/` for whichever category folder (below) holds the skill you want.

Every skill follows the same shape: the host authors one prompt, launches a single slow-tier `sandbox_agent` orchestrator, and that orchestrator fans the work out across child sandboxes (`sandbox_agent`, `sandbox_research`, `sandbox_script`). The orchestrator's children read each other's completed output at `/in/previous-jobs/<name>/` and surface results by copying them into `/out`. Skills name model **tiers** — slow, medium, fast — rather than specific models, so they run on whichever agent you've configured; the host maps each tier to a concrete model when it launches the run. See [the nested-sandboxes reference](../../docs/reference/nested-sandboxes.md) and [Develop demesne skills](../../docs/how-to/develop-demesne-skills.md) for the mechanics these skills are built on.

## Concurrent fan-out

Skills that fan work out across sibling children dispatch them **in the background**, not with blocking calls. The observed behaviour is that an orchestrator agent issues its child calls **one per turn** — it will not emit several as parallel tool calls in a single message, even when explicitly instructed to (this was tested directly, and it holds regardless of how forcefully the prompt asks). A blocking `sandbox_agent`/`sandbox_research`/`sandbox_script` call does not return until its child has finished, so blocking children issued one per turn run strictly one after another — sequential fan-out, however the prompt is written. Passing `background: true` returns immediately with `{job_id, status: "running"}` while the child runs detached, so the orchestrator dispatches the next child right away and they run at the same time; that is the only way to get siblings running concurrently.

The canonical fan-out loop every parallel stage uses:

1. **Dispatch** each child with `background: true`, collecting its `job_id`. Keep at most **8 in flight** — a host-resource guard, not an MCP limit (demesne enforces no cap). For N ≤ 8 dispatch all N; for N > 8 dispatch 8 and launch one more each time a job finishes (a rolling window).
2. **Wait** on each `job_id` with `sandbox_wait` using its long default, re-calling only a result that is still `running` until every job reaches a terminal state (`succeeded`/`failed`/`cancelled`); `sandbox_cancel` kills a stuck job and its subtree.
3. **Harvest** each child's output from `/out/child/<name>/` (siblings read a completed peer at `/in/previous-jobs/<name>/`).

Barriers still hold where a stage genuinely needs every prior result — a reducer over all map outputs, a judge over all candidates, debate round N+1 over round N: drain the whole in-flight set before dispatching the next stage. Steps that are **sequential by construction** — shared-`/workspace/repo` edit phases, a bisect probe loop — do not fan out at all and keep their blocking calls.

## The skills, by category

The skills are grouped into category folders. Each folder's own `README.md` carries the detailed per-skill table.

| Category | What's in it |
|----------|--------------|
| [`build-and-land/`](build-and-land/README.md) | Land a substantial code change on a branch — feature work, migration sweeps, test backfill, quality passes against a deterministic gate. |
| [`creative-works/`](creative-works/README.md) | Build a durable, bespoke creative artifact to experience, not a codebase change — currently coded real-time TypeScript games and branching Twine interactive fiction. |
| [`survey-and-map-reduce/`](survey-and-map-reduce/README.md) | Report-only surveys and map-reduce over a corpus or codebase — defect surveys, docs/appearance review, ETL into a structured store. |
| [`explore-and-decide/`](explore-and-decide/README.md) | Multiple attempts or perspectives on one question — open-web research, tournament search, multi-role debate, decoupled swarm exploration. |
| [`targeted-and-sequential/`](targeted-and-sequential/README.md) | Targeted, sequential pipelines — classify-and-route a batch, bisect a regression, sweep a benchmark parameter grid. |
| [`founder-playbook/`](founder-playbook/README.md) | The founder skill family, including pre-playbook problem discovery, the 29 playbook activities staged Idea → MVP → Launch → Scale, and a `meta/` skill-forge. See its [`README.md`](founder-playbook/README.md) for chaining and [`COVERAGE.md`](founder-playbook/COVERAGE.md) for the activity map. |

## Adapting a skill

Treat each `SKILL.md` as a template, not a fixed recipe. The frontmatter `description` is the trigger signal your agent matches against; the body is the contract the orchestrator follows. Tune the in-flight concurrency cap, egress modes, images, and quarantine policy to your task — the constraints each skill calls out are the parts to keep. The background-dispatch fan-out loop (above) is the one mechanism to leave intact: blocking children do not run concurrently.
