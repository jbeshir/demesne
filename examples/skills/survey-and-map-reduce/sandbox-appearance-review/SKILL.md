---
name: sandbox-appearance-review
status: alpha
description: Run a parallel, multi-lens visual review of a front-end through a demesne pipeline and deliver prioritised appearance-improvement proposals — report-only, no code changes. The host mounts the FE and launches a slow-tier orchestrator that explores it to pick a render matrix (screens × states × viewports × light/dark), builds and screenshots it offline in a browser sandbox including colour-blindness and halation simulations, fans out ten parallel reviewer agents — aesthetics & typography, spacing & alignment, layout & composition, colour & contrast (WCAG/APCA, halation, CVD), depth & elevation, imagery & icons, component states, micro-typography, brand cohesion, cross-cell consistency — then merges and tiers their findings into a proposals report backed by before-screenshots. Apply when the user wants an existing UI critiqued for appearance and given concrete visual improvements. Triggers include "review the look of this UI", "do a visual/appearance review", "critique the design of this front-end", "how could this UI look better", "screenshot review of the styling". Scoped to visual review from rendered screenshots only — no functional, keyboard, or screen-reader testing — and it proposes changes rather than making them. Skip when the FE cannot be built or loaded offline, when the user wants the changes implemented (use sandbox-feature-work), and for non-visual code or docs review (use sandbox-code-defect-survey or sandbox-docs-quality).
allowed-tools: Read, Glob, Grep, Bash, Write, Edit, mcp__demesne__sandbox_agent, mcp__demesne__sandbox_script
---

Review the *appearance* of an existing front-end and hand back a prioritised set of improvement proposals — not code changes. The host mounts the FE and launches a slow-tier orchestrator; the orchestrator copies the FE into its `/workspace`, explores it to pick a render matrix, builds and screenshots it offline in a browser sandbox (with colour-blindness and halation simulations), fans out ten parallel medium-tier reviewer agents each owning one visual lens, and merges their findings into a tiered proposals report backed by before-screenshots. The deliverable is a **report**, not a branch — same render-and-fan-out shape as the build pipelines, but it critiques and proposes; implementing the proposals is a separate task (e.g. `sandbox-feature-work`).

**Scope.** Visual review only — everything judged here is judgeable from a rendered screenshot. No keyboard navigation, ARIA, screen-reader, or other runtime-behaviour testing; accessibility is covered only on its *visual* axes (contrast, colour-only signalling, focus-ring visibility, touch-target size as drawn). The lenses, their criteria, the overlap rules, and the simulation recipes live in **`lenses.md`** beside this file — it is the authority for what each reviewer checks and must be passed into the pipeline.

**Watch out (cross-cutting):** the orchestrator must `cp` final artefacts (gallery + report) into its own `/out` itself — a `sandbox_script`/`sandbox_agent` child writes only to `/out/child/<name>`, so anything left there alone never reaches the host. `/workspace` is shared across the orchestrator and its children and is torn down on exit; only `/out` persists.

## What the FE has to be

The pipeline renders the FE **offline** (`egress=none`), so the target must build or load with no open network:

- A buildable web app (Vite/Astro/etc.) whose `dist/` loads over `file://`, or that serves from a local static server.
- A pre-built `dist/` or a static site (raw `index.html` + assets).
- An offline **dev harness** the project already ships (a fixtures-driven harness page, a Storybook build) — prefer this when it exists; it is purpose-built for rendering states in isolation.

Data-driven UIs must reach their states from committed **sample/fixture** data, not the live network. If a screen can only be reached with live data, the explore pass records it as not-captured rather than reaching for egress.

## Procedure

1. **Stage the FE.** From the orchestrator: `cp -a /in/<fe>/. /workspace/repo` (the `-a` preserves everything; `<fe>` is the basename of your `directories:` mount). Read the project to learn how it builds/serves offline and what harness/fixtures it already ships.

