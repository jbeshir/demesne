---
name: sandbox-prototype-sprint
status: alpha
description: Build and smoke-test a small prototype from validated discovery evidence, then deliver the worktree and review artefacts. Use only when a prototype is authorized.
---

Use native Demesne capabilities. Omit `model` unless the host supplies a concrete schema-valid value. Names are monotonically unique: `<role>-01`, then `<role>-02`; record the latest accepted name and job ID per role in `/workspace/jobs.json`.

## Procedure

1. Enumerate `/in` excluding `/in/previous-jobs`; identify required discovery evidence and repository mounts by manifest/content, then copy them to `/workspace`. On ambiguity or absence, deliver `/out/FAILURE.md` and stop. Preserve the repository's existing worktree and do not manufacture Git metadata.
2. Before dependency-dependent work, run a provisioning child with `egress=package-managers` that writes pinned dependency versions and a lockfile. If provisioning fails, deliver `FAILURE.md`; otherwise all smoke work uses `egress=none`. Alternatively, skip provisioning only for dependency-free or fully vendored inputs, and record that condition.
3. Execute numbered phases with unique child names, recording an accepted output only after `status == succeeded`, `exit_code == 0`, and its declared file exists. Retry a phase once; on second failure write `/out/FAILURE.md` and stop. Each phase writes `/out/PHASE-NN.md`.
4. Run the offline smoke gate. If it fails, produce the red-gate terminal contract: `/out/SMOKE.md`, `/out/CHANGES.md` beginning `status: RED_GATE`, `/out/FAILED`, and `/out/SUMMARY.md`; do not deliver a repo copy, reaction kit, or review artefacts.
5. If smoke passes, run the authorized implementation/review phases. The parent copies the accepted worktree into its own `/out/repo`; if `/out/repo` is absent or incomplete, report failure rather than asking a child to surface it. Do not assert a commit unless a usable Git worktree exists and the commit command succeeded.
6. Concatenate every `PHASE-NN.md` in numeric order into `/out/SUMMARY.md`, with one heading per phase, then verify all success-path outputs.

## Output contract

Success: `/out/repo`, `/out/SCOPE.md`, `/out/FINDINGS.md` when research ran, `/out/SMOKE.md`, `/out/CHANGES.md`, `/out/REVIEW.md`, `/out/reaction-test-kit/`, and `/out/SUMMARY.md`. Red gate: only the step-4 red-gate files.
