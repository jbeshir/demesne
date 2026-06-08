---
name: sandbox-bisect-hunt
description: Narrow down which commit, file, config flag, dependency version, or input subset introduced a defect, regression, performance cliff, or behaviour change — through a demesne-orchestrated binary search. A slow-tier orchestrator captures a deterministic reproducer and a search axis, drives a sequential sandbox_script probe loop (one fresh sandbox per midpoint, log2(N) steps), confirms the culprit with a verify probe, then hands everything to a sandbox_agent reader that writes a root-cause report with a suggested fix shape. Apply when the user knows something broke and wants to know exactly which change caused it — "bisect this regression", "find the commit that broke X", "which dependency version introduced this", "track down the flag that caused this", "what changed to make this fail", "regression hunt". Report-only by default — routes into sandbox-feature-work when the user wants the fix applied. Skip for broad quality sweeps (use sandbox-quality-improvement), static defect catalogues (use sandbox-code-defect-survey), and new feature work (use sandbox-feature-work).
allowed-tools: Read, Glob, Grep, Bash, Write, Edit, mcp__demesne__sandbox_agent, mcp__demesne__sandbox_script
---

Narrow down the exact commit, file, flag, or package version that introduced a regression by driving binary search over a search axis. A slow-tier orchestrator captures a deterministic reproducer, runs a sequential `sandbox_script` probe loop (log₂(N) probes, one fresh sandbox each), confirms the culprit with a verify probe, then hands the trace to a medium-tier reader that writes the root-cause report. Deliverable: markdown in `/out`; no code landing.

**Watch out:** The reproducer must be deterministic — same command, same binary exit every time; LLM judgment does not count. The loop is strictly sequential; fanning out probes in parallel breaks the bisect invariant. The orchestrator must `cp` artefacts into its own `/out`; a child doing the copy strands them under `/out/child/<name>/`.

## Procedure

1. **INTAKE** — Orchestrator writes `/workspace/symptom.md`: the reproducer command (the single good-vs-bad discriminant); the axis type and bounds (git commit range `<good>..<bad>`, version list, flag list, or file list); the known-good anchor; and, for performance regressions, the metric threshold (e.g. "bad if p99 > 200 ms"). No probe loop starts until `symptom.md` exists.

2. **AXIS PREP** — Enumerate the search axis into `/workspace/axis.jsonl` (one JSON object per line).
   - **Git-commit bisect**: `cp -a /in/<repo> /workspace/repo` — the `-a` flag copies `.git`; without it `git checkout` fails with "not a git repository". Run `git -C /workspace/repo log --oneline <good>..<bad>` and emit `{"index": N, "sha": "..."}` per line.
   - **Non-git bisect** (dependency versions, feature flags, file subsets): enumerate directly as `{"index": N, "value": "..."}`, with index 0 = good anchor and index len-1 = bad anchor.

   If N > 4096, surface the count and ask for confirmation before proceeding — at that scale ~12 probes are needed but a tighter range is almost always available.

3. **BISECT LOOP** — Maintain `good_idx` and `bad_idx`; loop until `bad_idx - good_idx == 1`.

   Each iteration:
   - Compute `mid_idx = (good_idx + bad_idx) // 2`.
   - Spawn `sandbox_script` named `probe-NN` (NN = zero-padded step counter, e.g. `probe-01`, `probe-12`). Image: match the repo's toolchain (`go`, `python`, `node`, `anaconda`). Egress: `none` if the reproducer needs no network; `package-managers` for dependency-version bisects that must install packages. The probe script: (a) checks out or configures the midpoint (`git -C /workspace/repo checkout <sha>` for git bisects; writes config/env for non-git); (b) runs the reproducer command verbatim from `symptom.md`; (c) writes `/out/result.json`: `{"exit": 0_or_nonzero, "metric": float_or_null, "log_tail": "last 30 lines of output"}`.
   - `exit == 0` (or metric below threshold) → `good_idx = mid_idx`; else `bad_idx = mid_idx`.
   - Append to `/workspace/bisect.log`: `step=NN mid=<sha_or_value> exit=N good=<idx> bad=<idx>`. Flush incrementally — the log lives in `/workspace` and survives an orchestrator crash mid-loop; generating it only at the end loses the trace.

   Use `sandbox_script` (not `sandbox_create`/`sandbox_exec`) so each probe starts from a clean slate — leftover build artefacts or cached packages from a prior probe silently corrupt the bisect. All probes share `/workspace/repo`; do not run them in parallel, as concurrent `git checkout` calls collide.

