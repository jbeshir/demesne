---
name: sandbox-pmf-diagnostic
status: alpha
description: "Assess early product-market-fit evidence and produce an auditable diagnostic or survey kit. Use with survey, usage, retention, and activity data; do not use it to deploy or send a survey."
allowed-tools: mcp__demesne__sandbox_agent, mcp__demesne__sandbox_script, mcp__demesne__sandbox_wait
---

Use host-default models. Required capabilities are agents, deterministic scripts, waits, and parent-side copies.

## Procedure

1. Enumerate `/in/*` and resolve the launch-labelled survey, usage/retention, founder-activity, and optional cycle mounts by content; fail on ambiguity. If no survey responses exist, write only `/out/survey-kit.md` and `SUMMARY.md`; do not send or deploy it.
2. Normalize inputs deterministically into `/workspace/normalized.jsonl`, logging dropped columns/rows. Define active as at least two uses in 14 days unless the launch prompt supplies a recorded alternative. Compute the Sean Ellis score as very-disappointed responses divided by active non-N/A respondents; mark samples under 30 `not-reportable`.
3. Dispatch `qual` and `effort` in the background after accepted normalization and `score-calc`; each writes `/out/<artifact>`. For every child, wait while running; accept only succeeded, exit code zero, and its declared output. Retry once as `<name>-r2`; otherwise write `/out/FAILURE.md` and stop dependent stages. Slug cohort names to lowercase `[a-z0-9-]`, trim edge hyphens, cap the complete name at 40 characters, and append a stable numeric disambiguator on collision.
4. Dispatch `bull` and `bear` only after all analyses pass. Supply their explicit accepted artifact paths, not assumed mount paths. Apply the same gate, then dispatch fresh `judge`; require `verdict.md` before delivery.
5. Copy accepted child artifacts into `/out/supporting/`. Write `pmf-scorecard.json` and `SUMMARY.md` with mode, denominator, sample adequacy, jobs, retries, and gaps. The verdict is `PMF-pattern`, `not-yet`, or `false-positive-warning`; it must say that one cycle cannot confirm a pattern and preserve dissent.

`survey-kit.md` contains the four questions, active-user definition, 40–100 target sample, and usage-triggered/once-per-user guidance. It is a deliverable only; external deployment requires a separate explicit authorization.
