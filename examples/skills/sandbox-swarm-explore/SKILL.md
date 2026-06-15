---
name: sandbox-swarm-explore
description: Drive a wide-coverage exploration through a demesne swarm — an orchestrator dispatches 15–50 fully isolated children (sandbox_script sweeps or sandbox_agent brainstorming) with no inter-child coordination, then an aggregator harvests consensus, outliers, ranked candidates, and a methodology report. Apply when the value is diversity of starting points rather than one coordinated plan — parameter sweeps, random seeds, Monte Carlo, fuzzing, hypothesis-space traversal, N-angle brainstorming, scenario generation. Triggers include "explore the space", "try N random seeds", "brainstorm N angles", "parameter sweep", "fuzz this", "run a swarm". Skip for coordinated multi-avenue research (use sandbox-product-research), fixed-taxonomy defect detection (use sandbox-code-defect-survey), fix-and-land quality work (use sandbox-quality-improvement), and feature builds (use sandbox-feature-work). Report-only.
allowed-tools: Read, Glob, Grep, Bash, Write, Edit, mcp__demesne__sandbox_agent, mcp__demesne__sandbox_research, mcp__demesne__sandbox_script
---

Drive a broad exploration through a demesne swarm. You (the host) author one orchestrator prompt, launch a single slow-tier `sandbox_agent`, and that orchestrator fans out 15–50 isolated children — each with a different seed, parameter, or lens — then a slow-tier aggregator synthesises what they found. The deliverable is four markdown reports in `/out`; there is no code landing.

**Watch out:** The orchestrator must `cp` final artefacts to its own `/out` directly — a `sandbox_script` child doing this copy writes to `/out/child/<name>/`, stranding the files. Swarm children must not read `/in/previous-jobs/` — that coordination collapses diversity into pseudo-consensus.

## Procedure

