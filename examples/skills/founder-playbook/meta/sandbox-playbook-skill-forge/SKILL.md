---
name: sandbox-playbook-skill-forge
status: alpha
description: "Convert a prescriptive playbook into reviewed, committed Demesne skills and coverage evidence. Use only with a valid Git repository and explicit integration approval."
---

# Playbook skill forge

The host gives one orchestrator this complete procedure, absolute source-document and repository directories, their basenames, an explicit current-practice-research flag, and an explicit integration flag. Omit `model` unless the host provides a concrete valid model.

## Procedure

1. Enumerate `/in`, excluding `previous-jobs`; identify exactly the supplied document and repository basenames. Fail to `/out/INCOMPLETE.md` on zero or ambiguous matches. Verify `git -C "$MOUNT" rev-parse --is-inside-work-tree` and `rev-parse HEAD`. If either fails, write a plain draft bundle and `GIT-UNAVAILABLE.md`; do not claim a branch or commit.
2. For a valid repository, reconstruct `/workspace/repo` from the mount with `cp -a "$MOUNT"/. /workspace/repo`; use `git -C /workspace/repo checkout -b pipeline/<short-name>-skills`. Read the source document, repository conventions, and 2–3 relevant exemplars.
3. Create the roster and `COVERAGE.md`. Merge activities only when they share one output and execution boundary. Split only when one activity contains a separately authorized host-side action. Record source line ranges and every ambiguity, merge, split, or exclusion in `COVERAGE.md`.
4. Write conventions and assignments. Dispatch research only when the host flag is enabled and a roster entry requires time-sensitive external facts; ask a bounded question and copy successful findings into `/workspace/brief/research/` before dependent design. Otherwise do not research.
5. Dispatch one `design-<slug>` child per roster entry, maximum eight in flight. A designer writes exactly `/workspace/drafts/<skill>/SKILL.md`. Escalate only entries whose assignment requires more than two source sections or a hybrid boundary; record the reason in `COVERAGE.md`.
6. For every job, repeat `sandbox_wait` while status is running. Accept only `succeeded`, `exit_code == 0`, and the declared nonempty artifact. Preserve diagnostics, retry once under a fresh `-r2` name, then record a coverage gap. Do not silently substitute failed output.
7. Critique related drafts in batches of at most four. Critics write one verdict per draft. A twice-failing draft is accepted-with-known-gap only when it has no safety, tool-contract, or output-contract blocker; otherwise exclude it and record the gap. Fixers do not review their own work.
8. If Git is valid, copy accepted drafts into `/workspace/repo`, update the existing skills index, write `COVERAGE.md`, and add a chaining document only when the source has two or more ordered stages with distinct output handoffs. Commit once as `Pipeline <pipeline@local>`. The orchestrator, not a child, copies `/workspace/repo` to `/out/repo`.
9. Write `/out/CHANGES.md`, `/out/COVERAGE.md`, and `/out/SUMMARY.md`. Verify the contract files and, when Git was valid, `/out/repo` are nonempty before `DONE`.

## Output contract

```text
/out/repo/                         # only after valid Git staging
/out/CHANGES.md
/out/COVERAGE.md
/out/SUMMARY.md
/out/INCOMPLETE.md                 # failed coverage, when applicable
/out/GIT-UNAVAILABLE.md            # invalid Git input, when applicable
```

## Host handoff

When integration is enabled and `/out/repo` exists, fetch `pipeline/<short-name>-skills` from that exact directory and review it before merge. A missing orchestrator-owned `/out/repo` is a failed output contract; do not search child output paths.
