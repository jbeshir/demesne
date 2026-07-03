---
name: sandbox-quality-improvement
status: alpha
description: Bring an existing codebase that has been through prior rounds of work up to good standing via a demesne-orchestrated audit → fix → re-review loop. A slow-tier orchestrator runs the deterministic gate first, fans out specialized qualitative reviewers (correctness, security, design/maintainability, completeness, types) that audit the whole codebase citing file:line, synthesizes and tiers their findings, applies behaviour-preserving fixes in small phases, and iterates until the gate is green and no blocking findings remain. Apply when the user wants to harden, clean up, or quality-pass a whole repo or subsystem rather than ship a feature — "get this to good standing", "quality sweep", "clean up the codebase", "audit and fix quality issues", "pay down quality debt". Skip for tidying a single just-written diff (use quality-pass on the host instead) or for building new functionality (use sandbox-feature-work).
allowed-tools: Read, Glob, Grep, Bash, Write, Edit, mcp__demesne__sandbox_agent, mcp__demesne__sandbox_research, mcp__demesne__sandbox_script
---

Audit a whole codebase through specialized reviewer agents — correctness, security, design, completeness, types — fix blocking findings in behaviour-preserving phases, and leave a documented backlog. The host launches one slow-tier `sandbox_agent` orchestrator that runs the loop and copies the committed repo to `/out/repo`; the host lands the branch.

**Watch out (cross-cutting):** any fix that changes observable behaviour is a feature decision, not a quality fix — route it to BACKLOG.md, never apply it in a fix phase. The orchestrator must `cp /workspace/repo /out/repo` itself; delegating to a `sandbox_script` child strands the repo at `/out/child/<name>/`.

## Procedure

1. **Stage the repo.** `cp -a /in/<repo>/. /workspace/repo` — the `-a` flag preserves `.git`; without it there are no branches to cut and reviewers have no `git log`.

2. **BASELINE** — `sandbox_script` with `image=<lang>` matching the project (`go`, `node`, `python`, or `anaconda`), `egress=none`. Run the project's full gate — formatter, linters, tests, build, often a single target like `make validate` — against `/workspace/repo` and capture the output. The base image ships only the language toolchain, not the project's linters; the gate must install them through the project's existing version pinning, never at `@latest`, so sandbox, host, and CI resolve identical versions (Go: `tool` directives in `go.mod`, run as `go tool golangci-lint run`, fetched via the package-proxy sidecar so `egress=none` works; Node: the lockfile + `npx`; Python: a `poetry`/`uv` lock). Never skip a major check because a tool isn't preinstalled. Fix all gate failures before qualitative review; everything the gate flags is off-limits for the auditors.

3. **AUDIT** — spawn one medium-tier `sandbox_agent` per dimension (names: `audit-correctness`, `audit-security`, `audit-design`, `audit-completeness`, `audit-types`; lowercase DNS-1123 — letters, digits, interior hyphens, ≤40 chars; bad names produce invalid volume names and poison sibling spawns). Dispatch each with `background: true` and poll with `sandbox_wait` so they run concurrently; ≤8 in flight — a host-resource guard, not a demesne-enforced cap. Each reads the codebase and `git -C /workspace/repo log`, then writes `/out/AUDIT-<dim>.md`: findings each with file:line, a severity tier (blocking / non-blocking), and a one-line rationale. Must not re-flag gate-covered issues; findings without file:line evidence are rejected. Dimension scope:
   - **correctness** — swallowed/ignored errors, missing error handling at boundaries, races, resource/goroutine leaks, incorrect edge-case logic.
   - **security** — trust-boundary correctness, credential/secret handling, injection surfaces, over-broad permissions.
   - **design / maintainability** — dead code, unused fields/functions, premature or unjustified abstraction, mutable accumulation where immutable would do, duplicate definitions, over-defensive optional fields, naming that encodes history.
   - **completeness / wiring** — half-implemented features, stray TODOs, fields never read/displayed, data that connects to no output, parity gaps (OpenAPI ↔ router, MCP client ↔ domain, tool defs ↔ manifest ↔ README, new invariants ↔ CI).
   - **types** — string comparisons on errors instead of `errors.Is`/`errors.As`, type switches that should be methods, bare primitives that want named types, loose `any` where the type is known.

4. **SYNTHESIZE** — merge the five audits into one deduped, prioritized `/workspace/ISSUES.md`, blocking findings first. Adjudicate overlaps yourself — reviewers do not debate each other.

5. **PLAN** — group blocking findings into small numbered phases (a few hundred lines of change each). Write `/workspace/PLAN.md` with a handoff contract per phase.

6. **FIX** — spawn one medium-tier `sandbox_agent` per phase (`name=fix01`, `fix02`, …), editing `/workspace/repo` directly. Fix phases share `/workspace`; run them sequentially unless genuinely independent. Each writes its own `/out/SUMMARY.md`. Any change to observable behaviour belongs in BACKLOG.md, not applied here.

