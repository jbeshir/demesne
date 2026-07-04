---
name: sandbox-code-defect-survey
status: alpha
description: Survey a codebase's source for the defect types common to vibecoded / AI-generated projects — an orchestrator runs an open-web research child that distils a sourced taxonomy of ~10 AI-code flaw types (error handling, security, concurrency, dead/duplicated code, shallow tests, hallucinated APIs, and the like), fans out one detection subagent per type that hunts evidence-backed instances in the source (file:line, confirmed vs suspected), and synthesises per-type reports plus a prioritised summary with an improvement plan. Report-only — surveys and proposes, does not fix or land code. Apply when the user wants to know what categories of latent code problems a repo has — "code defect survey", "find common flaws in our code", "what's wrong with this vibecoded project", "audit for AI-code antipatterns". Skip for documentation and prose flaws (use sandbox-prose-defect-survey), fixing-and-landing quality work (use sandbox-quality-improvement), and building features (use sandbox-feature-work).
---

Survey a codebase for failure modes common to vibecoded / AI-generated code, grounded in live external research. The host launches one slow-tier orchestrator; it researches a sourced taxonomy of ~10 flaw types on the open web, fans out one detection child per type, then synthesises per-type reports and an executive summary. You write one good prompt, launch it, then read the reports. **Report-only** — no edits, no fix loop, no commit. To act on findings, route to `sandbox-quality-improvement` or `sandbox-feature-work`.

The taxonomy is re-researched every run (not a hardcoded list) so it tracks evolving commentary on how AI code fails. This skill covers **source code** — logic, security, concurrency, error handling — not prose quality (that's `sandbox-prose-defect-survey`). Run the two surveys separately; a taxonomy that covers both dilutes both. The prose survey owns how the text is written; this survey owns whether the text is true of the code. **One boundary belongs here:** docs/code alignment — a doc, README, or comment that contradicts what the code actually does is a code finding because verifying it requires reading the code.

**Watch out:** The orchestrator must `cp` reports into its own `/out` directly — a `sandbox_script` child's `/out` is `/out/child/<name>` and would strand them. Detection children are medium-tier; verify any confirmed high-severity or security finding by reading the cited `file:line` before routing it to a fix pipeline.

## Procedure

1. **Research.** Spawn one medium-tier `sandbox_research` child (`name=research01`). `sandbox_research` runs in a fresh private workspace with no `/in` mounts and open web access — it cannot read the repo, which is correct: it only needs the web. It surveys practitioner and academic writing on common flaws in AI-generated code (Copilot/Codex security analyses, Go-concurrency corpora, slopsquatting reports, and the like) and distils ~10 distinct flaw types biased to the target's domain (pass the domain in the orchestrator prompt so it returns backend flaws for a Go server, not frontend ones). For each type: short name, 2–4 sentence description of the flaw and why AI code exhibits it, concrete detection signals (what to grep/read for), and ≥1 cited source (title + URL). Distinct categories, not variants of one theme. Output: `/out/child/research01/TAXONOMY.md`.

2. **Finalise taxonomy.** Read `/out/child/research01/TAXONOMY.md`. Keep ~10 types; drop or merge any wholly inapplicable to the target (note what was dropped and why). Write the consolidated numbered list to `/out/TAXONOMY.md` (name + description + detection signals + source). Do not reuse a canned list — the fresh research is what differentiates this from a fixed-dimension audit. One type to always keep even if the research doesn't surface it: **docs/code-alignment** (stale docs or comments that contradict the actual code).

3. **Detect.** Spawn one medium-tier `sandbox_agent` per finalised type (`name=detect-<slug>` — DNS-1123: lowercase letters, digits, interior hyphens, ≤40 chars; bad names produce invalid volume names and poison sibling spawns), dispatched with `background: true` and polled to completion with `sandbox_wait` — keep **≤8 jobs in flight** (a host-resource guard, not a demesne cap). Cap at 10–12 types total. Use `egress=none` — detection is read-only. Each child:
   - Reads the repo from `/in/<repo>` (inherited read-only mount).
   - **Confirms by reading code, not grep-then-guess** — a grep/signal hit locates a candidate; a finding requires reading the surrounding code.
   - Reports each instance with `file:line`, a short excerpt, why it qualifies, and severity; marks **confirmed vs suspected**.
   - Says **"clean on this axis"** plainly when it finds nothing — no padding; an empty list on a hardened axis is signal, not failure.
   - Writes an improvement plan tied to its specific findings.
   - Output: `/out/child/detect-<slug>/REPORT.md`, structured: `# <type>` → `## Summary` (N confirmed / M suspected / top severity) → `## Findings` → `## Improvement plan`.

   If the repo has had prior quality passes, say so in the child prompts — agents should expect and honestly report few or zero instances on hardened axes. Forbid manufactured findings: a hardened codebase should produce small or empty lists on several axes.

4. **Collate.** Copy each detection report to `/out/reports/<NN>-<slug>.md` with plain `cp` **in the orchestrator's own process** — not via a `sandbox_script` child (a child's `/out` is `/out/child/<name>` and would strand the files). Write `/out/EXECUTIVE_SUMMARY.md`:
   - Method + types investigated (any dropped, with why).
   - Findings table: `type | confirmed | suspected | top severity | one-line headline`.
   - Cross-cutting themes spanning ≥2 types.
   - Candid codebase-health read (strengths and real weaknesses; acknowledge prior cleanup).
   - Prioritised remediation list, each item pointing to the report that details it.

   Then print `DONE`.