4. **CULPRIT VERIFY** — Culprit is `axis[bad_idx]`; predecessor is `axis[good_idx]`. Spawn one `sandbox_script` (`name=verify-culprit`, same image and egress) that runs the reproducer against both: culprit must exit nonzero / above threshold; predecessor must exit 0 / below threshold. If either assertion fails, log the anomaly to `bisect.log` and note "non-monotone axis suspected" in `SUMMARY.md`. This probe guards against off-by-one in the index logic and against non-monotonic regressions (a fix mid-range that was later re-introduced).

5. **ROOT-CAUSE WRITEUP** — Before spawning the reader, write `/workspace/culprit.diff`: `git -C /workspace/repo diff <predecessor-sha> <culprit-sha>` for git bisects; the version or flag delta for non-git axes. Then spawn one medium-tier `sandbox_agent` (`name=reader`) that reads `/workspace/symptom.md`, `/workspace/bisect.log` (which includes the verify outcome), and `/workspace/culprit.diff`. It writes `/out/ROOT_CAUSE.md` with: culprit identified; the change that introduced the regression (diff or delta, summarised); the most likely mechanism; the suggested fix shape (what to change, not the patch itself); and what to monitor going forward. The reader does not apply any fix.

6. **DELIVER** — In the orchestrator's own process: `cp /out/child/reader/ROOT_CAUSE.md /out/` and `cp /workspace/bisect.log /out/bisect.log`. Do not delegate this copy to a `sandbox_script` child — that child writes only to its own `/out/child/<name>/`, stranding the files there. Write `/out/SUMMARY.md`: axis type, N / log₂(N) step count, culprit, verify outcome, one-paragraph root-cause summary. Print `DONE`.

   If the user wants the fix landed, pass `/out/ROOT_CAUSE.md` as context to a `sandbox-feature-work` orchestrator prompt — this skill identifies the culprit only.

## Variants

- **Git-commit bisect** — axis is `git log <good>..<bad> --oneline`; each probe does `git checkout <sha>` then runs the reproducer. Copy the repo with `.git` in step 2. The most natural fit for binary search.
- **File bisect** — axis is "which file in this changeset broke it"; each probe uses `git -C /workspace/repo checkout HEAD -- <subset>` to revert a file subset and runs the reproducer. Useful when a large diff landed at once and commit-level bisect is not possible.
- **Flag bisect** — axis is a list of feature flags; each probe sets a subset via env vars or a config file. Never needs egress.
- **Dependency-version bisect** — axis is candidate package versions; each probe pins to the version (writes `requirements.txt`, `go.mod` replacement, etc.) and installs it. Requires `egress: package-managers`.

## Writing the orchestrator prompt

Brief it as a complete document:

1. **The symptom and reproducer** — the exact command that distinguishes good from bad, deterministically. If you cannot hand over a deterministic reproducer, stop — the bisect cannot run.
2. **The axis** — type (git commit range, version list, flag list, file list) and bounds. For git: name the repo in `/in`. For non-git: provide the enumeration.
3. **The known-good anchor** — the commit or version confirmed symptom-free; without it the search has no lower bound.
4. **The pipeline contract** — the six steps above; loop is sequential; `sandbox_script` per probe is mandatory; repo must be copied whole including `.git` for git bisects.
5. **Image and egress** — which `sandbox_script` image to use and whether probes need `egress: package-managers`.
6. **Metric threshold** (if performance) — the threshold and direction (e.g. "> 200 ms is bad").
7. **Already-ruled-out facts** — the orchestrator should not re-tread ruled-out axes.
8. **Output contract** — `/out/ROOT_CAUSE.md`, `/out/bisect.log`, `/out/SUMMARY.md`; report-only, no fix applied.

## Output contract

```
/out/
  ROOT_CAUSE.md      # culprit, mechanism, fix shape
  bisect.log         # full step trace: one line per probe + verify result
  SUMMARY.md         # axis type, N, step count, culprit, verify outcome, one-para summary
```

## Launching the orchestrator

- **`directories: ["<abs path to repo>"]`** is required for git-commit and file bisects — the orchestrator copies the repo from `/in/<repo>`. Omitting it leaves the orchestrator with no repo and no way to run git commands. For non-git bisects with no repo, `directories` may be omitted.
- Tier: **slow** for the orchestrator; **medium** for the reader child. Probes are `sandbox_script` (no model tier).
- Child names must be DNS-1123: lowercase letters, digits, interior hyphens, ≤40 chars. `probe-01`, `probe-12`, `verify-culprit`, `reader` are all valid; `Probe_1` and `verify.culprit` are not — invalid names produce invalid volume names.