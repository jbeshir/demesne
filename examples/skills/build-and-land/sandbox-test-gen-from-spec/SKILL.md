---
name: sandbox-test-gen-from-spec
description: Generate and validate tests from an approved specification without changing product behavior.
---

# Test generation from specification

1. Enumerate `/in`, excluding `previous-jobs`; require exactly one mounted repository directory and copy it to `/workspace/repo`. Split the specification into independent shards and assign unique valid child names.
2. Dispatch WRITE jobs with `background:true`, at most four at once. Each writes nonempty `/out/write-<n>-summary.md`. Dispatch GATE jobs with the identical background, collected-job-id, rolling-window, and wait contract; each writes nonempty `/out/gate-<n>.json`.
3. For every job, re-wait while running. Consume it only on `succeeded` plus `exit_code=0` and its required artifact. Retry once with a new name; then cancel stuck dependents, quarantine that shard, retain stderr, and write the reason in `/out/FAILURES.md`.
4. TRIAGE reads only validated WRITE/GATE artifacts, rejects tests that alter product behavior, and records uncovered requirements. The parent copies accepted child summaries to its `/out` and copies the resulting repository to `/out/repo`.

## Outputs

Write `SUMMARY.md`, `VALIDATION.md`, `FAILURES.md`, and `/out/repo`. Require host capability for agent, script, wait, status, and cancel operations; do not rely on provider-specific frontmatter allowlists.
