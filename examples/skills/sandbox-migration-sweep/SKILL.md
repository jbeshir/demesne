---
name: sandbox-migration-sweep
description: Apply ONE specified edit operation — a codemod, rename, dependency-version bump, config-file rewrite, doc-frontmatter migration, license-header insertion, deprecated-API replacement — to N similar target files in parallel across a repo. A slow-tier orchestrator enumerates targets, shards them (≤ 20 files/shard), fans out parallel medium-tier editors each in its own git worktree, runs per-shard verification, quarantines failing shards, merges survivors onto an integration branch, and delivers /out/repo for the host to fetch. Apply when one well-specified edit must land on many similar files. Triggers include "apply this codemod across the repo", "bump X to version Y everywhere", "rename this API across all packages", "add license headers to all Go files", "replace all uses of deprecated Foo with Bar", "migrate all config files to the new format", "sweep the repo with this rule". Skip for a single substantial code change (use sandbox-feature-work), an open-ended quality audit (use sandbox-quality-improvement), and regenerating docs to match code (use uplift-docs).
allowed-tools: Read, Glob, Grep, Bash, Write, Edit, mcp__demesne__sandbox_agent, mcp__demesne__sandbox_script
---

Apply one well-specified edit operation to many similar targets through a demesne pipeline. The host mounts the repo and launches a slow-tier orchestrator; the orchestrator enumerates targets, shards them, fans out medium-tier editors each in its own git worktree, verifies per shard, quarantines anything that fails, merges survivors onto an integration branch, and copies the committed repo to `/out/repo`. The deliverable is a branch, not a patch — same shape as `sandbox-feature-work`.

**Watch out (cross-cutting):** the orchestrator must `cp` final artefacts to its own `/out` itself; a `sandbox_script` child writes only to `/out/child/<name>` and would strand the repo there. Quarantine is non-negotiable — never silently land a shard whose verifier returned non-zero.

## Procedure

1. **Stage the repo.** From the orchestrator: `cp -a /in/<repo>/. /workspace/repo`. The `-a` flag preserves `.git`; without it there are no branches to cut, no diffs, no merge step.

2. **Write the spec at `/workspace/migration.md`.** Required sections: the rule as a before→after transformation with 2–3 concrete examples; the target file pattern (glob or `grep` expression); the exclusion list (auto-generated files, vendored code, test fixtures unless explicitly in scope); and the per-shard verification command. Spec precision is the biggest predictor of sweep quality — an ambiguous rule produces divergent edits across shards. Over-specify the rule.

3. **Enumerate.** Run `find`/`grep` via Bash against `/workspace/repo` to produce `/workspace/targets.jsonl` (one record per file: `{"path":"<abs path>","shard":<n>}`). Chunk into S shards of ≤20 files; write assignments to `/workspace/shards.jsonl`. If N > 200 files, stop and propose splitting the sweep across multiple runs (per subsystem or directory) rather than fanning out 15+ simultaneous editor agents, which stresses sandbox infrastructure and makes failure diagnosis harder.

4. **Edit, one medium-tier `sandbox_agent` per shard.** Spawn with `name=edit-shard-NN` (lowercase DNS-1123 — letters, digits, interior hyphens, ≤40 chars; bad names produce invalid volume names and poison sibling spawns), in batches of ≤4 concurrent; wait for each batch to complete before spawning the next. Four is a recommended batch size — demesne enforces no cap, but beyond four the MCP keepalive pressure on nested sandboxes degrades stability. The editor's prompt MUST begin with worktree setup:
   ```
   git -C /workspace/repo worktree add /workspace/wt-<shard> shard/<shard>
   ```
   The editor then reads `/workspace/migration.md`, applies the rule to every file in its shard inside `/workspace/wt-<shard>` (never in `/workspace/repo` directly — two editors writing to `/workspace/repo` corrupt the shared index), commits on `shard/<shard>` authored as `Pipeline <pipeline@local>`, and writes its own `/out/SUMMARY.md` listing files touched.

5. **Verify, one `sandbox_script` per shard.** Use `image=<lang>` (`go`, `node`, `python`, `anaconda`) and `egress=none` when the project pins tooling (`go.mod tool`, `package-lock.json`, `poetry.lock`) so sandbox and host run identical versions; use `egress=package-managers` only when dependencies must install at verify time. Run the verification command from `/workspace/migration.md` against the shard's worktree; verify at the shard level (Go package, JS module, Python suite), not per file — a package that spans multiple files fails to build when checked file-by-file.

   On exit 0 the shard survives. On non-zero: move shard files to `/workspace/quarantine/<shard>/`, copy verifier stderr to `/workspace/quarantine/<shard>/verifier.log`, and append to `/workspace/quarantine.jsonl`: `{"shard":"<id>","files":[…],"reason":"<first 500 chars>"}`. Write `/workspace/quarantine.jsonl` even when empty — it is evidence the step ran; an absent file is ambiguous.

