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
| [`sandbox-appearance-review`](sandbox-appearance-review/) | Render a front-end into a screenshot matrix, fan out one visual-review lens per agent, merge into tiered appearance-improvement proposals. |
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

## The Founder's Playbook skills

These skills implement the activities in Anthropic's *The Founder's Playbook: Building an AI-Native Startup* — one ready-to-invoke pipeline per activity (a few closely-related activities are merged), spanning the playbook's four stages: Idea, MVP, Launch, Scale. `FOUNDER-PLAYBOOK.md` at the repo root chains them in invocation order with each stage's exit criteria; `COVERAGE.md` maps every numbered playbook activity to its skill. Skills whose real-world deliverable touches interactively-authenticated services (email send, calendar writes, recurring schedules) are **hybrid**: the sandboxed pipeline produces structured drafts and the skill's Host-side finishing section says what to do with them using the host's own connected tools.

**Idea stage** — validate the problem before building.

| Skill | What it does |
|-------|--------------|
| [`sandbox-hypothesis-stress-test`](sandbox-hypothesis-stress-test/) | Sharpen a vague observation into a testable hypothesis, then adversarially hunt disconfirming evidence. |
| [`sandbox-market-landscape`](sandbox-market-landscape/) | Tiered competitor map with per-tier threat cases + pressure-tested TAM/SAM/SOM + 2-year trend tailwind/headwind analysis. |
| [`sandbox-competitor-complaint-mining`](sandbox-competitor-complaint-mining/) | Mine competitor customer reviews for top unresolved complaints; score the hypothesis against each. |
| [`sandbox-interview-kit-design`](sandbox-interview-kit-design/) | Target profile + reachability map + per-persona interview questions, adversarially audited for leading framing. |
| [`sandbox-outreach-pipeline`](sandbox-outreach-pipeline/) | Hybrid: prospect research, personalized outreach drafts, follow-up cadence, tracking sheet; host sends. |
| [`sandbox-interview-synthesis`](sandbox-interview-synthesis/) | Per-interview debriefs + every-5-interviews confirmation-bias audit over a corpus of notes. |
| [`sandbox-solution-concept-pressure-test`](sandbox-solution-concept-pressure-test/) | Develop the solution concept, attack it from four angles, isolate the 3 load-bearing assumptions. |
| [`sandbox-prototype-sprint`](sandbox-prototype-sprint/) | Build the single-core-interaction prototype on a branch + a 5-person reaction-test kit. |

**MVP stage** — build the smallest thing that generates real PMF evidence.

| Skill | What it does |
|-------|--------------|
| [`sandbox-mvp-architecture-charter`](sandbox-mvp-architecture-charter/) | Pre-build architecture decisions with their WHY, as a CLAUDE.md-ready charter + session template/log. |
| [`sandbox-mvp-scope-guardrail`](sandbox-mvp-scope-guardrail/) | Scope doc (does / deliberately does-not / amendment criteria); amendment mode pressure-tests new feature ideas. |
| [`sandbox-prelaunch-security-review`](sandbox-prelaunch-security-review/) | Pre-launch audit on auth/session, data exposure, input validation, vulnerable deps; human-review flags. |
| [`sandbox-metrics-framework`](sandbox-metrics-framework/) | Metrics that matter, benchmark targets, and explicit false-positive signatures — defined before launch. |
| [`sandbox-feedback-loop-ops`](sandbox-feedback-loop-ops/) | Hybrid: recurring feedback intake → triage → weekly raw synthesis with a mandatory human-review gate. |
| [`sandbox-pmf-diagnostic`](sandbox-pmf-diagnostic/) | Sean Ellis test + push-vs-pull founder-effort test with an adversarially-checked verdict. |
| [`sandbox-pivot-diagnostic`](sandbox-pivot-diagnostic/) | Stalled-PMF diagnostic: segment divergence, positioning-vs-product, realism; adjust / pivot / return-to-idea. |

**Launch stage** — repeatable growth, hardened product, founder out of every loop.

| Skill | What it does |
|-------|--------------|
| [`sandbox-tech-debt-audit`](sandbox-tech-debt-audit/) | Tech-debt audit with sprint-sequenced remediation plan + architecture-decision backfill into CLAUDE.md. |
| [`sandbox-compliance-review`](sandbox-compliance-review/) | Code-level review mapped to SOC 2 / GDPR / HIPAA controls; remediation sequence + buyer-facing controls workstream. |
| [`sandbox-founder-load-audit`](sandbox-founder-load-audit/) | Founder operational-load inventory → automate / delegate / founder-only triage → workflow-logic designs. |
| [`sandbox-product-ops-system`](sandbox-product-ops-system/) | Hybrid: sprint cadence, minimum spec template, bug-triage tree, weekly metrics brief; host wires the schedule. |

**Scale stage** — systematic growth, enterprise readiness, a defensible moat.

| Skill | What it does |
|-------|--------------|
| [`sandbox-bottleneck-stress-test`](sandbox-bottleneck-stress-test/) | Founder-bottleneck map + one-week-absence stress test; fix designs per failure point. |
| [`sandbox-enterprise-procurement-pack`](sandbox-enterprise-procurement-pack/) | Hybrid: procurement doc pack (docs, playbooks, SLAs, questionnaire library) + drift-detection/observability plan. |
| [`sandbox-enterprise-gap-analysis`](sandbox-enterprise-gap-analysis/) | Per named target account: what procurement demands vs what exists; sequenced gap-closure plan. |
| [`sandbox-gtm-engine`](sandbox-gtm-engine/) | Per-audience GTM foundation (users, investors, enterprise buyers, analysts) + tactical execution layer designs. |
| [`sandbox-domain-knowledge-codify`](sandbox-domain-knowledge-codify/) | Convert domain expertise into a searchable context library + codified, replay-verified workflow routines. |
| [`sandbox-moat-test-suite`](sandbox-moat-test-suite/) | Turn observed vertical edge cases into a growing scenario-test suite that maps the moat. |
| [`sandbox-data-flywheel-audit`](sandbox-data-flywheel-audit/) | Interaction-data audit, usage→improvement loop design, adversarially-tested one-page moat narrative. |
| [`sandbox-lockin-audit`](sandbox-lockin-audit/) | Per-customer integration/switching-cost profiles → lock-in patterns → integration build backlog. |

**Meta**

| Skill | What it does |
|-------|--------------|
| [`sandbox-playbook-skill-forge`](sandbox-playbook-skill-forge/) | The pipeline that produced this family: turn any prescriptive playbook into a landed library of skills, one per activity, each design→critique→refine gated. |

## Adapting a skill

Treat each `SKILL.md` as a template, not a fixed recipe. The frontmatter `description` is the trigger signal your agent matches against; the body is the contract the orchestrator follows. Tune the in-flight concurrency cap, egress modes, images, and quarantine policy to your task — the constraints each skill calls out are the parts to keep. The background-dispatch fan-out loop (above) is the one mechanism to leave intact: blocking children do not run concurrently.
