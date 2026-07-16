---
name: sandbox-metrics-framework
status: alpha
description: "Build an auditable MVP metrics framework with definitions, scoring, and false-positive checks. Use before making product decisions from early metrics; do not use it to invent data."
allowed-tools: mcp__demesne__sandbox_agent, mcp__demesne__sandbox_research, mcp__demesne__sandbox_script, mcp__demesne__sandbox_wait
---

Use host-default models; never pass narrative tiers as a `model` value. Required capabilities are background agents/research, scripts, waits, and parent-side copies.

## Procedure

1. Write the product, core action, available exports, and metric decision to `/workspace/brief.md`. Launch generators and any research child with `background:true`; each declares its exact `/out/<artifact>`.
2. For every job, call `sandbox_wait` repeatedly while `running`. Accept only `status == succeeded`, `exit_code == 0`, and every declared file exists. Retry once with a distinct `-r2` name. On a second failure, write `/out/FAILURE.md` naming the missing contribution and stop dependent work; never treat failed or cancelled work as evidence. A completed sibling becomes available at `/in/previous-jobs/<name>/` only after completion.
3. Validate every `scores.jsonl` row in the orchestrator before compilation: required keys are `metric`, `definition`, `formula`, `source`, `cadence`, `owner`, and `decision`; reject non-JSON rows and missing/empty strings. `output_format` describes expected output; it does not validate it.
4. Limit false-positive signatures to the three standard signatures plus at most two product-specific ones. Each extra signature must identify a plausible vanity signal unique to the core action or business model.
5. Dispatch a fresh judge only after accepted inputs exist. It writes `/out/METRICS.md`, `/out/metrics.json`, and `/out/false-positive-checks.md`; apply the same gate before delivery.
6. Copy only accepted child artifacts into parent `/out` and write `SUMMARY.md` with job states, retries, validation rejections, and input gaps.

Prompt the orchestrator with the brief, the procedure, and the output contract; do not restate the procedure in a second prompt template.
