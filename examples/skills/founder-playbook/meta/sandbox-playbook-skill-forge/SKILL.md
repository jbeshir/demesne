---
name: sandbox-playbook-skill-forge
status: alpha
description: Convert a playbook, methodology guide, or process handbook — any document that prescribes a set of concrete activities — into a library of ready-to-invoke demesne skills, one per activity (merging or splitting where judgment says), each run through a design → fresh-critic → refine cycle before landing on a git branch. An orchestrator reads the source document, decomposes its activities into a skill roster, writes shared design briefs, fans out one designer child per skill, batches fresh-context critics over the drafts, runs one capped fix round, then lands accepted skills plus a coverage table and an updated skills README as a committed branch. Apply when the user has a prescriptive document and wants its activities turned into runnable pipelines — "turn this playbook into skills", "build a skill per activity in this guide", "skill-ify this methodology", "convert this handbook into agent pipelines". Skip when only one skill is wanted (design it directly), when the document is descriptive rather than prescriptive (use sandbox-corpus-map-reduce to extract first), or when the deliverable is a report about the document rather than skills (use sandbox-prose-defect-survey or sandbox-docs-quality).
allowed-tools: Read, Glob, Grep, Bash, Write, Edit, mcp__demesne__sandbox_agent, mcp__demesne__sandbox_research, mcp__demesne__sandbox_script
---

Turn a prescriptive source document into a landed library of demesne skills. The host authors one orchestrator prompt, launches a single slow-tier `sandbox_agent`, and that orchestrator decomposes the document's activities into a skill roster, fans out designer children against a shared conventions brief, gates every draft through a fresh-context critic, and commits the accepted skills to a branch in `/out/repo`. The deliverable is real SKILL.md files plus `COVERAGE.md` — not a summary of the document.

**Watch out (cross-cutting):** the conventions brief is the load-bearing artefact — designers never see each other, so any rule not written in `/workspace/brief/conventions.md` will be violated by at least one draft. If the skills repo arrives as a possibly-dirty checkout, reconstruct a clean tree from its `.git` object database (`cp -a /in/<repo>/.git`, then `git checkout -f origin/<default-branch>`) — never trust the mounted working tree. The orchestrator lands accepted drafts into `/workspace/repo` and runs `cp -a /workspace/repo /out/repo` itself; a `sandbox_script` child's `/out` is `/out/child/<name>/` and would strand the branch.

## Procedure

1. **Stage.** `mkdir -p /workspace/repo && cp -a /in/<repo>/.git /workspace/repo/.git`, then `git -C /workspace/repo checkout -f origin/<default-branch>` and `git checkout -b pipeline/<short-name>-skills`. Git operations run in a `sandbox_script` child (`image=go` or any image with git, `egress=none`) against the shared `/workspace/repo` if the orchestrator's own image lacks git.

2. **Orient.** Read the source document at `/in/<doc>` in full, plus the target repo's skill conventions (its skills README, its skill-development how-to, and 2–3 exemplar skills covering the shapes the activities will need: map-reduce, tournament, debate, code-landing). List every prescribed activity with a stable number.

3. **Decompose.** Write the skill roster: merge activities that share one natural deliverable, split activities that hide a host-side action inside an analysis task, and record every merge/split reason — these become `COVERAGE.md` rows. For each roster entry: skill name, source-activity numbers with document line ranges, exemplar skills to adapt, one-paragraph scope, and any special directive (hybrid boundary, code-landing, named methodology to encode verbatim).

