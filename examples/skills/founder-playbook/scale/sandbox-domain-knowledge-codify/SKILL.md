---
name: sandbox-domain-knowledge-codify
status: alpha
description: "Convert recurring expert knowledge into an evidence-linked, agent-neutral operating library. Skip when evidence is insufficient or the work is a one-off answer."
---

Deliver an agent-neutral index, source map, procedures, gaps, and `SUMMARY.md` in `/out`; do not modify product code.

## Control contract

Enumerate `/in` at runtime and record actual mounts in `/workspace/intake.md`. Classify mounts as expertise evidence or an existing library only from filenames and a sampled manifest; if more than one candidate remains, emit `/out/INPUT_AMBIGUITY.md` and stop. Do not assume `/in/<expertise>` or `/in/<library>`.

Give every nested child a unique DNS-1123 `name`; do not pass invented `tier` parameters. Omit `model`, or map a host-supplied supported model before launch. For every stage require `succeeded`, exit code 0, and nonempty outputs; retry once, then quarantine its evidence/status/missing-output record. Parent delivery copies accepted artifacts into `/out`.

## Procedure

1. Make `source-map.jsonl` with source path, author/date when present, topic, evidence quality, and reuse count. Use deterministic parsing for structured files; log unparseable inputs.
2. Select elicitation mode: use extraction when at least 10 source items or 5,000 words exist; otherwise use a questionnaire that asks no more than 12 questions. Pack shards at no more than 20 source items or 12,000 words.
3. Extract only evidence-supported rules. Each candidate records `{id,topic,rule,trigger,steps,exceptions,evidence:[...],confidence:"high"|"medium"|"low"}`. Send low-confidence or conflicting candidates to `gaps.md`, never to a normative procedure.
4. Reduce accepted candidates into one procedure per topic. Each procedure has purpose, trigger, prerequisites, numbered steps, stop/escalation criteria, expected output, and evidence links. Create `INDEX.md` with topic, trigger, procedure path, owner, and last-evidence date; it is agent-neutral.
5. Audit every procedure against source citations. Revise once from accepted audit findings; unresolved conflicts remain in `gaps.md`. Write `SUMMARY.md` with sources, procedures, gaps, quarantines, and revision result.

Mount absolute host evidence and optional existing-library paths. Require agent and deterministic-script capabilities and resolve them against the active host at launch.