7. **GATE** — after each fix phase or batch, re-run the step 2 gate against `/workspace/repo`. Must be green before re-review. Green tests are the proof behaviour was preserved.

8. **RE-REVIEW** — spawn a medium-tier `sandbox_agent` (`name=review01`, `review02`, …) to read `git -C /workspace/repo diff` and write `/out/REVIEW.md` ending in a verdict: `PASS` or `CHANGES_NEEDED`. On `CHANGES_NEEDED` (blocking findings only): spawn another fix phase and repeat gate + re-review. Cap at 3 rounds. If only non-blocking findings remain at any round, that is a `PASS` — push them to the backlog, don't loop.

9. **FINALIZE** — commit the validated work as a single commit on `pipeline/<short-task>` from the mounted HEAD; use multiple commits only when fix phases genuinely warrant it (name the commit range in CHANGES.md). Author as `Pipeline <pipeline@local>` — the host re-authors at landing. Then `cp -a /workspace/repo /out/repo` in the orchestrator's own process — not via a `sandbox_script` child (which writes to `/out/child/<name>/`). `/workspace` is torn down when the orchestrator exits; only `/out` persists. Write `/out/CHANGES.md` (branch name, base commit, commit count, what changed, validation summary, behaviour-preservation note), `/out/BACKLOG.md` (non-blocking findings + deferred behaviour changes), and print `DONE`.

## Definition of good standing

The loop is complete when all three hold: gate is green; no blocking findings remain; non-blocking findings are captured in BACKLOG.md. A clean gate + zero blocking findings + a documented backlog is good standing — perfection is not the bar.

## Writing the orchestrator prompt

Brief it fully — it starts cold with only your prompt and the mounted repo:

1. **Goal** — bring `/workspace/repo` to good standing: audit, fix blocking quality issues, leave a backlog. Not a feature change.
2. **Rubric** — tell it to read the root and project `CLAUDE.md` files; those define quality checks and parity requirements. Intended behaviour = tests + public API; behaviour must be preserved (green tests = proof).
3. **Pipeline contract** — the nine steps above, the dimension list, the child-naming rule, and the gate-first / no-re-find-gate-issues rule.
4. **Tooling constraint** — "Do NOT build, lint, or test yourself — run the gate via a `sandbox_script` child on the project's language image." The orchestrator agent image has no build toolchain.
5. **Exact gate command and `image=<lang>`** — the project's full gate (e.g. `make validate`) including the lint target, not just a bare compile-and-test.
6. **Severity discipline** — every finding needs file:line + tier; only blocking findings drive fix/re-review cycles; 3-round cap; "good standing with a backlog" is valid.
7. **Output contract** — `/out/AUDIT-<dim>.md`, `/workspace/ISSUES.md`, `/workspace/PLAN.md`, per-phase `/out/SUMMARY.md`, `/out/REVIEW.md` (PASS/CHANGES_NEEDED verdict), committed work on branch copied to `/out/repo` by the orchestrator itself, `/out/CHANGES.md` + `/out/BACKLOG.md` + `DONE`.

## Output contract

```
/out/
  repo/           # full .git repo; host fetches pipeline/<short-task> from here
  AUDIT-<dim>.md  # per-dimension findings (5 files)
  REVIEW.md       # last-round verdict (PASS / CHANGES_NEEDED)
  CHANGES.md      # branch name, base commit, commit count, what changed,
                  # validation summary, behaviour-preservation note
  BACKLOG.md      # non-blocking findings + deferred behaviour changes
```

Workspace artefacts (orchestrator reads; not in `/out`):
```
/workspace/
  repo/       # working copy with .git
  ISSUES.md   # deduped prioritized findings
  PLAN.md     # phase breakdown
```

## Launching the orchestrator

- **`directories: ["<abs path to repo>"]` is mandatory.** Without it the orchestrator mounts no repo and produces nothing.
- Tier: **slow** for the orchestrator; **medium** for all children (auditors, fixers, reviewers).
## Host-side landing

1. Read `/out/CHANGES.md` for the branch name and base commit.
2. `git -C <repo> fetch <output_dir>/repo <branch>` then `git -C <repo> merge --ff-only FETCH_HEAD`. If `/out/repo` is absent, the copy landed under `<output_dir>/child/<name>/repo` — fetch from there.
3. Re-author commits: single commit: `git commit --amend --reset-author --no-edit`; multiple: `git rebase <base> --exec "git commit --amend --reset-author --no-edit"`. The base commit is in CHANGES.md.
4. Run one in-repo gate re-check (e.g. `make validate`) and the project's host-only integration tests, if it has them (e.g. `make test-integration`). Read the log — a trailing command can mask a failure with exit 0.

The in-sandbox gate is authoritative; the host landing is a backstop. Do not re-run the full audit-fix loop.
