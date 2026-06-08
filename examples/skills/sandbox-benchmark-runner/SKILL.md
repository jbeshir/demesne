---
name: sandbox-benchmark-runner
description: Orchestrate a parameter sweep — run the same benchmark or experiment under N configurations, collect structured metrics, and produce a ranked comparative report. Apply when the user wants to sweep hyperparameters, compare algorithm variants, evaluate model sizes or prompt variants, or profile infra configurations against a stated objective metric. Triggers include "run a parameter sweep", "benchmark these configs", "hyperparameter search", "compare these variants", "grid search", "evaluate which config wins", "sweep learning rate / batch size / model", "rank these configurations by latency / accuracy / throughput". Skip for single-run profiling (call `sandbox_script` directly), open-web competitive research (use sandbox-product-research), and code correctness validation (use sandbox-feature-work).
allowed-tools: Read, Glob, Grep, Bash, Write, Edit, mcp__demesne__sandbox_agent, mcp__demesne__sandbox_script
---

Run a parameter sweep through a demesne pipeline: the host launches a slow-tier orchestrator that fans out `sandbox_script` children — one per configuration — then a slow-tier synthesiser ranks results and writes the report. The deliverable is `/out/REPORT.md` + `/out/results.jsonl`; there is no code landing.

**Watch out:** Every experiment run is a `sandbox_script`, not a `sandbox_agent` — using an agent for runs is slower and non-deterministic. Never silently drop a failed run; dropped failures bias rankings by hiding configurations that are hard to run. The orchestrator must `cp` synthesiser output into its own `/out` — do not delegate this copy to a child.

## Procedure

1. **Design.** Orchestrator writes `/workspace/design.md`: the objective metric (unit; whether lower or higher is better), secondary metrics to capture, the parameter grid or sampling rule, the exact per-run shell command, the expected wall-clock budget per run, and the dataset/code mounts needed. Record the image, egress mode, and data fingerprint here — results are only reproducible under the same sandbox environment.

2. **Build the parameter grid.** Orchestrator writes `/workspace/runs.jsonl`, one record per line:
   ```json
   {"id": "run-000", "params": {...}, "command": "python train.py --lr 0.01 --batch 32", "expected_duration_s": 120}
   ```
   Run IDs must be zero-padded (`run-000`, `run-001`) to match child naming. For grid search: Cartesian product of all axes. For Bayesian/randomised search: pick the next batch from `/workspace/results.jsonl` (accumulated prior results) and append to `runs.jsonl`; cap the loop at **10 iterations AND a maximum total run count AND a wall-clock budget** — without all three the orchestrator can loop indefinitely.

3. **Execute.** Spawn one `sandbox_script` per configuration (`name=run-NNN`, DNS-1123: lowercase, digits, interior hyphens, ≤40 chars — a bad name produces an invalid volume name and poisons sibling spawns), batched ≤4 concurrent (recommended, not a demesne-enforced cap; use fewer when runs are memory-heavy or you want fair per-run CPU allocation). Each child runs one configuration and writes:
   ```json
   {
     "run_id": "run-000",
     "params": {"lr": 0.01, "batch_size": 32},
     "objective": 0.923,
     "secondary": {"train_loss": 0.041, "walltime_s": 118},
     "exit_code": 0,
     "stderr_tail": ""
   }
   ```
   to its `/out/metrics.json` (i.e., `/out/child/run-NNN/metrics.json`). Each run writes to its own `/out` rather than a shared `/workspace` path — concurrent writes to the same path collide without locking. Image: `anaconda` for ML/scientific sweeps (sklearn, torch, lightgbm pre-available); `python` for slim Python; `go` or `node` for engineering benchmarks. Egress: `none` seals the sandbox — use with a pre-installed image and a pinned `requirements.txt` for reproducible benchmarks; `package-managers` allows installs but introduces version non-determinism across retries. Each `sandbox_script` run inherits the orchestrator's read-only `/in` mounts (at `/in/<name>`), sibling reads at `/in/previous-jobs/<name>` (the mount point registers when the child is created; files appear once that sibling completes), and the shared `/workspace`; it has no `files:`/`directories:` parameters of its own — mount datasets/code into the orchestrator at launch. (Only `sandbox_research` runs isolated, with no `/in` and a fresh private `/workspace`.)

