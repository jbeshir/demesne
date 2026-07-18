---
name: sandbox-benchmark-runner
status: alpha
description: "Run a bounded benchmark design and deliver validated ranked results without changing product code."
---

Benchmark only a command and metric supplied by the host. Do not infer benchmark validity from successful execution.

1. The host mounts every needed code, data, and config path. The orchestrator enumerates `/in`, ignores `previous-jobs`, binds the declared mounts by basename and content, and stops with `/out/REPORT.md` `status: input-invalid` if any is absent or ambiguous. Copy mutable inputs to `/workspace`.
2. Write `/workspace/design.md` and `/workspace/runs.jsonl`: objective, direction, parameter grid or a maximum 40-run adaptive plan, command, metric schema, image, and egress. A run writes `/out/metrics.json` containing configuration, metric values, unit, command outcome, and seed. Use a host-supplied concrete model only where a synthesis agent is needed; otherwise omit `model`.
3. Dispatch `run-<NN>` scripts at most 8 in flight. Wait until terminal and accept only `succeeded`, exit 0, nonempty valid `metrics.json`, and a metric matching the schema. Retry once as `run-<NN>-retry`; then append a failed record with diagnostic to `/workspace/results.jsonl`. Never drop a planned run.
4. After every run is accepted or recorded failed, spawn `synthesiser`. It reads the accumulated results and writes `/out/RANKED.md`, `/out/REPORT.md`, and `/out/results.jsonl`. Require terminal success, exit 0, nonempty artifacts, valid JSONL, and a one-retry `synthesiser-retry` gate. On failure, copy the raw results and write an incomplete report instead of claiming a recommendation.
5. Copy accepted synthesiser artifacts from `/out/child/<name>/` to parent `/out`. The report lists failed runs, exact coverage, and reproducibility limits.

## Output contract

```
/out/RANKED.md
/out/REPORT.md
/out/results.jsonl
```
