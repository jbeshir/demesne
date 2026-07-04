---
name: sandbox-moat-test-suite
status: alpha
description: Turn a founder's observed vertical edge cases into a growing scenario-based test suite through a demesne-orchestrated pipeline — a slow-tier orchestrator stages the product repo, extracts candidate edge cases from a mounted corpus of feedback/tickets/anecdotes, runs a per-case "would a generic horizontal competitor get this wrong?" moat filter (fresh judge per candidate; only vertical-specific cases qualify), fans out test-writers that build realistic-fixture scenario tests each carrying a why-generic-fails note, runs a deterministic gate that separates live moat markers (product passes) from moat gaps (product fails today), and lands surviving tests + fixtures + an accretion protocol on a branch with a moat-map report. Apply when the user wants to convert real vertical edge cases into a durable, competitor-differentiating test suite — "build a moat test suite", "turn these edge cases into tests", "tests only we would know to write", "scenario tests from support tickets", "what would a generic competitor get wrong". Skip for growing plain coverage on undertested code (use sandbox-test-gen-from-spec — it keeps generic cases this skill deliberately rejects), tests bundled with a new feature (use sandbox-feature-work), and report-only moat work (data-flywheel narrative → sandbox-data-flywheel-audit; switching-cost profiles → sandbox-lockin-audit; codifying domain knowledge into context/skills → sandbox-domain-knowledge-codify).
allowed-tools: Read, Glob, Grep, Bash, Write, Edit, mcp__demesne__sandbox_agent, mcp__demesne__sandbox_script
---

Convert observed vertical edge cases into a scenario-based test suite that a generic horizontal competitor could not write, because they haven't seen what this vertical's users have. A slow-tier orchestrator `sandbox_agent` extracts candidate edge cases from a mounted corpus, filters them through a per-case moat judge (only cases a generic competitor would get wrong survive), fans out test-writers that build a realistic input fixture + expected-behaviour assertion + why-generic-fails note per case, gates them, and delivers a branch at `/out/repo` plus a moat-map report and an accretion protocol. This LANDS CODE. The suite is meant to *accrete*: the accretion protocol landed in the repo tells the founder how to append each new edge case as it surfaces.

**Watch out (cross-cutting):** The orchestrator must `cp -a /workspace/repo /out/repo` itself — a child's `/out` is `/out/child/<name>/` and delegating the copy strands the branch. The moat filter is the whole point: without a FRESH judge verdict on every candidate (a judge that is never the extractor) the suite fills with generic cases (empty input, encoding, pagination) that belong in `sandbox-test-gen-from-spec`, and stops being a moat. A scenario test the product *fails today* is a moat GAP, not a bad test — annotate and keep it (it encodes the target behaviour); dropping it loses the spec.

## Procedure

