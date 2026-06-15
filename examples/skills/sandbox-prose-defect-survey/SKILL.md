---
name: sandbox-prose-defect-survey
description: Survey a project's documentation, READMEs, code comments, and generated text for the flaw types common to AI-generated prose — an orchestrator runs an open-web research child that distils a sourced taxonomy of ~10 AI-text flaw types (hallucinated claims, verbosity, terminology drift, structural bloat, example rot, broken cross-references, marketing fluff), fans out one detection subagent per type that hunts evidence-backed instances across the prose (file:line, confirmed vs suspected), and synthesises per-type reports plus a prioritised summary with an improvement plan. Report-only — surveys and proposes, does not rewrite or land changes. Apply when the user wants to know what categories of writing problems a project has — "prose defect survey", "find the AI-slop in our docs", "audit our docs for AI-text tells". Skip for code-logic flaws (use sandbox-code-defect-survey), refreshing a README (use uplift-docs), and competitive gap analysis (use sandbox-product-research).
---

Survey a project's prose — documentation, READMEs, examples, code comments, and the text the code generates — for failure modes common to AI-generated writing. You author one orchestrator prompt, launch a slow-tier `sandbox_agent`, and read the reports; the orchestrator runs the pipeline autonomously. This is the prose twin of `sandbox-code-defect-survey`; keep the split clean: factual docs/code alignment belongs to the code survey, this survey judges how the text is *written*. Differs from `uplift-docs`, which regenerates docs to match the code — a production task, not a flaw survey. Report-only; acting on findings is a separate, explicit step (rewrite by hand, or route specific changes through `sandbox-feature-work`; regenerate stale READMEs/diagrams with `uplift-docs`).

**Watch out:** Name every prose surface explicitly in each detection child's prompt or generated text (help, errors, context files) gets skipped; findings on generated text must point at the generator/template, not sample output. Re-research the taxonomy from the open web every run — never let the orchestrator reuse a canned tell-list from its priors.

## Procedure

1. **Research.** Spawn one medium-tier `sandbox_research` child (`name=research01`). `sandbox_research` has a fresh private workspace and no `/in` mounts — it cannot see the repo, which is correct: it only needs the open web. It surveys practitioner and critical writing on common flaws in AI-generated prose (LLM writing-tell catalogues, documentation-quality studies, style-guide critiques of generated docs) and distils ~10 distinct flaw types. Each entry: short name, 2–4-sentence description, concrete detection signals (phrasings, structures, tells), and ≥1 cited source. Aim for breadth — distinct categories, not 10 shades of "too wordy". Output: `/out/child/research01/TAXONOMY.md`.

2. **Finalise taxonomy.** Read `/out/child/research01/TAXONOMY.md`. Keep ~10 types; drop or merge any wholly inapplicable to this project, noting what was dropped and why. Write the consolidated numbered list to `/workspace/TAXONOMY.md` and `/out/TAXONOMY.md`. This becomes the spec the detection children work from.

3. **Detect.** Spawn one medium-tier `sandbox_agent` per finalised flaw type (`name=detect-<slug>`, lowercase DNS-1123 — letters, digits, interior hyphens, ≤40 chars; malformed names produce invalid volume names and poison sibling spawns). Detection children inherit the `/in/<repo>` mount and share `/workspace`; their outputs land at `/out/child/detect-<slug>/REPORT.md`. Run in batches of ≤4 concurrent — a recommended batch size, not a demesne-enforced cap; spawning all ten at once degrades keepalive stability. Cap at ~10–12 agents total.

   Each child gets the prose-surface map and its single flaw type. It must:
   - Read the text, not skim it.
   - Report only evidence-backed instances: file:line, short verbatim quote, why it qualifies, severity.
   - Mark confirmed vs. suspected.
   - State "clean on this axis" plainly when the writing is fine — no manufactured findings; well-edited docs produce empty lists on some axes.
   - Write an improvement plan tied to its specific findings (rewrite / cut / consolidate / cross-link).
   - When a claim is outright false against the code, flag-and-hand-off to `sandbox-code-defect-survey`; do not adjudicate factual accuracy here.

   Prose surfaces to name explicitly in each child's prompt: docs tree (`*.md`, tutorials, how-tos, reference, explanation), all READMEs including `examples/`, source-code comments and doc-comments, and the text the code generates — help output, error messages, and any builder that emits prose. For generated text, the finding points at the generator/template, not the sample output.

   Report structure: `# <type>` → `## Summary` (N confirmed / M suspected / top severity) → `## Findings` → `## Improvement plan`.

4. **Collate.** Copy each detection report to `/out/reports/<NN>-<slug>.md` from the orchestrator's own process — not via a `sandbox_script` child (a child's `/out` is `/out/child/<name>`; only the orchestrator's `/out` persists, and only it can write there directly). Write `/out/EXECUTIVE_SUMMARY.md`: method + types investigated (any dropped with reasons); findings table (type | confirmed | suspected | top severity | one-line headline); cross-cutting themes (tells recurring across surfaces); candid prose-health read (fair — acknowledge where the writing is strong); prioritised remediation list pointing at reports. Print `DONE`.

## Writing the orchestrator prompt

Brief it as a complete document:

1. **The target and its docs** — what the project is and the shape of its docs (Diátaxis tree? single README? generated help?), plus the prose-surface map so detection children know every text surface.
2. **The pipeline contract** — the four steps above, child-naming rule, that `sandbox_research` is isolated (open web, no repo access), and the ≤4-concurrent detection batch.
3. **The taxonomy bar** — ~10 distinct prose-flaw types grounded in cited commentary (not model priors); drop/merge inapplicable ones.
4. **Detection discipline** — read not skim; verbatim quote with file:line; confirmed vs. suspected; "clean on this axis" is a valid, valuable result; no manufactured findings; improvement plan per type tied to specific quotes.
5. **The boundary with the code survey** — claims false against the code are docs/code-alignment findings; detection children flag-and-hand-off, not adjudicate.
6. **Prior-state note** — if the docs have had prior review or simplification passes, say so; agents should expect and honestly report few or zero findings on some axes.
7. **Output contract** — the files below; report-only, no edits/rewrites/commits/branch.

## Output contract

```
/out/
  EXECUTIVE_SUMMARY.md            # The headline deliverable
  TAXONOMY.md                     # The ~10 finalised prose-flaw types + sources
  reports/
    NN-<slug>.md                  # One detailed report per flaw type
```

## Launching the orchestrator

- **`directories: ["<abs path to repo>"]` is mandatory** — detection children inherit this mount and read the project's text from `/in/<repo>`. Forgetting it leaves them with nothing to survey.
- Tier: **slow** for the orchestrator; **medium** for the research child and detection children.
- State the ≤4-concurrent detection batch explicitly in the orchestrator prompt — the default failure mode is spawning all ten at once.
- Verify load-bearing findings in context before they drive rewrites — prose judgement is subjective; a "confirmed high-severity" from a medium-tier child should be re-read by the host before becoming a rewrite target.- Scripts and data work run in demesne, never on the host. The host's only role is reading the returned `/out`, verifying the top findings, and deciding what to rewrite.