## Writing the orchestrator prompt

Brief it as a complete document:

1. **Target and domain** — what the codebase is and what kind of system; a repo map (key dirs/packages) so detection children don't have to rediscover structure.
2. **Pipeline contract** — the four steps above; child-naming rule; `sandbox_research` is isolated (open web, no repo access); background-dispatched detection children (≤8 in flight via `sandbox_wait`).
3. **Taxonomy bar** — ~10 distinct types, cited sources, domain-appropriate; drop/merge inapplicable; always include docs/code-alignment lens.
4. **Detection discipline** — confirm by reading code; `file:line` evidence; confirmed vs suspected; "clean on this axis" is valid; no manufactured findings; improvement plan per type.
5. **Prior-state note** — if the repo has had quality passes, say so; agents should expect honest near-empty reports on hardened axes.
6. **Output contract** — the files below; report-only, no edits/builds/commits/branch.

## Output contract

```
/out/
  EXECUTIVE_SUMMARY.md          # headline deliverable
  TAXONOMY.md                   # ~10 finalised flaw types + sources
  reports/
    NN-<slug>.md                # one report per flaw type
```

## Launching the orchestrator

- **`directories: ["<abs path to repo>"]` is mandatory** — detection children inherit this mount and read the repo from `/in/<repo>`. Without it they have nothing to survey.
- Tier: **slow** for the orchestrator; **medium** for the research child and detection children (the orchestrator sets this).
- Detection fans out via **background dispatch + `sandbox_wait`, ≤8 in flight** — say this explicitly in the prompt. Blocking calls won't do: the orchestrator issues children one per turn, so blocking detectors are never issued in parallel and run sequentially however the prompt is phrased.
## Host-side landing

1. Read `/out/EXECUTIVE_SUMMARY.md` for the findings table, cross-cutting themes, and prioritised remediation list.
2. For any **confirmed high-severity, security, or concurrency finding**, read the cited `file:line` in the actual source before treating it as actionable — false positives happen; a quick read settles it.
3. Triage suspected findings separately from confirmed ones.
4. All computation stays in demesne. To act on findings: route to `sandbox-quality-improvement` (behaviour-preserving fixes) or `sandbox-feature-work` (a specific change). The host reads `/out` and decides routing — it does not re-run detection or apply fixes directly.