6. **Land.** Merge surviving shard branches into `pipeline/migration-<short>` in `/workspace/repo` with `git merge --no-ff shard/<n>` in order. Then, in the orchestrator's own process, `cp -a /workspace/repo /out/repo`. Do not delegate this copy to a `sandbox_script` child: that child's `/out` is `/out/child/<name>/`, so the repo would land there instead and the host could not find it. `/workspace` is torn down when the orchestrator exits; only `/out` persists.

   Write `/out/CHANGES.md`: branch name, base commit, surviving shards with file counts, quarantined targets with reasons, total counts (N targets / M changed / K quarantined), and the full text of `/workspace/migration.md`.

7. **Report.** Write `/out/SUMMARY.md`: changed vs. total counts, quarantined targets grouped by reason, and manual-review next steps for each quarantined group (including how to re-run the verification command locally).

## Writing the orchestrator prompt

Brief it as a complete document:

1. **The migration rule** — exact edit, 2–3 before→after examples, target pattern, exclusion list, verification command. "Update the API" produces inconsistent edits; spell out the textual transformation.
2. **The repo path** — `/in/<repo>` (matched to your `directories:` mount); tell it to `cp -a /in/<repo>/. /workspace/repo`.
3. **The pipeline contract** — the seven steps above, the ≤4 concurrent editor batch, worktree-per-shard, quarantine on non-zero verifier.
4. **The verification command exactly** — image and self-contained command (e.g. `cd /workspace/wt-<shard> && go build ./...`). The verifier sandbox has no host environment.
5. **The shard cap** — if N > 200 files, warn and split the run.
6. **Author identity** — commit all changes as `Pipeline <pipeline@local>`; the host re-authors after fetching.
7. **Output contract** — `/workspace/migration.md`, `/workspace/targets.jsonl`, `/workspace/shards.jsonl`, per-editor `/out/SUMMARY.md`, `/workspace/quarantine.jsonl` (written even if empty), `/out/CHANGES.md`, `/out/SUMMARY.md`, and the branch in `/out/repo`.

Over-specify the rule; under-specify the implementation details.

## Output contract

```
/out/
  repo/          # full .git repo; host fetches pipeline/migration-<short> from here
  CHANGES.md     # branch name, base commit, surviving shards, quarantined targets
                 # + reasons, total counts (N targets / M changed / K quarantined),
                 # full migration.md spec
  SUMMARY.md     # N changed of M targets; K quarantined with reason breakdown;
                 # manual-review next steps
```

Workspace artefacts (not in `/out` but the orchestrator reads them):
```
/workspace/
  migration.md       # the spec
  repo/              # working copy with .git
  targets.jsonl      # enumerated targets
  shards.jsonl       # shard assignments
  wt-shard-<n>/      # per-shard git worktrees
  quarantine/        # per-shard quarantined files + verifier.log
  quarantine.jsonl   # machine-readable quarantine log (written even if empty)
```

## Launching the orchestrator

- **`directories: ["<abs path to repo>"]` is mandatory.** Without it the orchestrator wakes with no repo, stalls diagnosing the empty mount, and produces nothing. Verify it on every launch.
- Tier: **slow** for the orchestrator; **medium** for editor children (the orchestrator sets this).
## Host-side landing

The per-shard in-sandbox verifier is the authoritative gate; host work is deliberately minimal — land the branch and confirm, never re-run the sweep.

1. Read `/out/CHANGES.md` for the branch name, base commit, and quarantine list.
2. Fetch and fast-forward: `git -C <repo> fetch <output_dir>/repo <branch>` then `git -C <repo> merge --ff-only FETCH_HEAD`. If `<output_dir>/repo` is absent, the committed repo is under `<output_dir>/child/<name>/repo` (the orchestrator routed the copy through a child) — fetch from there.
3. Re-author the landed commits: `git commit --amend --reset-author --no-edit` for a single commit, or `git rebase <base> --exec "git commit --amend --reset-author --no-edit"` for several. The base commit is named in `/out/CHANGES.md`.
4. Run one backstop check — the project's own gate (`make validate`, or whatever the per-shard verifier ran). With pinned tooling it passes first time; a failure signals an environment gap, not routine churn. Read the log, not just the exit code.
5. Triage the quarantine. Every target in `/out/CHANGES.md`'s quarantine list needs a manual pass — fix by hand or run a narrower sweep. Never merge quarantined targets blind.

The host does not re-apply the migration rule; the verified branch is the deliverable.
