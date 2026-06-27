---
name: sandbox-make-ts-game
description: Build a real coded arcade/action/puzzle game in TypeScript from a concept through a demesne pipeline and deliver a durable bundle the recipient can play and keep building on. A slow-tier orchestrator runs research → plan the core loop → scaffold from the baked Phaser+Vite+TypeScript template → continuous correctness checks (build with tsc + vite; an offline playtest that loads the build, drives scripted inputs, and asserts state changes with no console errors) → an open-ended visual/feel improvement cycle that iterates until improvements run dry, then a host-side delivery of the editable TypeScript project plus the runnable dist. Apply when the user wants a real-time game with input, motion, and score — a platformer, shooter, puzzle, or toy. Triggers include "build me a game where you do X", "a little arcade/platformer/puzzle game", "a playable game about Y with controls". Skip for branching text/narrative games (use sandbox-make-twine-game), data dashboards or visualizations (use build-widget), and slide decks (use sandbox-make-slide-deck).
allowed-tools: Read, Glob, Grep, Bash, Write, Edit, mcp__demesne__sandbox_agent, mcp__demesne__sandbox_research, mcp__demesne__sandbox_script
---

Turn a concept into a finished, playable TypeScript game and hand the recipient something they can both **play** (a built `dist/` that runs offline on desktop and mobile, ready to drag onto itch.io) and **keep building on** (the full TypeScript project — open in any editor, `npm install && npm run dev`). A slow-tier orchestrator runs research → plan → scaffold → continuous correctness checks (build + playtest) → an open-ended quality improvement cycle, copies the bundle to `/out`, and the host delivers it. The deliverable is a durable artifact bundle, not a repo branch.

The output is *useful to the recipient*, not a demo of demesne: a real game they can publish and a clean code project they can extend. This is the `build-widget` machinery — scaffold, build, render/verify, improve — generalized from a widget to a game, with the verify target swapped from "renders" to "plays."

## The improvement cycle (not a gate)

Quality here is *pursued*, not merely passed. Keep two things distinct:

- **Correctness invariants** — it builds (`tsc` + `vite`); the canvas readies; no console/page errors; scripted input changes state. These are hard constraints: a violation means the game is *broken*, not merely unpolished. They must hold after every change — but holding them is the floor, never the finish line.
- **Quality** — fun, game feel, readability, control clarity, visual polish, completeness. Pursue it through an open-ended improvement cycle: each round actively hunts for the highest-value improvements and applies them. Do **not** ask "is this good enough?" and stop. End the loop on *diminishing returns* (a round that finds nothing worth doing, or two rounds with nothing materially new), or on a round budget reached as a safety stop — in which case deliver the outstanding-improvements backlog rather than calling it done. Stop because little is left worth improving, not because a minimum bar was cleared.

**Watch out (cross-cutting):** the orchestrator must `cp` the final bundle to its own `/out` itself — a `sandbox_script` child writes only to `/out/child/<name>`. `/workspace` is torn down on exit; only `/out` persists. The build and playtest run on `image=webgamedev` at `egress=none` — the template's dependencies are baked into the image, so there is no `npm install` over the network in the gates (cold npm in the sandbox is glacially slow; the baked image is what keeps the pipeline fast). Never deliver a bundle whose playtest exited non-zero.

## Stack and the baked image

- **Image: `webgamedev`** — a demesne-built image (Playwright/Chromium for playtest + Node + a warm Phaser + Vite + TypeScript template at `/opt/game-template` with `node_modules` already installed). Build and playtest run `egress=none`. If `webgamedev` is unavailable, fall back to `image=node` + `egress=package-managers` and a real `npm install` (much slower) — but the baked image is the supported path.
- **Engine: Phaser** by default (batteries-included: scenes, input, physics, audio, asset loader; well-documented). Pick **Kaplay** or plain Canvas only for a deliberately tiny game, and say so in the plan.
- **Bundler: Vite** → a static `dist/` with `base: './'` so it runs over `file://` (the offline playtest) and at any static-host root.
- **Ready + state markers (the playtest contract):** the template's entry sets `id="game-ready"` on an element once the first scene's `create()` has run, and reflects lifecycle on `<html data-game-state="…">` (`boot`/`playing`/`won`/`lost`). The playtest waits on these markers, never a settle-timeout.

## Procedure

1. **Host prep.** Derive a lowercase-hyphenated `<slug>` from the concept. Launch the orchestrator (slow tier) — no repo mount needed; it scaffolds from the image's baked template under `/workspace` and delivers to `/out`.

2. **Research** (`sandbox_research`, isolated, open egress) — for the genre's mechanics, controls conventions, and a feasible scope, or an asset approach (generated sprites vs. primitive shapes). Returns `/out/FINDINGS.md`. A simple, well-understood game can skip this.

3. **Plan the core loop** — the orchestrator writes `/workspace/PLAN.md`: the one-sentence game, the moment-to-moment loop, controls (keyboard + touch), win/lose conditions, the scene list (boot/menu/play/gameover), the art approach (generated PNG sprites via the image-gen MCP, or drawn primitives), and scope guardrails so it finishes. Name the `data-game-state` values the game will use.

4. **Scaffold + implement** — one or two medium-tier `sandbox_agent` children. First `cp -a /opt/game-template/. /workspace/game` and set the slug/title; then write `src/` (TypeScript): scenes, the core loop, input (keyboard + pointer), entities, score, and win/lose, wiring the `#game-ready` marker and `data-game-state` transitions. Generate sprites/audio into `src/assets/` if the plan calls for it. Keep it polished from the start: a title screen, a visible score, a restart path, `prefers-reduced-motion`, responsive canvas. Phases share `/workspace` — run them sequentially. Each writes `/out/SUMMARY.md`.

