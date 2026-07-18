# Targeted / sequential

Pipelines that route, probe, or sweep — sequential by construction rather than a broad fan-out.

| Skill | What it does |
|-------|--------------|
| [`sandbox-routing-triage`](sandbox-routing-triage/) | Classify a heterogeneous batch and dispatch each item to a specialist sub-pipeline, low-confidence items quarantined. |
| [`sandbox-bisect-hunt`](sandbox-bisect-hunt/) | Binary-search the commit / file / flag / version that introduced a regression, fresh sandbox per probe. |
| [`sandbox-benchmark-runner`](sandbox-benchmark-runner/) | Sweep a parameter grid with deterministic `sandbox_script` runs, rank the configurations. |

See the [top-level skills README](../README.md) for the shared frontmatter format, symlinking, and the concurrent fan-out loop.
