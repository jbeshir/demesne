---
name: sandbox-migration-sweep
description: Apply a bounded, independently verifiable migration across an existing repository.
---

# Migration sweep

1. Mount the repository, enumerate `/in` excluding `previous-jobs`, require one directory, copy it to `/workspace/repo`, and record its base revision when Git works. For more than 200 targets, stop and require the caller to split the request. Omit `model` unless the host supplies a valid concrete model.
2. Partition at most 200 targets into shards of at most 25 files. For each shard create `/workspace/wt-<shard>` with `git worktree add -b shard-<shard> /workspace/wt-<shard> <base>`; if Git is unavailable, use isolated copies and record that no merge/commit is possible.
3. Dispatch at most four editors at a time. Each writes `/out/CHANGES.md` and `/out/SUMMARY.md`. Wait in a loop (≤120 seconds/call); accept only `succeeded`, `exit_code=0`, and nonempty artifacts. Retry once with a unique name; then cancel dependents, quarantine the shard, preserve stderr, and record it in `FAILURES.md`.
4. Verify each non-quarantined shard with an explicit language image. Use `egress=none` only if all dependencies are provisioned; otherwise use `package-managers` solely for pinned dependencies. Require a zero exit code and `VALIDATION.md` before merge.
5. Merge only validated shards. The orchestrator copies every accepted child artifact and `/workspace/repo` into its own `/out`; missing `/out/repo` is failure. Use `N=targets`, `M=changed`, `K=quarantined` in both reports.

## Outputs

Write `/out/repo`, `CHANGES.md`, `SUMMARY.md`, `VALIDATION.md`, and `FAILURES.md`. The host reads `<output_dir>/…`, never an in-sandbox `/out` path.