4. **Retry.** After each batch, check every child's exit code and read its `/out/child/run-NNN/metrics.json`. A run is failed if: exit code is nonzero, `metrics.json` is absent, or `metrics.json` is present but missing the `objective` field or is not valid JSON. On failure: re-dispatch as `run-NNN-r1`, then `run-NNN-r2` (up to 2 retries). After 2 retries mark `"status": "failed"` in `results.jsonl`; include the run in the report with a `FAILED` status row in `RANKED.md` and its stderr tail in `REPORT.md`. Never silently drop — dropped failures bias rankings.

5. **Collect.** After all batches complete (including retries), read each `/out/child/run-NNN/metrics.json` (or the successful retry's output) and append to `/workspace/results.jsonl`, adding `"status": "ok"` or `"status": "failed"`. Do not collect incrementally during dispatch — wait for the batch to settle, then harvest.

6. **Rank and report.** Spawn one slow-tier `sandbox_agent` named `synthesiser` that reads `/workspace/results.jsonl` and writes to its `/out`:
   - `RANKED.md` — top-K configurations by objective, a column per secondary metric, and confidence intervals where ≥3 repeated seeds are available (point estimates only if seeds are absent — the synthesiser must not fabricate intervals).
   - `REPORT.md` — narrative: what won and by what margin, objective-vs-secondary trade-offs, recommended configuration with rationale, axes to sweep next, and all failed runs with their stderr tails.
   - `results.jsonl` — full results mirrored for downstream use.

   This runner verifies that runs completed and `metrics.json` is well-formed; it does not validate that your benchmark measures what you intend — that is the user's responsibility.

7. **Deliver.** In the orchestrator's own process: `cp -a /out/child/synthesiser/. /out/`. Do not delegate this copy to a child — a child writes only to `/out/child/<name>/`, stranding the files there. The orchestrator's `/workspace` is torn down on exit; only `/out` persists to the host.

## Writing the orchestrator prompt

Brief it as a complete document:

1. **The objective** — metric to maximise or minimise, its unit, and whether lower or higher is better.
2. **The parameter grid** — explicit axes and values (grid search), or sampling budget and prior (Bayesian; include the cap: 10 iterations, 4 runs each = 40 total max).
3. **The per-run command** — exact shell command, required environment variables, dataset/code paths available in the sandbox.
4. **Mount contract** — `sandbox_script` has no `files:`/`directories:` of its own; mount datasets/code into the orchestrator via `directories:`/`files:` at launch (runs inherit at `/in/<name>`) or stage into `/workspace`.
5. **The pipeline contract** — the seven steps above: runs are `sandbox_script` not `sandbox_agent`, batches ≤4 concurrent, retry policy (2×, mark failed, never drop), synthesiser is the only `sandbox_agent` child.
6. **The metrics.json schema** — embed the exact schema; a run that does not write it is treated as failed.
7. **Image and egress** — `anaconda`/`python`/`go`/`node` + `none`/`package-managers`; prefer `anaconda` + `none` with pinned deps for reproducible benchmarks. If unsure, prefer `anaconda` + `package-managers`; it is slower to start but avoids re-installing the scientific stack.
8. **The deliver step** — plain `cp` from synthesiser's `/out/child/synthesiser/` into the orchestrator's own `/out`; this is the orchestrator's job, not a child's.

## Output contract

```
/out/
  RANKED.md          # Top-K configurations by objective, with secondary metrics
  REPORT.md          # Narrative analysis, trade-offs, recommended config, axes to sweep next, failed runs
  results.jsonl      # Full per-run results
```

Intermediate workspace artefacts (not surfaced to host):
```
/workspace/
  design.md          # Step 1 output
  runs.jsonl         # Step 2 output (parameter grid)
  results.jsonl      # Step 5 output (accumulated metrics)
```

## Launching the orchestrator

- **`directories: ["<abs path>"]`** — required if runs need data or a codebase. Mounts into the orchestrator; every `sandbox_script` run inherits it read-only at `/in/<name>`. Forgetting it leaves runs with nothing to read.
- **`files:`** — for individual scripts or config files, likewise inherited by runs.
- Tier: **slow** for the orchestrator and synthesiser. Medium is not recommended for the orchestrator — it will under-specify retry logic and batch structure.
- Child naming: `run-000`, `run-001`, … for runs; `run-000-r1`, `run-000-r2` for retries; `synthesiser` for the report agent. All DNS-1123: lowercase, digits, interior hyphens, ≤40 chars.