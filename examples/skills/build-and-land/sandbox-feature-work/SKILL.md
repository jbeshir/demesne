---
name: sandbox-feature-work
description: Implement and land a bounded feature change in an existing repository.
---

# Feature work

Use this only when the caller authorizes a repository change and landing review.

## Procedure

1. Mount the repository with the host `directories` argument (a workflow requirement, not a tool requirement). In the orchestrator, list `/in`, exclude `previous-jobs`, require exactly one directory, copy it to `/workspace/repo`, and fail on zero or multiple candidates. Preserve `.git` if it is usable; otherwise record Git provenance as unavailable and do not fabricate it.
2. Write `/workspace/plan.md` with the requested behavior, files in scope, and one acceptance test per requirement. Spawn uniquely named children (`[a-z0-9]([-a-z0-9]*[a-z0-9])?`, at most 40 characters). Omit `model` to use the configured default, unless the host supplies a valid concrete provider model.
3. Have `implement-1` change `/workspace/repo`, write `/out/SUMMARY.md`, and run no landing operation. For every background job, call `sandbox_wait` with its long default; repeat only if it returns `running`. Advance only when `status=succeeded`, `exit_code=0`, and the required nonempty artifact exists. On failure/cancellation/missing output, retry once with a new name; then stop, preserve stderr, and write `/out/FAILURE.md`.
4. Run the repository's existing full validation target in a `sandbox_script` using its language image and `egress=none`. Install tools only through the repository's pinned mechanism; reject unpinned or `@latest` installs. Record the exact command and result in `/out/VALIDATION.md`. If dependencies are absent, use `egress=package-managers` only for the named pinned install.
5. Spawn `review-1` after implementation. Require `/out/REVIEW.md` with pass/fail findings. Apply at most one fix/review cycle; use the same barrier and retry rule. Do not create a commit unless the caller requested it and the usable worktree permits it.
6. The orchestrator copies child artifacts explicitly: `cp /out/child/implement-1/SUMMARY.md /out/SUMMARY.md`, `cp /out/child/review-1/REVIEW.md /out/REVIEW.md`, and `cp -a /workspace/repo /out/repo`. Missing copies are failures, not fallback paths.

## Outputs

Return `/out/repo`, `SUMMARY.md`, `REVIEW.md`, `VALIDATION.md`, and either `FAILURE.md` or `BACKLOG.md`.