2. **Explore + render plan** (orchestrator, reading `/workspace/repo`). Reuse any existing harness/fixtures rather than inventing a server. Then pick the **render matrix** by exploration:
   - **Screens / pathways** — the distinct views worth reviewing (routes, key pages, or the representative fixtures/stories that exercise the important components). A covering set, not every permutation.
   - **States** — per screen, the reachable states worth a shot: default, and as applicable hover, focus, active, disabled, loading, empty, error, and long/edge-case content (real-data overflow).
   - **Viewports** — a small ladder spanning the FE's real breakpoints (e.g. ~360 / ~768 / ~1280, adjusted to the project).
   - **Colour schemes** — light and dark via `prefers-color-scheme`, plus any in-app theme toggle.
   For each cell record the concrete drive recipe (URL/params, clicks/hovers, scheme/theme emulation) and the readiness signal to wait on (a ready marker, network-idle, or a settle the harness exposes — prefer an explicit marker over a timeout). Save `lenses.md` (passed in by the host) to `/workspace/lenses.md`. Write `/out/RENDER-PLAN.md`: the build/serve commands, the matrix, the per-cell recipe, and anything that couldn't be reached and why.

3. **Build** — a `sandbox_script` child, `name=build`, `image=node`, `egress=package-managers` (skip this step if the FE ships a usable pre-built artefact). Install and build per the plan, leaving the artefact in the shared `/workspace/repo`. Cold npm proxy can stall; if dependencies must be fetched, pre-installing `node_modules` in an open-egress sandbox and mounting it is the escape hatch.

4. **Capture** — a `sandbox_script` child, `name=capture`, `image=browser`, `egress=none`, running a Playwright script the orchestrator authors from the render plan against `/workspace/repo`'s built artefact. For each matrix cell it sets the viewport, emulates the colour scheme, drives the state, waits on the readiness signal, and screenshots to the shared `/workspace/gallery/` as `<screen>__<state>__<viewport>__<scheme>.png`. It freezes `Date`/`Math.random` and disables animation for stable shots, and records `/workspace/gallery/capture-report.json` (per-cell horizontal overflow, console/page errors, and any cell it couldn't produce). For each base light/dark cell it also produces the **colour simulations** the colour lens needs — deuteranopia / protanopia / tritanopia and halation — **in the same pass** by injecting the SVG `feColorMatrix` filters and the halation glow CSS from `lenses.md` Appendix B and re-screenshotting (`…__deutan.png`, `…__protan.png`, `…__tritan.png`, `…__halation.png`). Do it in-page: the `browser` image has no ImageMagick/Python, so the SVG-filter + CSS-injection method is the reliable one.

5. **Parallel multi-lens review** — fan out **ten** medium-tier `sandbox_agent` reviewers (`name=lens-01-typography`, `lens-02-spacing`, … `lens-10-cross-cell`; lowercase DNS-1123 labels), one per lens in `lenses.md` Appendix A, using the background-dispatch loop below. Give each: its **lens section verbatim** from `/workspace/lenses.md`, the **overlap rule** for that lens (what it must NOT re-flag), the render plan, and the gallery at `/workspace/gallery/` (reviewers view PNGs with Read; the colour reviewer also reads the CVD/halation variants). Each emits structured findings to `/out/findings/lens-NN-<slug>.json`, one finding per object:
   `{lens, severity: critical|high|medium|low, screen, state, viewport, scheme, location, issue, principle, evidence, suggested_fix}` — `severity` reserves **critical** for contrast below the WCAG AA floor, clipped/overflowing/invisible content, a state broken in one scheme, or text below the absolute size floor (Lens 1 criterion 1); reviewers judge size and contrast by the numeric rule, not by their own ability to resolve the text (see the reviewer-calibration note in `lenses.md`); `principle` names the cited heuristic/threshold; `evidence` names the gallery file(s) and where to look. The orchestrator harvests each reviewer's output from `/out/child/lens-NN-<slug>/`.

6. **Merge + synthesise proposals** (orchestrator). Pool the findings, **dedup using the Appendix A overlap rules**, cluster by area/component/theme, and convert clusters into concrete **appearance-improvement proposals**, tiered by the Appendix A merge-priority order. Then `cp -a /workspace/gallery /out/gallery` and write `/out/APPEARANCE-REVIEW.md`: a short overall read, then proposals each carrying — title; area/theme; severity (max of its findings); **what to change**; **where** (screens/components/cells); **why** (the lenses + principles it answers); **evidence** (the before-screenshot filenames); expected impact; rough effort (S/M/L). Group the proposals into **quick wins** (low-effort, high-signal — usually spacing, micro-typography, contrast tweaks) and **structural** (layout/composition/system changes). Print `DONE`.