1. **Stage.** `cp -a /in/<repo>/. /workspace/repo` (the `-a` preserves `.git` — needed for worktrees and the host's fetch). The edge-case corpus is a second read-only mount at `/in/<corpus>/`.

2. **Write `/workspace/surface.md`.** The vertical (what sector/domain this product serves); the test framework + how scenario tests are run and where they live (`tests/moat/`); the language image + egress for the gate; and the vertical-concern taxonomy the moat-map is bucketed by (regulatory/compliance gotcha, domain terminology & ontology, sector-workflow assumption, data-shape peculiarity, domain-logic "the obvious answer is wrong here"). Every later step reads this file.

3. **EXTRACT — one medium-tier `sandbox_agent` (`name=extract`).** Reads the corpus at `/in/<corpus>/`. Formats are messy (support-ticket exports, feedback CSVs, founder anecdote notes, transcripts) — parse what it can and log unparseable files to `/workspace/extract-skipped.md` rather than silently dropping them. Emit one candidate per observed edge case to `/workspace/candidates.jsonl`, locked schema:
   ```json
   {"id":"ec-0007","source":"ticket-4412.eml","observed":"<raw observation, quoted>","vertical_concern":"regulatory","expected_behaviour":"<what the product should do>","generic_failure_hypothesis":"<why a horizontal tool would mishandle it>"}
   ```
   For a large corpus, shard the files and run one extractor per shard (`extract-shard-1`…, background fan-out per step 5) — each shard writes its **own** `/workspace/candidates-N.jsonl` (two children appending one shared file produce corrupt JSONL). Once every extractor is terminal the orchestrator concatenates them in its own process — `cat /workspace/candidates-*.jsonl > /workspace/candidates.jsonl` — before the moat filter reads it. A single (unsharded) extractor writes `candidates.jsonl` directly.

4. **MOAT-FILTER — fan out fresh fast-tier judges across the candidates** (`moat-judge-01`, …; each judge takes a batch of ~10 candidates so the fan-out is judges-not-candidates). A FRESH judge, never the extractor. Each answers, per candidate: *would a competent generic/horizontal competitor with no exposure to this vertical get this wrong, AND does the correct behaviour depend on vertical-specific knowledge?* Each judge writes its **own** `/workspace/verdicts-NN.jsonl` keyed to its child index — never a shared file, because concurrent appends to one path corrupt the JSONL; the orchestrator concatenates them at the BUILD barrier (step 5). One verdict line per candidate:
   ```json
   {"id":"ec-0007","verdict":"MOAT","reason":"...","confidence":"high"}
   ```
   `MOAT` → qualifies. `GENERIC` (any careful engineer handles it — empty input, unicode, off-by-one) → route out, do not build. `UNCLEAR` → founder review, do not build. Dispatch with `background: true`, collect `job_id`s, poll `sandbox_wait` (`timeout_seconds: 120`), keep **≤8 in flight** (blocking calls issued one per turn run sequentially and defeat the fan-out; background is the only way siblings run concurrently). This is a filter, not a fix loop — no iteration.

5. **BUILD — one medium-tier `sandbox_agent` per shard of MOAT candidates** (`build-case-01`, …; lowercase DNS-1123, ≤40 chars — bad names poison sibling spawns). **Barrier:** build only after every judge is terminal; the orchestrator then concatenates the per-judge files into one `verdicts.jsonl` in its own process — `cat /workspace/verdicts-*.jsonl > /workspace/verdicts.jsonl` — which is the collect-then-reduce the separate per-judge files exist for. Group qualifying cases into ≤8 shards by target test file to minimise merge surface; background-dispatch the same fan-out loop. Each writer first cuts its own worktree — `git -C /workspace/repo worktree add /workspace/wt-N -b moat/N` — and works only in `/workspace/wt-N` (two writers editing `/workspace/repo` corrupt the shared index). Per case it builds a **scenario test, not a unit test**: a realistic input fixture under `tests/moat/fixtures/<slug>.<ext>` (a representative vertical artefact, not a toy literal), a test that loads the fixture and asserts `expected_behaviour`, and a `// moat: <generic_failure_hypothesis> [concern=<vertical_concern>] [src=<source>]` annotation on each test. It does not modify production code. Commit new files to `moat/N`; write `/workspace/build-N-summary.md`.

6. **GATE — one `sandbox_script` per shard** (`image=<lang>`, same egress as staged; `egress: package-managers` only if test deps must install, pinned through the repo's existing lockfile/tool directives, never `@latest`). Deterministic → `sandbox_script`, never an agent. Run only the new scenario tests in `/workspace/wt-N`; write `/workspace/gate-N.json` with per-test `{"name":..., "pass":true|false, "loads_fixture":true|false}`. A test that does not load its fixture is a disguised unit test — flag it. **Do not treat a failing test as a defect to drop:** `pass:false` means the product gets this vertical case wrong *today* — a moat gap.

7. **REVIEW / TRIAGE — one medium-tier `sandbox_agent`** (`review01`; fresh context, not a builder). Reads every `gate-N.json` and the drafted tests and `git diff`s each worktree. Drop: **(a)** tests that don't load a real fixture or assert only against a same-literal value (tautological/disguised unit tests — `x := makeX(f:"v"); assert x.f=="v"`); **(b)** MOAT false positives the judge let through — a `moat:` note that is generic or unsubstantiated on inspection; **(c)** any shard that touched a non-test/non-fixture file (out of scope). For surviving tests, classify each from the gate: `pass:true` → **live moat marker**; `pass:false` → **moat gap**, re-annotate `// moat_gap: product fails this today` and keep it. On build/collection errors, spawn one fix phase and re-review — cap **2 rounds**. Cherry-pick survivors onto `pipeline/moat-suite-<short>` from the mounted HEAD (plain `git cherry-pick`/`git merge` — the orchestrator has no toolchain); since TRIAGE drops individual tests but shards commit per-file, `git rm` each dropped test file from its worktree before cherry-picking that shard's commit (or keep one test per file, so a drop removes exactly one). End `/out/REVIEW.md` with a `PASS`/`CHANGES_NEEDED` verdict line.

8. **LAND.** `cp -a /workspace/repo /out/repo` — orchestrator only (`/workspace` is torn down on exit; only `/out` persists). Land `tests/moat/ACCRETION.md` in the repo: the fixture/annotation layout, the required `moat:` metadata fields, and the re-run procedure (mount a new corpus, the pipeline appends new cases to `tests/moat/` — extraction and filter are append-safe, existing tests untouched). Write `/out/CHANGES.md` (branch, base commit, cases built, live-markers vs gaps count).

9. **REPORT.** Write `/out/MOAT-MAP.md`: a coverage table by vertical concern (rows = the taxonomy from `surface.md`, columns = live markers / gaps / untested), each qualifying test listed under its concern with its one-line why-generic-fails note — this is the map of the moat. Write `/out/ROUTED.md`: `GENERIC` candidates (→ `sandbox-test-gen-from-spec`) and `UNCLEAR` candidates (→ founder review), each with the judge's reason, so nothing observed is silently lost. Flag every `moat_gap` for `sandbox-feature-work`.

## Writing the orchestrator prompt

Brief it as a complete document:

1. **The vertical and the product** — enough that the moat filter can tell vertical-specific from generic. Name the sector explicitly.
2. **The moat filter test, verbatim** — "would a competent generic/horizontal competitor with no exposure to this vertical get this wrong, and does the correct behaviour depend on vertical-specific knowledge?" — plus GENERIC/MOAT/UNCLEAR routing. This is the load-bearing judgment; over-specify it.
3. **The vertical-concern taxonomy** (regulatory/compliance, domain terminology, sector-workflow, data-shape, domain-logic) — the moat-map axes; adjust to the actual vertical.
4. **Scenario-test discipline** — realistic fixture (a representative vertical artefact, not a toy literal) + expected-behaviour assertion + `moat:` annotation; NOT a unit test. State that a test not loading a fixture is dropped.
5. **The gap policy** — a failing scenario test is a moat gap to annotate and keep, not a defect to drop; flag it for `sandbox-feature-work`.
6. **The nine-step contract** with the background-dispatched MOAT-FILTER and BUILD fan-outs (≤8 in flight via `sandbox_wait`), the barrier before BUILD, and worktree discipline.
7. **Corpus location** (`/in/<corpus>/`), repo source (`/in/<repo>/`), gate `image=<lang>`, and the output contract below.
8. **The locked candidate + verdict schemas** and the branch name `pipeline/moat-suite-<short>`, authored `Pipeline <pipeline@local>`.

Terse prompts collapse the moat filter into generic test-gen — spell out the vertical-specificity bar and the gap policy.

## Output contract

```
/out/
  repo/          # full .git repo with pipeline/moat-suite-<short> (tests/moat/ + fixtures + ACCRETION.md)
  CHANGES.md     # branch, base commit, cases built, live-markers vs gaps count
  REVIEW.md      # verdict: PASS or CHANGES_NEEDED
  MOAT-MAP.md    # coverage by vertical concern; each test + why-generic-fails note; gaps marked
  ROUTED.md      # GENERIC → sandbox-test-gen-from-spec, UNCLEAR → founder, with reasons
```

Workspace artefacts (read by the orchestrator; torn down on exit): `/workspace/surface.md`, per-shard `candidates-N.jsonl` + concatenated `candidates.jsonl`, per-judge `verdicts-NN.jsonl` + concatenated `verdicts.jsonl`, `gate-N.json`, `build-N-summary.md`, `extract-skipped.md`.

## Launching the orchestrator

- **`directories: ["<abs path to repo>", "<abs path to corpus>"]` is mandatory.** Forget the repo and the orchestrator stalls on an empty `/in`; forget the corpus and EXTRACT produces zero candidates and the run yields an empty suite. Verify both on every launch. If the founder has no export, they can drop a plain-text file of anecdotes into the corpus dir — children must handle messy files and log the unparseable.
- Tier: **slow** orchestrator; **fast** moat-filter judges; **medium** BUILD and REVIEW. GATE is `sandbox_script` — no model.

## Host-side landing

The in-sandbox gate is authoritative. Host work is minimal.

1. Read `/out/CHANGES.md` for the branch and base commit.
2. `git -C <repo> fetch <output_dir>/repo pipeline/moat-suite-<short>` then `git merge --ff-only FETCH_HEAD`. Fallback: if `<output_dir>/repo` is absent, fetch from `<output_dir>/child/<name>/repo`.
3. Re-author: `git commit --amend --reset-author --no-edit` (one commit) or `git rebase <base> --exec "git commit --amend --reset-author --no-edit"` (several).
4. Run the moat tests once as a backstop — **expect the moat-gap tests to fail**; that is the designed signal, not a regression. `MOAT-MAP.md` lists which.
5. Route `moat_gap` tests to `sandbox-feature-work` and `ROUTED.md`'s GENERIC cases to `sandbox-test-gen-from-spec`. On the next observed edge case, re-run this skill against an updated corpus per `tests/moat/ACCRETION.md` — the suite grows into the moat map.