1. **SPEC.** Orchestrator writes `/workspace/spec.md` covering: the problem statement; the *dimension of variation* (what changes child-to-child — random seed, numeric parameter, scenario assumption, or brainstorming lens); per-child budget (tool-call or wall-clock limit); and the **output schema** every child writes to `/out/finding.json`. The schema is domain-specific but must include at minimum a `name` field (the child's own identifier) plus the payload fields the aggregator will interpret. The schema must name every field and type; under-specifying it produces heterogeneous JSON the aggregator cannot reliably synthesise.

2. **PARAMS.** Orchestrator writes `/workspace/params.jsonl`, one record per child: `name` (DNS-1123 label — lowercase letters, digits, interior hyphens, ≤40 chars; bad names produce invalid volume names and poison sibling spawns), `seed`/`param` (the varying quantity), and `preamble` (the child's lens or initial condition). Write the full `params.jsonl` **before spawning any child** — the parameter set must be auditable before any child is spawned. For deterministic children the preamble may be minimal; for generative children, distinct preambles are the **primary mechanism of lens diversity** and must be meaningfully distinct per child.

3. **SWARM.** Dispatch N children **in batches of ≤4 concurrent** (recommended; demesne enforces no cap, but beyond four, MCP keepalive pressure degrades stability). Spawn ≤4 in one tool-call message, wait for all to complete, then the next batch.

   Children are **fully isolated**: do not reference `/in/previous-jobs/` in swarm child prompts. Each child reads only its embedded params record and writes exclusively to its own `/out` — never to a shared `/workspace/` path (two children writing `/workspace/result.json` stomp each other). Each child produces:
   - `/out/finding.json` — required; structured per spec.md schema.
   - `/out/notes.md` — optional; reasoning log.

   **Deterministic** (`sandbox_script`): suited to fuzz testing, parameter sweeps, Monte Carlo simulation. Use `egress: "package-managers"` if the script needs to install dependencies; otherwise `egress: "none"`. Pass the seed/param via environment variable or embed it in the command string. The script **must write valid JSON to `/out/finding.json` before exiting** — stdout is captured to `/out/stdout.log` but HARVEST reads only `/out/finding.json`.

   **Generative** (`sandbox_agent`): suited to N-angle brainstorming, scenario planning, design alternatives. Each agent gets a distinct `preamble`. Use the **medium** tier. Set `output_path: "/out/finding.json"` and `output_format` matching the spec.md schema.

4. **HARVEST.** After each batch completes, read `/out/child/swarm-NNN/finding.json` for every child in that batch and append to `/workspace/findings.jsonl`. Log any child that failed to produce a valid `finding.json` by name and failure reason — never silently drop it. Missing-child metadata is meaningful to the aggregator.

5. **AGGREGATE.** Spawn one slow-tier `sandbox_agent` (`name=aggregator`). It reads `/workspace/findings.jsonl` (shared via `/workspace`) and writes four files to its own `/out`:
   - `CONSENSUS.md` — findings that appeared across multiple children, with frequency counts and the convergence threshold (e.g. ≥30% of children or ≥5 children, per spec).
   - `OUTLIERS.md` — findings from ≤2 children, absent from consensus. These are not noise; they are the swarm's minority signal. The aggregator must argue for or against each one — a finding from one child that explored a region the majority never reached is often the most valuable output. Prompt it: "outliers that survive scrutiny must be preserved, not discarded."
   - `RANKED.md` — top-K candidates by a stated scoring rubric (novelty, internal consistency, support count, estimated impact, or domain-specific criteria from spec.md).
   - `REPORT.md` — methodology (N children, dimension of variation, flavour, batch count, failures), exploration coverage statement, quality read.

6. **DELIVER.** In the orchestrator's own process: `cp -a /out/child/aggregator/. /out/` and `cp /workspace/findings.jsonl /out/findings.jsonl`. Do not route this through a `sandbox_script` child — that child writes to `/out/child/<copy-child-name>/`, stranding the files. Only the orchestrator's `/out` persists to the host.

## Writing the orchestrator prompt

Brief it as a complete document:

1. **The problem and exploration goal** — what space is being explored. Specificity produces a better spec.md and params.jsonl.
2. **The dimension of variation** — what changes across children: random seed, numeric parameter, scenario assumption, or brainstorming lens. Determines the params.jsonl schema and which flavour to use.
3. **N, the child count** — 15–50. Larger N improves coverage and sharpens outlier-vs-noise distinctions, but adds more children to run and aggregate.
4. **The output schema** — field names and types for `/out/finding.json`. The aggregator's contract; under-specifying produces JSON it cannot reliably parse.
5. **The swarm flavour** — deterministic (`sandbox_script`) or generative (`sandbox_agent`). If generative, describe what dimension of the preamble varies (role, constraint, prior belief, attack angle).
6. **The aggregator mandate** — outliers from ≤2 children that are internally consistent must appear in `OUTLIERS.md` with an argument for or against. "Majority wins" collapses the swarm's value.
7. **The pipeline contract** — the six steps above; batches of ≤4; no `/in/previous-jobs/` for swarm children; orchestrator-level copy in DELIVER.

Over-specify the contract; under-specify the solution.

## Output contract

```
/out/
  CONSENSUS.md        # Findings that converged across children, with frequency
  OUTLIERS.md         # Minority findings — preserved, argued for or against
  RANKED.md           # Top-K candidates by stated scoring rubric
  REPORT.md           # Methodology, coverage statement, quality read
  findings.jsonl      # Raw harvest, one record per child, for reproducibility
```

## Launching the orchestrator

- **`directories: ["<abs path>"]`** if swarm children need access to a repo or dataset. Optional — omit for pure generative sweeps.
- Tier: **slow** for orchestrator and aggregator; **medium** for swarm children (the orchestrator sets this).
- Explicitly say "≤4 concurrent" in the orchestrator prompt — the instinct is to fire all N at once.