# Nested sandboxes reference

When an agent is running inside a `sandbox_agent` or `sandbox_research` sandbox, demesne re-exposes its own tools so the agent can spawn child sandboxes. This page is the reference for the layout and conventions those calls follow.

> **Prerequisites**: rootless podman hosts need the `fs.pipe-user-pages-soft=0` sysctl set — see [requirements.md §Rootless Podman pipe-page cap](requirements.md#rootless-podman-pipe-page-cap). The sandbox fan-out pattern this page describes hits the default cap routinely.

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

Every child-spawning call requires a `name` parameter. Rules:

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

### The copy-to-`/out` gotcha

A child's `/out/child/<name>` is the **child's** output directory. Files the child writes there are visible to the parent at `/out/child/<name>` — but they do NOT automatically appear in the parent's own `/out`.

To hand a child-produced artefact back to your caller, you must explicitly copy it into your own `/out`:

```bash
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

The root run's `results.json` already sums the whole tree in `total_usage_usd`, so you rarely need to read children's `results.json` directly. See [`results-json.md`](results-json.md) and [`usage-json.md`](usage-json.md) for the field reference.

## Spawning a verifier/judge child

Use a second `sandbox_agent` to evaluate a worker's output against explicit criteria rather than self-critiquing in the same context window. An external judge has a fresh context and cannot rationalise away errors it did not produce.

Spawn the judge as a sibling of the worker. After the worker completes, the judge sees the worker's output at `/in/previous-jobs/<worker-name>/`. The full reasoning trace is at `/in/previous-jobs/<worker-name>/transcript.jsonl` — see [`transcript-jsonl.md`](transcript-jsonl.md).

```json
{
  "name": "sandbox_agent",
  "arguments": {
    "name": "judge",
    "preamble": "You are a strict reviewer. Do not modify files.",
    "prompt": "Read /in/previous-jobs/worker/report.md. Does it satisfy the criteria? Write PASS or FAIL followed by one sentence to /out/verdict.txt.",
    "output_path": "/out/verdict.txt",
    "success_criteria": [
      "verdict.txt exists",
      "first word is PASS or FAIL"
    ]
  }
}
```

The `output_path` and `success_criteria` params render as a `## Definition of done` block prepended to the task in the judge's context file.

## Recommended artefact layout

| Purpose | Path |
|---------|------|
| Plan / in-progress findings | `/workspace/<phase>.md` — shared scratch, visible to all siblings |
| Final artefacts | `/out/<name>` — your output directory; copy child results here explicitly |
| Sibling outputs | `/in/previous-jobs/<name>/` — read-only mount of a completed sibling |

Enumerate available previous-job mounts with:

```bash
ls /in/previous-jobs/
```

Use stable sorted names for phases — `phase01-gather`, `phase02-analyse`, `phase03-report` — so ordering is unambiguous when siblings consume one another's results.

## Writing the task prompt

Structure every child call in three layers:

| Layer | Parameter | Purpose |
|-------|-----------|---------|
| Role | `preamble` | Agent identity, must-not constraints, style rules — prepended verbatim before the auto-generated context. |
| Task | `prompt` | What to do and where to write output. |
| Success criteria | `output_path`, `output_format`, `success_criteria` | Rendered as a `## Definition of done` block before `## Task` in the child's context file. |

The `## Definition of done` block appears before the task body, so the agent reads the acceptance bar before the instructions.

Example using all three layers:

```json
{
  "name": "sandbox_agent",
  "arguments": {
    "name": "summariser",
    "preamble": "You are a technical writer. Write in plain English. Do not use bullet points.",
    "prompt": "Read /in/data.json and write a one-paragraph summary to /out/summary.txt.",
    "output_path": "/out/summary.txt",
    "output_format": "plain text, one paragraph, ≤ 200 words",
    "success_criteria": [
      "summary.txt exists",
      "word count ≤ 200",
      "no bullet points or headers"
    ]
  }
}
```

## Context management across phases

For work that would fill a single context window, split into checkpointed phases:

1. Write the plan and in-progress findings to `/workspace/<phase>.md`.
2. Spawn a fresh `sandbox_agent` referencing the checkpoint file path in its prompt.
3. Repeat as needed; each fresh agent starts with a clean context window.

A fresh window is cheaper than letting one context grow unbounded — token costs scale with context length and model reliability decreases at long contexts.
