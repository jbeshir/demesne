---
name: sandbox-feature-work
status: alpha
description: Drive a non-trivial code change through a demesne-orchestrated pipeline — a slow-tier orchestrator that runs research → plan → orchestrator-chosen numbered implementation phases → an authoritative in-sandbox validation gate → iterative code+completeness review and fix, then a minimal host-side landing (branch fetch + a cheap re-check + integration tests). Apply when the user wants a feature, refactor, or other substantial change built in a sandbox rather than edited directly on the host. Triggers include "run this through demesne", "plan a pipeline to…", "use a sandboxed orchestrator", "implement this in phases in the sandbox", and any request to build something larger than a quick edit where the demesne MCP is available. Skip for trivial one-line edits, pure investigation, or work in a repo demesne can't reach.
allowed-tools: Read, Glob, Grep, Bash, Write, Edit, mcp__demesne__sandbox_agent, mcp__demesne__sandbox_research, mcp__demesne__sandbox_script
---

Drive a substantial code change through a demesne pipeline. A slow-tier orchestrator copies the repo, runs research → plan → numbered implementation phases → authoritative in-sandbox validation → code review and fix, commits on a branch, and delivers `/out/repo`. The host does a minimal landing — fetch, fast-forward, re-author, one cheap re-check, and integration tests.

**Watch out:** The orchestrator must `cp -a /workspace/repo /out/repo` itself — a `sandbox_script` child writes only to `/out/child/<name>` and would strand the repo there. `/workspace` is torn down when the orchestrator exits; only `/out` persists.

## Procedure

1. **Stage.** Copy the repo whole (including `.git`) into `/workspace/repo`: `cp -a /in/<repo>/. /workspace/repo`. `.git` enables `git diff` for reviewers.

2. **Research.** Spawn a `sandbox_research` child (open egress). It has a fresh private `/workspace`, no `/in` mounts, and no access to the repo — any codebase facts it needs must be in its prompt. It writes `/out/FINDINGS.md`.

3. **Plan.** Read `/out/FINDINGS.md` and decide the numbered implementation phases. Write `/workspace/PLAN.md` with a handoff contract: what each phase receives, edits, and leaves for the next.

4. **Implement.** Spawn one medium-tier `sandbox_agent` per phase (`name=phase01`, `phase02`, …). Names must be lowercase DNS-1123 labels — letters, digits, interior hyphens, ≤40 chars; `Phase_1` or `review.final` are invalid and poison sibling spawns. Each phase edits `/workspace/repo` and writes `/out/SUMMARY.md`. Phases share `/workspace` and must run sequentially — parallel writes to `/workspace/repo` corrupt the shared index.

5. **Validate.** Spawn a `sandbox_script` with `image=<lang>` matching the project (`go`, `node`, `python`, or `anaconda`) and `egress=none`. Run the project's **full** validation gate — formatter, linters, tests, build — usually wrapped in a single target (often `make validate`). A bare compile-and-test (`go build && go test`, `npm test`, `pytest`) skips the format/lint/static-analysis checks the project enforces. The base image ships only the language toolchain, not the project's linters, so the gate must install them — through the project's existing version pinning, never at `@latest`, so the sandbox, host, and CI resolve identical versions. Pin through whatever the project already uses (Go: `tool` directives in `go.mod`, run as `go tool golangci-lint run`, fetched via the package-proxy sidecar so `egress=none` works; Node: the lockfile + `npx`; Python: a `poetry`/`uv` lock). Unpinned tools cause in-sandbox green / host-re-check red. This gate is authoritative; the host does not repeat it.

6. **Review.** Spawn a medium-tier `sandbox_agent` (`name=review01`, `review02`, …). It reads `git -C /workspace/repo diff`, judges code quality and completeness against the spec, and writes `/out/REVIEW.md` ending in a `PASS` or `CHANGES_NEEDED` verdict line. On `CHANGES_NEEDED` the orchestrator spawns a fix phase and re-reviews. Cap at 3 rounds.

