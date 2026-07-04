---
name: sandbox-test-gen-from-spec
status: alpha
description: Generate a test suite for an existing, undertested code surface through a demesne-orchestrated pipeline — a slow-tier orchestrator baselines coverage, enumerates units ordered by lowest coverage, fans out parallel test-writer agents (one per unit shard), runs a deterministic gate that measures coverage delta per shard, triages failing/redundant/tautological tests, and lands surviving tests on a branch for the host to fetch. Apply when the user wants to grow test coverage on existing, undertested code — "write tests for this package", "add test coverage", "generate tests for this module", "our coverage is low on X", "backfill tests for existing code". Skip for tests that accompany new functionality (use sandbox-feature-work), codebase-wide quality sweeps (use sandbox-quality-improvement), and defect hunting (use sandbox-code-defect-survey).
allowed-tools: Read, Glob, Grep, Bash, Write, Edit, mcp__demesne__sandbox_agent, mcp__demesne__sandbox_script
---

Generate a test suite retroactively covering an existing, undertested code surface. A slow-tier orchestrator `sandbox_agent` fans out medium-tier test-writer agents per shard, gates coverage delta, triages failing/redundant/tautological tests, and delivers a branch at `/out/repo`. Writers characterise *existing* behaviour only — a failing test is dropped unless annotated `// expected_bug=true`, which flags a known bug for follow-up via `sandbox-feature-work`. Without this policy the pipeline lands tests that assert broken behaviour as the spec.

**Watch out (cross-cutting):** The orchestrator must `cp -a /workspace/repo /out/repo` itself — a child `sandbox_script`'s `/out` is `/out/child/<name>/` and would strand the repo. Use `sandbox_script` (not `sandbox_agent`) for BASELINE and GATE — a covering agent running only a shell command adds latency and non-determinism for no benefit.

## Procedure

1. **Stage the repo.** `cp -a /in/<repo>/. /workspace/repo`. The `-a` flag preserves `.git`; without it there are no worktrees to cut and no branch for the host to fetch.

2. **Write `/workspace/surface.md`.** Target package/module path; the coverage tool (`go test -cover`, `pytest --cov`, `jest --coverage`); current coverage baseline (or "unknown — measure first"); exclusion list (generated code, vendored dirs, proto output). Every later step reads this file.

3. **BASELINE — one `sandbox_script`.** Match the image to the project: `go`, `python`, `anaconda`, or `node`. Set `egress: package-managers` if the coverage plugin requires install (`pytest-cov`, `nyc`, `c8`); `egress: none` is fine for Go (`go test -cover` is built in). For projects with in-repo tool pinning (`go.mod` tool directives, `pyproject.toml`, Node lockfile), versions are inherited automatically — do not add `@latest` installs. Run the existing suite with coverage against `/workspace/repo`, excluding paths listed in `/workspace/surface.md`; write `/workspace/baseline-coverage.json` (`{file: {lines, covered, pct}}`).

4. **ENUMERATE.** Read `baseline-coverage.json`, filter excluded paths, and pick units ordered ascending by coverage percentage. Write `/workspace/units.jsonl` (`{file, symbol, current_pct, target_uplift, shard}`). Assign to ≤8 shards, grouping by file to minimise merge surface. Skip units at 100%.

5. **WRITE — one medium-tier `sandbox_agent` per shard.** Name each `write-shard-N` (lowercase DNS-1123: letters, digits, interior hyphens, ≤40 chars; bad names break volume names and poison sibling spawns). Dispatch each with `background: true` (collect its `job_id`) and poll with `sandbox_wait` so they run concurrently, keeping **≤8 in flight** — a host-resource guard, not a demesne-enforced cap; for more shards than that, launch a replacement as each finishes. Blocking calls are issued one per turn and run sequentially, defeating parallelism.

   Each writer begins with `git -C /workspace/repo worktree add /workspace/wt-N -b shard/N`; it works only inside `/workspace/wt-N`. Two writers editing `/workspace/repo` directly corrupt the shared index. The writer reads assigned unit files and nearby existing tests, drafts tests targeting uncovered branches from `units.jsonl` (each marked `// covers: <branch description>`), does not modify production code, annotates bug-revealing tests `// expected_bug=true` instead of patching the impl, commits new test files to `shard/N`, and writes `/workspace/write-N-summary.md`.

