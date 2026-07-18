# Build / land code on a branch

The orchestrator commits to a branch in `/out/repo`; the host lands it with `git fetch` + ff-merge.

| Skill | What it does |
|-------|--------------|
| [`sandbox-feature-work`](sandbox-feature-work/) | One substantial change: research → plan → numbered phases → in-sandbox `make validate` gate → review/fix → branch. |
| [`sandbox-migration-sweep`](sandbox-migration-sweep/) | One specified edit applied to N similar files in parallel, each in its own git worktree, per-shard verify, failures quarantined. |
| [`sandbox-test-gen-from-spec`](sandbox-test-gen-from-spec/) | Backfill tests for existing undertested code; per-unit writers gated on coverage delta, tautologies dropped. |
| [`sandbox-quality-improvement`](sandbox-quality-improvement/) | Audit-and-fix loop against a deterministic gate. |

See the [top-level skills README](../README.md) for the shared frontmatter format, symlinking, and the concurrent fan-out loop.