4. **Brief.** Write `/workspace/brief/conventions.md` (the target repo's SKILL.md format distilled to hard rules: frontmatter shape, section order, tier language, fan-out mechanics, artefact-path discipline, execution boundary for host-authenticated services, quality bar) and `/workspace/brief/assignments.md` (the roster plus per-skill directives). Where a family of skills depends on current real-world practice, dispatch 1–3 `sandbox_research` children now (background; fresh private workspace, no `/in` mounts — the questions travel in the prompt) and copy their findings into `/workspace/brief/research/` before the dependent designers launch. Tell dependent designers to embed durable findings in their SKILL.md — the findings files won't exist when the skills later run.

5. **DESIGN — one slow-tier `sandbox_agent` per roster entry** (`name=design-<slug>`, DNS-1123: lowercase, digits, interior hyphens, ≤40 chars). Dispatch with `background: true`, collect `job_id`s, poll `sandbox_wait`, keep ≤8 in flight (blocking calls are issued one per turn and run sequentially — background dispatch is the only real concurrency). Each designer's prompt: read the two briefs, its assignment section, the source document, its exemplars; write exactly `/workspace/drafts/<skill-name>/SKILL.md`. Escalate the 3–5 most judgment-heavy roster entries to the highest available tier; a designer that returns claiming to have delegated work "to the background" produced nothing — check the draft file exists before counting it done.

6. **CRITIQUE — batch fresh-context critics** (`name=critic-<group>`, slow tier), one per 3–4 related drafts, after all their designers finish. Write `/workspace/brief/critic-brief.md` first: verdict format (`PASS`/`CHANGES_NEEDED` + numbered blocking findings + non-blocking notes, one file per draft at `/workspace/critiques/<skill-name>.md`), and the four axes — repo-convention fidelity, end-to-end runnability with real inputs, execution-boundary respect, and encodes-the-document's-specifics-not-generic-advice. Critics read the same briefs and document; they must not edit drafts.

7. **FIX — one round, capped.** Group `CHANGES_NEEDED` drafts into batches of ≤3 sharing context; one medium-tier fixer child per batch edits the drafts in place addressing every blocking finding, then one fresh re-critic child per batch re-verdicts. Two rounds total including the first critique; a draft still failing lands anyway with its gap recorded in `COVERAGE.md` — an honest known-gap beats an unbounded loop.

8. **LAND.** The orchestrator itself copies accepted drafts into `/workspace/repo/<skills-dir>/<skill-name>/SKILL.md`, updates the repo's skills README table (matching its existing format), writes `COVERAGE.md` (one row per source activity: description, covering skill, or the merge/exclusion reason), and any chaining doc the source document's stage structure calls for. Commit as one logical commit on the branch, author `Pipeline <pipeline@local>`, via the same git `sandbox_script` mechanism as step 1.

9. **Deliver.** In the orchestrator's own process: `cp -a /workspace/repo /out/repo`; write `/out/CHANGES.md` (branch, base commit, commit range, roster, caveats), `/out/COVERAGE.md` (copy), `/out/SUMMARY.md` (per-skill one-liners, child-call count, follow-ups needed). Print `DONE`.

## Writing the orchestrator prompt

Brief it as a complete document:

1. **The source document** — what it is, where mounted (`/in/<doc>`), and how activities are marked (numbered list, per-stage exercises, checklists). If activity extraction is ambiguous, say who resolves ambiguity (orchestrator judgment + a COVERAGE.md note).
2. **The target repo** — where mounted, which directory holds skills, which files define its conventions. State the dirty-checkout rule from Watch out if applicable.
3. **The decomposition latitude** — merging/splitting is expected, every departure from one-skill-per-activity needs a stated reason in COVERAGE.md.
4. **The execution boundary** — which host services are and are not reachable from sandboxes in the deployment environment, so designers mark activities touching them as hybrid (sandboxed drafting + host-side finishing with illustrative tool names and a manual fallback).
5. **The pipeline contract** — steps 1–9; designer/critic/fixer separation (no self-review); the 2-round cap; the check-the-file-exists rule for every child's claimed output.
6. **Budget** — a hard ceiling on total child calls, tracked in `/workspace/budget.md`; on approach, stop expanding scope and land what exists with an honest COVERAGE.md cutoff note.
7. **The output contract** below, verbatim.

## Output contract

```
/out/
  repo/            # full .git repo; host fetches pipeline/<short-name>-skills from here
  CHANGES.md       # branch, base commit, commit range, what was produced, caveats
  COVERAGE.md      # one row per source activity → skill / merge reason / exclusion reason
  SUMMARY.md       # final roster with one-liners, child-call count, human follow-ups
```

## Launching the orchestrator

- **`directories: ["<abs path to source doc dir>", "<abs path to skills repo>"]` — both mandatory.** The document mount missing means nothing to decompose; the repo mount missing means nowhere to land.
- Tier: **slow** for the orchestrator, designers, and critics (design quality is the product); **medium** for fixers; the highest available tier for the few hardest designs.
- Child names: `design-<slug>`, `critic-<group>`, `fix-<group>`, `recritic-<group>`, `research-<topic>` — all DNS-1123, ≤40 chars.

## Host-side landing

Read `/out/CHANGES.md`, then `git -C <repo> fetch <output_dir>/repo pipeline/<short-name>-skills` and `git merge --ff-only FETCH_HEAD` (fallback: the repo may be under `<output_dir>/child/<name>/repo` if a child did the copy). Re-author commits (`git commit --amend --reset-author --no-edit`, or rebase with `--exec` for a range). Spot-read 2–3 landed skills against their source activities before merging to main — the critics enforced conventions, but whether the library is worth shipping is a human call.