6. **GATE — one `sandbox_script` per shard.** Use the same image and egress as BASELINE — a mismatched image silently fails the install step and produces empty gate output. Honour in-repo pinned versions; do not add `@latest` installs. Run the test suite scoped to new test files in `/workspace/wt-N`. Write `/workspace/gate-N.json`:
   ```json
   {"shard": "N", "tests": [{"name": "TestFoo", "pass": true, "coverage_delta": 0.04}], "coverage_delta": 0.12}
   ```
   `coverage_delta` per test is its marginal contribution (coverage lost when that test is removed and the suite re-run). The top-level `coverage_delta` is the shard aggregate. TRIAGE requires per-test deltas to drop individual dead-weight tests; an aggregate-only gate forces triage to guess.

7. **TRIAGE.** Read every `gate-N.json` and the drafted test files; drop:
   - **(a) Failing** — `pass: false`, unless annotated `// expected_bug=true`.
   - **(b) Redundant** — passed, but removal leaves `coverage_delta` unchanged. Computed per-test, not per-shard aggregate.
   - **(c) Tautological** — asserts a value constructed from the same literal. Include this pattern verbatim in the triage prompt: `x := makeX(field: "val"); assert x.field == "val"`. Tautological tests pass, touch new lines, and produce a positive delta — the gate will not catch them without explicit instruction.

   The orchestrator must also `git diff` each shard's worktree against its base; any shard that modified a non-test file has gone out of scope — drop it or flag it. Cherry-pick survivors into integration branch `pipeline/test-gen-<short>` from the mounted HEAD of `/workspace/repo`. Use plain `git cherry-pick` or `git merge` — the orchestrator has no language toolchain.

8. **LAND.** `cp -a /workspace/repo /out/repo` — orchestrator only (see Watch out). `/workspace` is torn down when the orchestrator exits; `/out` persists. Write `/out/CHANGES.md`: branch name, base commit, units covered, per-file coverage uplift, dropped-test breakdown.

9. **REPORT.** Write `/out/SUMMARY.md`: before/after coverage table per file; total tests added and dropped with breakdown (failed/redundant/tautological); units still at low coverage with hypotheses (`"requires live DB"`, `"only reachable via panic path"`, `"exclusion list candidate"`). Name every low-coverage unit — silence is a false signal of completeness. Flag any `// expected_bug=true` survivors for `sandbox-feature-work`.

## Writing the orchestrator prompt

Brief the orchestrator as a complete document: surface definition with exclusion list (→ `/workspace/surface.md`), exact repo source path (`/in/<repo>`), `sandbox_script` image for BASELINE and GATE, the nine-step pipeline contract with the background-dispatched WRITE fan-out (≤8 in flight via `sandbox_wait`) and worktree discipline, writer policy (characterise existing behaviour; drop failing tests unless `// expected_bug=true`; no impl patches), tautology pattern verbatim, the bug-flag protocol (if a shard's gate reveals an implementation bug, the orchestrator must include a section in SUMMARY.md flagging it for separate `sandbox-feature-work` handling — it does not auto-patch the production code), and the output contract below. State the integration branch as `pipeline/test-gen-<short>` authored `Pipeline <pipeline@local>`.

Terse prompts produce shallow pipelines. Worktree discipline and triage drop criteria both have a failure mode where the orchestrator skips them under pressure — spell them out.

## Output contract

```
/out/
  repo/        # Full git repo with integration branch (incl .git)
  CHANGES.md   # Branch name, base commit, units covered, coverage uplift per file,
               #   dropped-test breakdown
  SUMMARY.md   # Before/after coverage table, tests added/dropped (with breakdown),
               #   uncovered units with hypotheses, bug flags
```

## Launching the orchestrator

- **`directories: ["<abs path to repo>"]` is mandatory.** Forgetting it mounts nothing; the orchestrator stalls diagnosing an empty `/in` and produces nothing. Verify on every launch.
- Tier: **slow** for the orchestrator; **medium** for WRITE agents. BASELINE and GATE are `sandbox_script` — no model selection needed.
## Host-side landing

The in-sandbox gate is the authoritative signal. Host work is minimal.

1. Read `/out/CHANGES.md` for the branch name and base commit.
2. `git fetch <output_dir>/repo pipeline/test-gen-<short>` then `git merge --ff-only FETCH_HEAD`.
3. Re-author: `git commit --amend --reset-author --no-edit` (single commit) or `git rebase <base> --exec "git commit --amend --reset-author --no-edit"` (several).
4. Run one cheap in-repo test pass as a backstop. A failure signals an environment gap, not routine churn.
5. Review `// expected_bug=true` tests from `SUMMARY.md` and route to `sandbox-feature-work`.

The host does not re-run the generation pipeline.
