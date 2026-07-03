---
name: sandbox-tech-debt-audit
description: Audit an MVP-era codebase for accumulated technical debt and emit a sprint-sequenced remediation plan plus a backfill of undocumented architecture decisions. A slow-tier orchestrator measures test coverage deterministically (never guesses it), fans out reviewers across structural weaknesses / thin coverage / refactoring candidates / undocumented decisions citing file:line, then a sequencer buckets every finding into must-fix-before-next-release / can-wait-a-sprint / acceptable-ongoing-debt with per-item effort and risk, a fresh critic demotes over-classified must-fixes, and a backfill child drafts the architecture decisions that lived only in the founder's head as a proposed CLAUDE.md section. Report-only — produces the plan and proposed docs, does not fix or land code. Apply when a founder whose product now carries real traffic wants to know what debt built for MVP speed is compounding and in what order to pay it down — "tech debt audit", "what should we refactor first", "remediation plan across sprints", "our MVP codebase needs cleanup sequencing", "document the architecture decisions in my head". Skip when you want the debt actually fixed and landed (use sandbox-quality-improvement), a security- or compliance-framed audit (use sandbox-prelaunch-security-review or sandbox-compliance-review), a fresh taxonomy of AI-code antipatterns (use sandbox-code-defect-survey), tests actually written (use sandbox-test-gen-from-spec), or architecture defined *before* a build (use sandbox-mvp-architecture-charter — this skill backfills decisions after the fact).
allowed-tools: Read, Glob, Grep, Bash, Write, Edit, mcp__demesne__sandbox_agent, mcp__demesne__sandbox_script
---

Audit an existing MVP codebase for compounding technical debt and hand the founder a prioritized, sprint-sequenced remediation plan plus a proposed CLAUDE.md architecture-decision section — the two deliverables the Launch-stage "technical debt comes due" activity calls for. The host launches one slow-tier `sandbox_agent` orchestrator that measures coverage, fans out auditors, sequences the findings, and copies the reports into `/out`. **Report-only:** it produces the plan and the proposed docs; it does not apply fixes or cut a branch. Fixes route out to `sandbox-quality-improvement` (behaviour-preserving) or `sandbox-feature-work` (larger refactors).

**Watch out (cross-cutting):** coverage must be **measured** by a `sandbox_script` running the real test suite — an LLM reading test files and estimating a percentage silently corrupts the whole test-coverage dimension and the sequencing built on it. The proposed CLAUDE.md section is a **draft for founder review**, never landed by this pipeline — architecture decisions become canonical only after a human confirms them. The orchestrator must `cp` deliverables into its own `/out`; a child's `/out` is `/out/child/<name>/` and delegating the copy to a `sandbox_script` child strands the files.

## Procedure

1. **Stage the repo.** In the orchestrator's own process: `cp -a /in/<repo>/. /workspace/repo`. The `-a` preserves `.git` — without it the decision-archaeology child has no `git log` and the coverage run may miss test config. Read any existing root/project `CLAUDE.md` now so backfill only documents what is *not* already recorded.

2. **COVERAGE (deterministic).** One `sandbox_script`, `name=coverage-run`, `image=<lang>` matching the project (`go`, `node`, `python`, `anaconda`), `egress=package-managers` if the coverage tool isn't in the base image (install through the project's own pinning — Go `go test -coverprofile`, Node lockfile + `c8`/`jest --coverage`, Python `pytest --cov`), else `egress=none`. Run the suite against `/workspace/repo` and emit per-package/per-file coverage. Write `/workspace/coverage.txt`. If a module's tests won't run, record it as **UNMEASURED** with the reason — never fabricate a number. This is a blocking call and completes before the fan-out, so the audit children read a coverage report that already exists.

