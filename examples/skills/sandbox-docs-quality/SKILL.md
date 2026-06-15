---
name: sandbox-docs-quality
description: Audit a project's documentation against the code as ground truth — an orchestrator fans out four fixed lenses (completeness vs code, accuracy vs code, placement and information architecture, cross-doc consistency), each cross-referencing the docs against the source, and synthesises per-lens reports plus a summary with an information-architecture recommendation. Catches docs that understate a capability the code supports, contradict the code, bury operational requirements in the wrong place, or state facts inconsistently. Report-only — surveys and proposes, does not rewrite or land changes. Apply when the user wants to know whether the docs correctly and completely describe what the code does — "docs quality pass", "audit the docs against the code", "are the docs complete and accurate". Skip for AI-slop in prose (use sandbox-prose-defect-survey), regenerating a README to match code (use uplift-docs), and code-logic flaws (use sandbox-code-defect-survey).
---

Audit whether a project's documentation correctly, completely, and coherently describes what the code actually does. The host launches a slow-tier orchestrator, which fans out four concurrent medium-tier lens agents over docs + code, then synthesises per-lens reports and an executive summary. Report-only — no edits, no commits, no branch.

Use `sandbox-prose-defect-survey` to judge how text is written; use `uplift-docs` to regenerate docs from code; use `sandbox-code-defect-survey` to audit code logic — its docs/code-alignment lens is a side-check, whereas this skill is a dedicated, thorough docs audit. This skill checks whether the docs are true and complete against the code. No web research is needed — ground truth is the repo itself.

**Watch out:** Verify the top load-bearing findings against the cited file:line in your host session before routing them into a fix pass; lens children are medium tier and high-severity findings (especially on live wire surfaces) merit re-reading by the host. All analysis runs inside demesne; the host's only role is reading `/out`.

## Procedure

1. **Author the orchestrator prompt** (see [Writing the orchestrator prompt](#writing-the-orchestrator-prompt)). The prompt must name the doc surfaces and, critically, the **code ground-truth map** — where authoritative facts live (tool registration, provider/agent/model enumerations, env/config loader, output format code). Without the code map the lenses catch only internal inconsistency and miss capability and accuracy gaps.

2. **Launch a slow-tier `sandbox_agent` as the orchestrator** with `directories: ["<abs path to repo>"]` — the repo mounts at `/in/<repo>` (read-only) and lens children inherit the same mount. Without `directories:` the orchestrator wakes with no repo.

3. **Fan out four medium-tier `sandbox_agent` children** — one per lens — named `detect-<lens>` (lowercase DNS-1123: letters, digits, interior hyphens, ≤40 chars). All four run concurrently; four is the fixed count, not a tunable batch. Each child reads both docs and code from `/in/<repo>`, reports findings with **file:line for both the doc claim and the code ground-truth**, and writes its own `/out/REPORT.md`. "Clean on this axis" is a valid result — no manufactured findings.

4. **Collate and synthesise.** `cp` each child's report directly: `/out/child/detect-<lens>/REPORT.md` → `/out/reports/NN-<lens>.md`, then write `/out/EXECUTIVE_SUMMARY.md`. The orchestrator must do the `cp` itself — a `sandbox_script` child writes only to its own `/out/child/<name>/` subtree and strands the reports there. The executive summary must include the **information-architecture recommendation** — required output, not optional polish; the highest-leverage result is usually "operational requirements have no coherent home → create one and link everything to it."

## The four lenses

Each child reads BOTH docs and code. Report format: `# <lens>` → `## Summary` → `## Findings` (each: doc location + code ground-truth + gap + severity + proposed fix) → `## Improvement plan`.

1. **completeness-vs-code** — every capability the code supports reflected in docs? All tool params, providers/agents/models/modes/options, env vars, config fields, output fields documented? Flag understated, narrowed, or partial descriptions; propose completing text.
2. **accuracy-vs-code** — is what IS documented correct? Wrong defaults, stale counts, removed/renamed features, changed behaviours, example output that no longer matches. Cite doc line + code line.
3. **placement / information architecture** — is each fact in the right doc type (Diátaxis: tutorial = learning; how-to = task; reference = lookup; explanation = concepts) and discoverable? Explicitly assess where operational requirements/prerequisites/config live and propose a concrete home.
4. **consistency** — same fact/term/requirement stated consistently across docs and cross-referenced where relevant? Flag divergences and facts that are single-homed when they should be linked from several places.

## Writing the orchestrator prompt

Brief it as a complete document:

1. **The target and its docs** — what the project is, the doc surfaces (Diátaxis tree, READMEs, `manifest.json`/tool metadata, schemas), and the **code ground-truth map**: tool registration + descriptions + output formatting, supported providers/agents/models, env/config loader, egress/mode enums, image allowlist.
2. **The four lenses** above, each cross-referencing docs vs code with file:line for both sides.
3. **The two canonical archetypes to seed** — (a) *capability narrowing*: a feature described as a subset of what the code supports; (b) *operational-requirement misplacement*: a prerequisite buried in a tutorial step and not discoverable elsewhere.
4. **Detection discipline** — read-not-skim; cite both doc and code; confirmed vs suspected; "clean on this axis" is valid; no manufactured findings.
5. **The synthesis must include an IA recommendation** — where operational requirements/config should consistently live and how tutorials/READMEs/how-tos should link to it.
6. **Output contract** — report-only, no edits.

## Output contract

```
/out/
  EXECUTIVE_SUMMARY.md            # findings table + archetypes confirmed + IA recommendation + prioritised fix list
  reports/
    NN-<lens>.md                  # one per lens (completeness, accuracy, placement, consistency)
```

The executive summary carries: findings table (lens | confirmed | suspected | top severity | headline); canonical archetypes confirmed with full extent; cross-cutting themes; concrete IA recommendation; prioritised fix list pointing at the reports.

## Launching the orchestrator

- **`directories: ["<abs path to repo>"]` is mandatory** — lens children inherit this mount and read both docs and code from `/in/<repo>`.
- Tier: **slow** for the orchestrator; **medium** for lens children.