7. **Report.** Write `/out/SUMMARY.md`: how many cells were captured (and any not-captured), how many findings each lens raised, and the proposal counts by tier and by quick-wins vs structural.

## Concurrent fan-out

The ten reviewers are independent, so dispatch them in the background — blocking children issued one per turn run sequentially. Use the canonical loop:

1. **Dispatch** each reviewer with `background: true`, collecting its `job_id`; keep at most **8 in flight** (a host-resource guard) — dispatch 8, then launch one more each time a job finishes (a rolling window).
2. **Poll** each `job_id` with `sandbox_wait` (`timeout_seconds: 120`), re-calling any still `running` until all reach a terminal state.
3. **Harvest** each reviewer's findings from `/out/child/lens-NN-<slug>/`.

The merge in step 6 is a genuine barrier — drain all ten reviewers before synthesising. Steps 1–4 are sequential by construction (shared `/workspace`) and do not fan out.

## Writing the orchestrator prompt

Brief it as a complete document: the FE to review and **what good appearance means for this kind of product**; the offline-render constraint and the harness/fixture/sample-data preference; the **full `lenses.md` content** (the host passes it in) with the instruction to save it to `/workspace/lenses.md` and hand each reviewer its lens section plus overlap rule verbatim; the seven steps with the child-naming rule, the explicit BUILD (`image=node`, install+build) and CAPTURE (`image=browser` `egress=none`, the Playwright matrix-drive plus in-page CVD/halation variants) commands; the ten-lens **background** fan-out (≤8 in flight via `sandbox_wait`) and the finding schema; the **merge** that dedups by the overlap rules, tiers by the priority order, copies the gallery to `/out`, and emits proposals grouped quick-wins vs structural; and the output contract below ending in `DONE`. Make clear it **proposes, never edits** — no commit, no branch, no PR. Tell it explicitly: do NOT build, render, or screenshot itself — the agent image has no Node toolchain or browser; those are `sandbox_script` children. The orchestrator only explores, authors the capture script and reviewer prompts, and synthesises.

## Launching the orchestrator

- **`directories: ["<abs path to FE>"]` is mandatory.** Without it the orchestrator wakes with no FE to review. Verify it on every launch.
- Tier: **slow** for the orchestrator (it does explore + merge itself); **medium** for the ten reviewer children (the orchestrator sets this); the build and capture children are `sandbox_script` (`image=node` / `image=browser`, no tier).
- Restate the grounding facts: offline render at `egress=none`, prefer the project's own harness/fixtures, reach states from committed sample data, the in-page SVG-filter/CSS method for the colour simulations, and that `lenses.md` is the authority for the ten lenses, their criteria, the overlap rules, and the simulation recipes.

## Host-side readback

Nothing to merge or push — this is report-only.

1. Read `/out/APPEARANCE-REVIEW.md` and `/out/SUMMARY.md`.
2. Present the overall read, the quick wins, and the top tiered proposals, surfacing a few representative before-screenshots from `/out/gallery/` (Read renders them).
3. Offer, only if the user wants it, to save the report into the repo (e.g. `docs/appearance-review-<date>.md`) or to follow up by implementing selected proposals — that implementation is a separate change, not part of this skill.

## Output contract

```
/out/
  RENDER-PLAN.md        # build/serve commands, the matrix, per-cell drive recipe, not-captured notes
  gallery/              # <screen>__<state>__<viewport>__<scheme>.png + __deutan/__protan/__tritan/__halation
                        # + capture-report.json (per-cell overflow, console errors, missing cells)
  findings/             # one structured file per lens (lens-NN-<slug>.json)
  APPEARANCE-REVIEW.md  # merged, deduped, tiered proposals with before-screenshots; quick-wins vs structural
  SUMMARY.md            # cells captured / not-captured, findings per lens, proposal counts by tier
```