3. **AUDIT (fan-out).** Spawn four medium-tier `sandbox_agent` children — `audit-structural`, `audit-coverage`, `audit-refactor`, `audit-decisions` (DNS-1123: lowercase letters, digits, interior hyphens, ≤40 chars; bad names poison sibling spawns). Dispatch each with `background: true`, collect `job_id`s, poll `sandbox_wait` (`timeout_seconds: 120`) to completion — ≤8 in flight (a host-resource guard). Blocking calls issue one per turn and run sequentially, so background dispatch is the only way they run concurrently. Each reads `/workspace/repo` (and `git -C /workspace/repo log`), writes `/out/AUDIT-<dim>.md` — every finding needs `file:line`, a one-line rationale, and a remediation-effort estimate (S/M/L); no-evidence findings are rejected. Dimension scope (from the playbook's audit brief):
   - **structural** — god modules, tangled/circular dependencies, missing layer boundaries, duplicated subsystems, MVP-speed shortcuts (hardcoded config, copy-pasted handlers, missing abstractions) that now compound under traffic and new features.
   - **coverage** — reads `/workspace/coverage.txt`; for each low- or zero-coverage area classify **high-risk-untested** (hot path, changes every sprint, handles money/auth/data) vs **trivial-untested** (stable, low-blast-radius). Cite measured numbers; surface every UNMEASURED module.
   - **refactor** — duplication, over-long functions, dead code, primitive obsession, and patterns that drifted inconsistent across accumulated sessions.
   - **decisions** — mine `git log`, code, and comments for architecture decisions living only in the founder's head: undocumented choices, "why X not Y", implicit invariants. Output raw mined decisions with commit SHA / `file:line` evidence — the backfill child turns these into prose. Skip anything already in the existing CLAUDE.md.

4. **SEQUENCE.** After all four audits complete, the orchestrator dedups and merges them into `/workspace/FINDINGS.md` (adjudicate overlaps itself — auditors don't debate). Then spawn one **slow-tier** `sandbox_agent`, `name=sequence-plan`, reading `/workspace/FINDINGS.md` + `/workspace/coverage.txt`, to produce `/out/REMEDIATION-PLAN.md`: every finding placed in exactly one of the playbook's three buckets — **must-fix-before-next-release / can-wait-a-sprint / acceptable-ongoing-debt** — each row carrying id, dimension, `file:line`, why-it's-debt, measured evidence, effort (S/M/L), risk-if-deferred, target sprint, and execution route (`sandbox-quality-improvement` / `sandbox-feature-work`). Order within must-fix by risk-if-deferred desc then effort asc, respecting dependencies (a refactor that unblocks others sequences first).

5. **VERIFY (fresh critic, filter not fix-loop).** Spawn one medium-tier `sandbox_agent`, `name=verify-plan`, that reads `/out/REMEDIATION-PLAN.md` (via `/in/previous-jobs/sequence-plan/`) and the repo, and tries to **refute each must-fix classification** — demoting any item whose release-blocking risk isn't evidenced at the cited `file:line`, and flagging acceptable-ongoing-debt that's actually understated. Verdict `PASS` / `CHANGES_NEEDED` → `/out/PLAN-REVIEW.md`. On `CHANGES_NEEDED`, re-run `sequence-plan` once with the critique. **Cap: 2 rounds** — a plan is a proposal, not a proof.

6. **BACKFILL.** Spawn one medium-tier `sandbox_agent`, `name=backfill-claudemd`, reading the mined decisions (`/in/previous-jobs/audit-decisions/` + `/workspace/repo`), that drafts `/out/ARCHITECTURE-DECISIONS.md` — a proposed CLAUDE.md section, one entry per decision: **Decision** / **Context** / **Alternatives considered & why rejected** / **Constraints it imposes** / **Evidence** (SHA or `file:line`). Header states plainly this is a draft for founder review, not a landed change.

7. **COLLATE.** The orchestrator `cp`s the audit files and plan into its own `/out` (plain `cp`, its own process — not a `sandbox_script` child). Write `/out/EXECUTIVE_SUMMARY.md`: candid codebase-health read (what MVP speed bought and what it cost), a findings table (`dimension | count | top severity | headline`), the three-bucket sprint headline, and the routing note. Print `DONE`.

## Writing the orchestrator prompt

Brief it as a complete document — it starts cold with only your prompt and the mounted repo:

1. **Target and domain** — what the product is, its stack, a repo map (key dirs/packages), and the exact test/coverage command + `image=<lang>` for `coverage-run`. Say the product now carries real traffic: this is Launch-stage debt that MVP speed deferred, not a greenfield review.
2. **Pipeline contract** — the seven steps; child-naming rule; coverage is a deterministic `sandbox_script` (measured, UNMEASURED-not-fabricated); audit fan-out is background-dispatch + `sandbox_wait`, ≤8 in flight; `sequence-plan` runs only after all four audits complete.
3. **Audit dimensions** — the four scopes above verbatim; every finding needs `file:line` + effort estimate; "clean on this axis" is a valid honest report, not a failure.
4. **Sequencing rubric** — the three buckets with their definitions: *must-fix-before-next-release* = correctness/security risk on hot paths, zero coverage on high-churn code, refactors other must-fixes depend on; *can-wait-a-sprint* = real cost that survives one release; *acceptable-ongoing-debt* = logged and consciously accepted so it isn't re-litigated. Within-bucket ordering by risk then effort, dependencies first.
5. **Decision-backfill rule** — only decisions *not already in* an existing CLAUDE.md; each entry gets Context + rejected alternatives + imposed constraints + SHA/`file:line` evidence; the output is a **proposed** section for founder review, never landed.
6. **Verifier discipline** — `verify-plan` is a fresh context, refutes must-fix classifications, 2-round cap.
7. **Output contract** — the files below; report-only, no edits/commits/branch; fixes route to `sandbox-quality-improvement` or `sandbox-feature-work`.

## Output contract

```
/out/
  EXECUTIVE_SUMMARY.md        # health read, findings table, sprint headline, routing
  REMEDIATION-PLAN.md         # the deliverable — three-bucket sprint sequencing
  ARCHITECTURE-DECISIONS.md   # proposed CLAUDE.md section (DRAFT, not landed)
  PLAN-REVIEW.md              # verifier verdict (PASS / CHANGES_NEEDED)
  coverage.txt               # measured coverage report
  AUDIT-structural.md
  AUDIT-coverage.md
  AUDIT-refactor.md
  AUDIT-decisions.md
```

`REMEDIATION-PLAN.md` section order: `## Method` → `## Must-fix before next release` → `## Can wait a sprint` → `## Acceptable ongoing debt` → `## Sprint sequence` (which items land which sprint) → `## Execution routing`.

Workspace scratch (orchestrator reads; not in `/out`): `/workspace/repo/` (staged copy with `.git`), `/workspace/coverage.txt`, `/workspace/FINDINGS.md`.

## Launching the orchestrator

- **`directories: ["<abs path to repo>"]` is mandatory.** Without it the orchestrator mounts no repo — coverage measures nothing and the auditors have nothing to read. If a CLAUDE.md already lives in the repo it rides along inside that mount.
- Tier: **slow** for the orchestrator and `sequence-plan`; **medium** for the four auditors, `verify-plan`, and `backfill-claudemd`; `coverage-run` is a `sandbox_script` (no tier).
- No `sandbox_research` — this audit is entirely internal to the mounted repo; there is no open-web step and no branch output directory.

## Acting on the output

The host reads `/out/EXECUTIVE_SUMMARY.md` and `/out/REMEDIATION-PLAN.md` and decides routing — it does not re-run the audit. Must-fix items go to `sandbox-quality-improvement` (behaviour-preserving fixes) or `sandbox-feature-work` (structural refactors) in sprint order. The founder reviews `/out/ARCHITECTURE-DECISIONS.md` and merges the confirmed entries into the repo's CLAUDE.md themselves — the pipeline drafts, the human ratifies.
