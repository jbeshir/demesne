# Spawn nested agents

When your agent is running inside a `sandbox_agent` or `sandbox_research` sandbox, demesne re-exposes its own tools so the agent can spawn child sandboxes. This page covers the conventions and gotchas for that pattern.

> **Prerequisites**: rootless podman hosts need the `fs.pipe-user-pages-soft=0` sysctl set — see [docs/reference/requirements.md §Rootless Podman pipe-page cap](../reference/requirements.md#rootless-podman-pipe-page-cap). The whole sandbox-fan-out pattern this page describes hits the default cap routinely.

## Available child tools

Inside a `sandbox_agent` or `sandbox_research` sandbox, the following demesne tools are available:

- `sandbox_script` — single-shot child shell command
- `sandbox_agent` — child AI coding agent
- `sandbox_research` — child research agent (open egress, no inputs)
- `sandbox_create` — create a persistent child sandbox
- `sandbox_exec` — run a command in a persistent child sandbox
- `sandbox_destroy` — destroy a persistent child sandbox

`sandbox_upload` and `sandbox_download` are not available to children.

## The `name` parameter

Every child-spawning call requires a `name` parameter. The name rules are enforced by `validateChildName` in `internal/sandbox/children.go`:

- **Lowercase letters, digits, and interior hyphens only.** No underscores, dots, uppercase letters, or spaces.
- **Interior hyphens only** — the name may not start or end with a hyphen.
- **Maximum 40 characters.**
- **Unique within the current sandbox.** Two children of the same parent may not share a name; the second attempt returns an error.

Valid examples: `analyzer`, `build-step`, `test-runner-1`  
Invalid examples: `Analyzer` (uppercase), `build_step` (underscore), `test.runner` (dot), `-leading` (leading hyphen), `a` × 41 (too long)

The name becomes a path segment and also part of an OpenSandbox volume name (`prevjob-<name>`), which must be a valid DNS-1123 label — hence the strict character set.

## Output directory convention

A child sandbox's output goes to:

```
<parent-output-dir>/child/<name>/
```

For example, if the parent's output dir is `/tmp/demesne/out/<parent-job-id>/out`, a child named `analyzer` writes to:

```
/tmp/demesne/out/<parent-job-id>/out/child/analyzer/
```

Grandchildren nest further under the child:

```
/tmp/demesne/out/<parent-job-id>/out/child/analyzer/child/sub-task/
```

Inside the sandbox, the parent sees its own output dir at `/out`. A child named `analyzer` therefore lands at `/out/child/analyzer/` from the parent's perspective.

## The copy-to-`/out` gotcha

A child's `/out/child/<name>` is the **child's** output directory. Files the child writes there are visible to the parent at `/out/child/<name>` (inside the sandbox) or at `<parent-out-host>/child/<name>` on the host — but they do NOT automatically appear in the parent's own `/out`.

To hand a child-produced artefact back to your caller, you must explicitly copy it into your own `/out`:

```bash
# Inside a sandbox_script or sandbox_exec run in the parent:
cp /out/child/analyzer/report.txt /out/report.txt
```

Or use another child:

```json
{
  "name": "sandbox_script",
  "arguments": {
    "name": "collect-results",
    "command": "cp /out/child/analyzer/report.txt /out/final-report.txt"
  }
}
```

Never rely on the child to copy its own results to the parent's `/out` — the child can only write to its own `/out/child/<name>` subtree.

## Completed siblings under `/in/previous-jobs`

After a child completes, its output directory is mounted read-only under `/in/previous-jobs/<name>` for all subsequently spawned siblings. This allows sibling agents to read earlier sibling outputs without going through a shared `/out`.

## Depth

Nesting depth is tracked (starting at 0 for the root run) and recorded in `results.json` as the `depth` field. There is no recursion depth cap — but each level adds latency and cost, and deeply nested trees can be hard to reason about.

## Reading child results

Each child agent run writes `results.json` and `usage.json` to its output directory. From the parent, read them at:

```
/out/child/<name>/results.json
/out/child/<name>/usage.json
```

The root run's `results.json` already sums the whole tree in `total_usage_usd`, so you rarely need to read children's `results.json` directly. See [`../reference/results-json.md`](../reference/results-json.md) and [`../reference/usage-json.md`](../reference/usage-json.md) for the field reference.