7. **Finalize.** Commit the validated work as a single commit on `pipeline/<short-task>` in `/workspace/repo`, branched from the mounted HEAD. Multiple commits only when logical phases genuinely warrant separation; `/out/CHANGES.md` must then list the commit range. Author as `Pipeline <pipeline@local>` — the host re-authors after landing.

   Then, in the orchestrator's own process: `cp -a /workspace/repo /out/repo`. Write `/out/CHANGES.md` (branch name, base commit, commit count or range, what changed, how it was validated, any caveats). Print `DONE`.

## Launching the orchestrator

- **`directories: ["<abs path to repo>"]` is mandatory.** Without it the orchestrator wakes with no repo and stalls diagnosing the empty mount. Double-check it on every launch.
- Tier: **slow** for the orchestrator; **medium** for implementation and review phases (the orchestrator sets these when spawning children).
- Long runs survive Anthropic quota windows via the retry/resume wrapper. There is no per-pipeline checkpoint-resume — size the work accordingly.
- If children start dying mid-run with partial transcripts and no `results.json`, suspect the keepalive mechanism (progress notifications over the held-open MCP connection) — investigate by reading the code path; don't guess.

## Writing the orchestrator prompt

The orchestrator starts cold with only your prompt and the mounted repo. Write a complete briefing:

1. **Goal and why** — enough depth for the orchestrator to make judgment calls.
2. **Feature spec + settled design decisions** — so phases don't relitigate resolved choices.
3. **Pipeline contract** — the seven steps above, the child naming rule, and that research is isolated with no repo access.
4. **"Do NOT build, lint, or test yourself — validate via a `sandbox_script` child on the project's language image."** The agent image has no build toolchain; an orchestrator that tries to compile wastes turns failing.
5. **VALIDATE command explicitly** — which `image=<lang>` to use and the project's full gate (formatter + linters + tests + build, e.g. `make validate`), not just a bare compile-and-test.
6. **Files to study** — key files/dirs so the orchestrator doesn't rediscover structure during research.
7. **Handoff contract requirement** — write `/workspace/PLAN.md`; each phase honours it strictly.
8. **Already-ruled-out facts** — anything investigated and disproven, so the orchestrator doesn't re-tread dead ends.
9. **Output contract** — `/out/FINDINGS.md`, per-phase `/out/SUMMARY.md`, `/out/REVIEW.md` with a `PASS`/`CHANGES_NEEDED` verdict line, committed branch in `/out/repo`, `/out/CHANGES.md` (branch name, base commit, commit count or range), `DONE`.

## Output contract

```
/out/
  repo/          # full .git repo; host fetches pipeline/<short-task> from here
  FINDINGS.md    # research output
  SUMMARY.md     # per phase (phase01, phase02, …)
  REVIEW.md      # verdict: PASS or CHANGES_NEEDED
  CHANGES.md     # branch name, base commit, commit count/range, what changed, caveats
```

Workspace artefacts (read by the orchestrator; torn down on exit):
```
/workspace/
  repo/     # working copy with .git
  PLAN.md   # numbered phases + handoff contract
```

## Host-side landing

The in-sandbox validation gate is authoritative. The host does not re-run the full build/lint/test loop.

1. Read `/out/CHANGES.md` for the branch name, base commit, and summary.
2. Fetch and fast-forward:
   - `git -C <repo> fetch <output_dir>/repo <branch>`
   - `git -C <repo> merge --ff-only FETCH_HEAD`
   - **Fallback:** if `<output_dir>/repo` is absent (orchestrator copied via a child), find the repo under `<output_dir>/child/<name>/repo` and fetch from there.
3. Re-author the landed commits:
   - Single commit: `git -C <repo> commit --amend --reset-author --no-edit`
   - Multiple: `git -C <repo> rebase <base> --exec "git commit --amend --reset-author --no-edit"` (`<base>` is in `CHANGES.md`)
   - Confirm: `git log -1 --format='%an <%ae>'` (or `git log <base>..HEAD --format='%an <%ae>'` for multiple)
4. Run the project's gate once (e.g. `make validate`) as a backstop. With pinned tooling it passes first time; a failure is an environment gap to fix. Read the log — a trailing command can mask an earlier failure with exit 0.
5. Run the project's host-only integration tests, if it has them — the checks the sandbox can't run (real services, hardware, or credentials; e.g. `make test-integration`).
6. Manual quality pass; integrate/commit when the user asks.
