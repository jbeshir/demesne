---
name: sandbox-moat-test-suite
status: alpha
description: "Turn observed vertical edge cases into fixture-backed scenario tests and a moat map. Exclude generic coverage work and production-code changes."
---

Deliver `/out/repo`, `CHANGES.md`, `REVIEW.md`, `MOAT-MAP.md`, and `ROUTED.md`.

## Control contract

Enumerate `/in`, identify exactly one repository and one corpus by manifest inspection, and record actual paths in `/workspace/intake.md`; stop with `INPUT_AMBIGUITY.md` on ambiguity. Copy the selected repository with `cp -a <discovered-repo>/. /workspace/repo`; preserve the supplied Git/worktree state and do not invent Git metadata.

Choose one `sandbox_script.image` from `node`, `python`, `go`, `anaconda`, `browser`, `media`, `twine`, or `webgamedev` by inspecting the repository’s native test toolchain. Use `egress: none` unless a named lockfile-backed dependency is absent. If no supported image can run the native tests, write `/out/UNSUPPORTED_TOOLCHAIN.md` and stop.

Run nested workers synchronously. A phase requires `succeeded`, exit code 0, and nonempty declared output; retry once, then quarantine it and stop before any dependent build. Parent copies accepted child output to its declared `/out` destination. Omit model unless the host supplies a valid concrete value.

## Procedure

1. Write `surface.md` with vertical, native test command, chosen image, test/fixture paths, and concern taxonomy. Extract candidates to JSONL: `{id,source,observed,vertical_concern,expected_behaviour,generic_failure_hypothesis}`; log unparseable corpus files.
2. Fresh judges classify every candidate `MOAT`, `GENERIC`, or `UNCLEAR` using: “would a competent horizontal competitor without this vertical exposure get it wrong, and does correct behavior require vertical knowledge?” Build only MOAT; route the other two with reasons.
3. Write one isolated worktree per accepted build shard. Add only tests and realistic fixtures; use the repository’s native comment syntax for annotations while retaining payloads `moat: <reason> [concern=<...>] [src=<...>]`. Record the selected syntax in `tests/moat/ACCRETION.md`. Never edit production code.
4. Gate each shard with the selected script image and native focused test command. `gate.json` records `{name,pass,loads_fixture}`. Drop tests that do not load a real fixture or are tautological. Keep `pass:false` cases as `moat_gap: product fails this today`.
5. Fresh review rejects generic, unsupported, or out-of-scope changes. Revise once only. Commit surviving test-only changes to a uniquely named branch based on the mounted HEAD; if Git metadata is unusable, write `GIT_UNAVAILABLE.md` and deliver the tested worktree without claiming a branch.
6. Parent copies `/workspace/repo` to `/out/repo`, writes maps/routes/review/changes, and lists quarantines and gaps.

Mount absolute repository and corpus directories; resolve agent/script capabilities and any concrete model against the active host.
