---
name: sandbox-data-flywheel-audit
status: alpha
description: "Assess whether observed product data supports a defensible flywheel and deliver a narrative or an instrumentation plan. Do not use for switching-cost analysis or code changes."
---

Deliver `inventory.md`, `data-verdict.md`, `flywheel.md`, `collection-plan.md`, `patterns.jsonl`, conditional `moat-narrative.md` and `attack.md`, and `SUMMARY.md` in `/out`.

## Control contract

At runtime enumerate `/in`; identify exactly one data mount or stop with `/out/INPUT_AMBIGUITY.md`. Record its actual path in `/workspace/intake.md`; host source directory basenames determine mounts. Use `sandbox_script` with `image: python`, `egress: none` to profile parseable files, writing `manifest.jsonl`, `stats.json`, `unparsed.md`, and `inventory.md`. Omit `model` unless the host supplies a valid concrete value.

Invoke nested children synchronously. Accept an output only on `succeeded`, `exit_code: 0`, and a nonempty declared file. Retry a failed/cancelled/missing-output invocation once; otherwise record the stage, status, error, and missing file in `/workspace/quarantine.md`, exclude it, and state the incomplete result. Copy accepted child outputs into the parent `/out`; do not assume a child output is automatically delivered.

## Procedure

1. Profile every input file. Issue **THIN-DATA** if timestamp span is under 90 days, no stable identifier links a user over time, or fewer than 50 users appear in two distinct weeks. Issue **PARTIAL** when those thresholds pass but any intended loop lacks evidence; otherwise issue **FLYWHEEL-READY**. Write measured values and reasons to `data-verdict.md`.
2. On THIN-DATA, design a collection plan from the manifest and stats: required event, properties, stable identifier, retention, owner, and re-audit date. Do not mine, narrate, or attack; `patterns.jsonl`, `moat-narrative.md`, and `attack.md` are absent.
3. Otherwise shard parseable inputs into explicit path lists of at most 50 files or 25 MB each. Each miner writes `patterns.jsonl` records `{pattern_id,description,evidence,users_covered,time_span,signal_type:"longitudinal"|"cross-sectional",why_high_signal}` and `log.md`. Merge accepted miner files into parent `/out/patterns.jsonl`; quarantine makes the verdict incomplete rather than silently omitting a shard.
4. Design 3–5 loops from accepted patterns. Each loop names source fields, aggregation cadence, action threshold and owner, product change, success metric, and cycle time. Every signal must trace to `manifest.jsonl`. Write `flywheel.md`; on PARTIAL also write `collection-plan.md`.
5. Draft a one-page narrative only from accepted loops and measured stats. Tag every claim with a pattern, stat, or loop identifier. A fresh adversary classifies each claim `HOLDS`, `CUT`, or `CONTESTED` against purchase/scrape, generic-insight, and unwired-loop tests in `attack.md`. Revise once; retain unresolved claims only in a Contested Claims footer.
6. Write `SUMMARY.md` with verdict, profiled/unparsed counts, accepted/quarantined stages, and claim outcomes.

Launch with one absolute host data-export directory. Require agent and deterministic-script capabilities; resolve host tool names and any concrete model at launch.