5. **Build correctness check** — a `sandbox_script` child, `name=build`, `image=webgamedev`, `egress=none`. In `/workspace/game`: `npm run build` (runs `tsc` then `vite build`). It must exit 0 and produce `dist/index.html` + assets. On failure (type errors, bundler errors) fix and rebuild until it holds. A correctness invariant maintained through every improvement round below — not the finish line.

6. **Playtest correctness check** — a `sandbox_script` child, `name=playtest`, `image=webgamedev`, `egress=none`. Run the input-driving harness (below) over `/workspace/game/dist/index.html`. It must exit 0, write `playtest-ok` + `playtest-report.json`, and a screenshot gallery to `/out/gallery/`. A non-zero exit means a real defect — the canvas never readies, a console/page error fired, or scripted input produced no state change — so fix, rebuild, and re-playtest until it holds. Also a correctness invariant, not the finish line.

7. **Improvement cycle (three lenses)** — keep making the game better until it stops being worth it. Each round, fan out **three parallel** medium-tier `sandbox_agent` reviewers over the current gallery + report — **visual** (readability, hierarchy, polish of the title/HUD/gameover), **controls/feedback** (input discoverability, feedback for hit/score/win/lose, game feel), **scope/completeness** (does it start, play, and end — and what would make it more fun) — each proposing the **highest-value improvements** it can find, ranked, as proposals rather than a pass/fail verdict. The orchestrator merges and applies the worthwhile improvements, re-runs the build + playtest correctness checks, and goes again. Continue until a round finds nothing worth doing (diminishing returns) or a safety budget (≈5 rounds) is reached — on which, record what remains. Log each round's changes and the final outstanding-improvements backlog to `/out/IMPROVEMENTS.md`.

8. **Deliver** — the orchestrator itself `cp`s the bundle to `/out` (see Output contract — both the editable `src` project and the runnable `dist/`), writes `/out/README.txt` in plain language (open `dist/index.html` to play or upload the `dist` folder to itch.io; `npm install && npm run dev` in the project to edit), and `/out/CHANGES.md` (slug, engine, controls, scenes, correctness-check summary, how many improvement rounds ran and whether the cycle stopped on diminishing returns or the budget, and any backlog). Print `DONE`.

## The playtest harness (Playwright, the orchestrator writes it to `/workspace/playtest.cjs`)

Loads `file://…/dist/index.html` (`--allow-file-access-from-files`), then:

- Wait for `#game-ready` (attached), screenshot the title/first frame.
- Drive a scripted input sequence from the plan (e.g. press Right/Space for N frames, advancing time deterministically), screenshotting at intervals. Assert `data-game-state` advances as planned (`boot`→`playing`, and at least one of `won`/`lost` reachable by a scripted path or a forced condition).
- **Fail (exit non-zero)** if `#game-ready` never appears (blank/broken build), if any console or page error fires, or if no scripted input changes `data-game-state` or any observable state hook (a frozen game). Record frames seen, state transitions, and errors to `playtest-report.json`.
- Freeze `Date`/`Math.random` and fix the rAF cadence so runs are deterministic and screenshots are stable.

## Launching the orchestrator

- No `directories:` mount required. Tier: **slow** orchestrator; **medium** implement/review/fix phases; `build`/`playtest` are `sandbox_script` (`image=webgamedev`, `egress=none`).
- Tell it explicitly: **do NOT build or playtest yourself** — the agent image has no Node toolchain or browser; those are `sandbox_script` children on `image=webgamedev`. And **scaffold from `/opt/game-template`** rather than reinventing the Vite/Phaser/TS config.
- Brief it as a complete document: the concept and what a good, fun version looks like; the default stack (Phaser + Vite + TS, `base: './'`); the `#game-ready` + `data-game-state` markers; the scaffold-from-template rule; the eight steps with the child-naming rule; the build/playtest commands and harness contract; the three-lens improvement cycle (reviewers propose ranked improvements, not pass/fail; iterate to diminishing returns, not a minimum bar); and the output contract.

## Host-side delivery

No branch or PR — the bundle in `/out` is the deliverable, and the in-sandbox build+playtest is authoritative; the host does not rebuild.

1. Read `/out/CHANGES.md` and `/out/IMPROVEMENTS.md`; confirm `playtest-ok` is present (the correctness invariant held) and check why the improvement cycle stopped — diminishing returns is the goal; if it stopped on the budget, surface the backlog to the user.
2. Surface the bundle: give the user the `<output_dir>` path and offer to copy it where they want (`sandbox_download` or a named `cp`). Show a screenshot or two from `/out/gallery/`.
3. Tell them, in one line, how to **play** (open `dist/index.html`, or upload `dist/` to itch.io) and how to **edit** (`npm install && npm run dev` in the project).

## Output contract

```
/out/
  game/               # the editable TypeScript project (src/, package.json, vite.config.ts, tsconfig.json, assets) — minus node_modules
  dist/               # the built, runnable game (offline, mobile-ok, itch.io-ready)
  gallery/            # title + scripted-playthrough frames
  playtest-report.json# frames seen, state transitions, any console/page errors
  FINDINGS.md         # research notes (only if step 2 ran)
  PLAN.md             # core loop, controls, scenes, win/lose, art approach
  SUMMARY.md          # per implement/fix phase
  IMPROVEMENTS.md     # per-round three-lens changes + the remaining-opportunities backlog
  README.txt          # plain-language: how to play, how to edit
  CHANGES.md          # slug, engine, controls, scenes, correctness summary, improvement rounds + stop reason + backlog
```
