---
name: sandbox-make-twine-game
description: Build, compile, playtest, review, and deliver a production-depth Twine interactive-fiction game from a concept or an existing mounted repository. Use for branching text and narrative games; use sandbox-make-ts-game for real-time coded games.
---

# Build a Twine game

Own the complete workflow. Produce self-contained playable HTML, editable Twee source, reproducible graph/browser evidence, and a record of editorial decisions and risks.

Read [resources/compile-playtest.md](resources/compile-playtest.md) before working and again before final validation. It owns source, fixture, compiler, graph, browser, report, and artifact contracts; do not improvise them.

## Prepare and orchestrate

1. Derive intake candidates from the generated Environment input list when present; otherwise inspect immediate `/in` entries excluding `/in/.agent` and `/in/previous-jobs`. Identify projects by repository/project markers, not raw entry count. Use the sole repository, or greenfield mode when a concept was supplied; fail on multiple repositories or a missing explicitly requested repository.
2. `/workspace/repo` is the only project root. It must be absent or empty before staging; if it is nonempty, fail explicitly rather than choosing another root or altering unexplained contents. Copy an input repository completely, including `.git`, and record source, destination, HEAD, and initial status. Greenfield mode creates the resource's clean Twee 3 layout there.
3. Use uniquely named children, reasoning agents for design/writing/review, and deterministic scripts for compile/graph/browser gates. Nested tools other than `sandbox_research` share `/in` and `/workspace`; research has neither and must return findings through its output. Omit `model` by default. Honor an explicit model only when it exactly matches a concrete value in the live child-tool description or known configured allowlist (currently `haiku`, `sonnet`, `opus`, `fable`, `gpt-5.6-sol`, `gpt-5.6-terra`, `gpt-5.6-luna`, `gpt-5.5`, or `gpt-5.4-mini`) and its provider is configured; unconstrained schema acceptance is insufficient.
4. For a background call retain both `job_id` and child `name`; call `sandbox_wait(...)` with its long default and repeat only if it returns `running`, then cancel obsolete dependents. Read full diagnostics at `/out/child/<name>/stderr.log` in the spawning parent (or `/in/previous-jobs/<name>/stderr.log` in a later sibling); treat returned `output_path`/`output_dir` as informational unless the live tool explicitly guarantees an in-sandbox path.
5. Retry an infrastructure/tool failure exactly once under a fresh name. Treat a completed nonzero deterministic command or failing structured report as a product defect: preserve evidence, dispatch a fix, then recompile and retest under fresh names, for at most three fix/retest rounds per gate. On exhaustion write `/out/FAILURE.md`, cancel dependents, and stop.
6. Copy selected child evidence to parent `/out`; child output is not surfaced automatically. Delivery must satisfy every applicable entry in the resource's final-delivery manifest, including the complete repository at `/out/repo`.

## Workflow

1. **Intake and research.** Record input/concept, audience, boundaries, experience, workspace provenance, constraints, and sources in `WORKLOG.md`. Distinguish fact from invention.
2. **Design the graph.** Have a fresh agent write `DESIGN.md` with premise, themes, voice, cast, player role, variables, format decision, boundaries, passage budget, endings, accessibility, state projection, and fixture policy. Write `PASSAGE-GRAPH.md` with every passage, choice, condition/effect, merge, and ending.
3. **Plan.** Use Harlowe by default. Choose SugarCube only when a named, verified requirement is unsupported or materially impractical in the pinned Harlowe version; record the requirement and verified format versions in `DESIGN.md`. Plan source, ownership, macros, styling, media provenance, saves, and fixtures.
4. **Vertical slice.** Implement a complete branch through a meaningful choice to at least two endings. Re-read the resource, then compile and run static graph and browser checks before expanding.
5. **Expand.** Add dependency-ordered graph sections in bounded phases. After each, compile, validate, and traverse. Require complete planned reachability, resolvable choices, approved terminals, escapable cycles, ending paths, and no browser/console errors. Fix deterministic failures within the bounded loop.
6. **Review and fix.** After checks pass, use an independent fresh-context reviewer for prose, agency, branch distinctness, state, pacing, accessibility, continuity, and presentation. Apply fixes separately and rerun all checks. Run at most three rounds; stop early only when no material improvement remains and record deferrals.
7. **Final cohesion.** Independently play representative routes/endings and assess voice, payoff, branch equity, state continuity, pacing, endings, typography, mobile behavior, and graph balance. Apply worthwhile fixes and rerun complete validation within the same bounded rules.
8. **Deliver.** Run the resource's exact clean commands and satisfy its final-delivery manifest. Write `/out/SUMMARY.md` with commands, captured tool/format/image versions, results, passage/ending counts, stop reason, backlog, and risks. Print `DONE` only after every manifest entry whose condition applies passes its stated gate.

Never accept self-review as independent review or describe an incomplete delivery as successful.
