---
name: sandbox-make-ts-game
status: alpha
description: Build a real coded TypeScript game — from a one-mechanic toy to a multi-system game — through a demesne pipeline and deliver a durable bundle the recipient can play and keep building on. A slow-tier orchestrator runs research → a game-design phase that fleshes the brief into a detailed spec and a system decomposition → spec-driven, orchestrator-determined vertical-slice phases, each with a build + scenario-playtest correctness check and an interspersed per-slice review before the next phase → a final whole-game cohesion pass → host-side delivery of the editable TypeScript project plus the runnable dist. The design phase right-sizes the build, so the same skill scales down to a tiny game and up to a complex one. Apply when the user wants a real-time game with input, motion, and state — a platformer, shooter, puzzle, roguelike, or toy. Triggers include "build me a game where you do X", "an arcade/platformer/puzzle/roguelike game", "a playable game about Y with controls", "a bigger game with several systems". Skip for branching text/narrative games (use sandbox-make-twine-game), data dashboards or visualizations (use build-widget), and slide decks (use sandbox-make-slide-deck).
allowed-tools: Read, Glob, Grep, Bash, Write, Edit, mcp__demesne__sandbox_agent, mcp__demesne__sandbox_research, mcp__demesne__sandbox_script
---

Turn a concept into a finished, playable TypeScript game and hand the recipient something they can both **play** (a built `dist/` that runs offline on desktop and mobile, ready to drag onto itch.io) and **keep building on** (the full TypeScript project — open in any editor, `npm install && npm run dev`). A slow-tier orchestrator runs research → game design → spec-driven vertical-slice phases (each build-checked, scenario-playtested, and reviewed before the next) → a final cohesion pass, copies the bundle to `/out`, and the host delivers it. The deliverable is a durable artifact bundle, not a repo branch.

The output is *useful to the recipient*, not a demo of demesne: a real game they can publish and a clean code project they can extend. This is the `build-widget` machinery — scaffold, build, verify, review — generalized to a game and made **spec-driven and phased** so it scales: a design phase turns the brief into a detailed spec whose system decomposition *is* the phase plan, and review is woven through the build rather than bolted on at the end.

## How quality works here (interspersed review, not a one-shot gate)

Two things stay distinct, as in the small-game flow — but the review is **interspersed**, not terminal:

- **Correctness invariants** — after every phase the game builds (`tsc` + `vite`) and the whole scenario suite passes (canvas readies, scripted scenarios reach their asserted states, no console/page errors). A violation means the game is *broken*; these hold continuously, but holding them is the floor, never the finish line.
- **Quality** — pursued by **reviewing each vertical slice as it lands**, so a system is critiqued while it is small and cheap to change and later phases build on reviewed foundations. Each review proposes the highest-value improvements (ranked, not pass/fail) and iterates to diminishing returns within the phase. A single **final cohesion pass** at the end catches only what cannot be seen until the systems are assembled (cross-system feel, balance, emergent issues). Stop reviewing because little is left worth improving, not because a minimum bar was cleared; carry anything deferred into the backlog.

## Watch out (cross-cutting)

- **Every child's `/out` is isolated** at `/out/child/<name>` (`sandbox_script` and `sandbox_agent` alike). Children write artefacts (screenshots, reports, summaries) under the shared `/workspace`; the orchestrator relays them into its own `/out`. `/workspace` is shared across the run but torn down on exit.
- **Launch every `sandbox_agent` child (design, implement, reviewers, fix) with `background: true` and poll with `sandbox_wait`.** A synchronous agent past the ~300s MCP idle timeout aborts on the orchestrator side while it keeps running, racing on the shared `/workspace`.
- **Phases are sequential and one agent mutates `/workspace/game/src/` at a time.** Game systems are coupled — integration, not isolation, is the hard part — so the backbone is an ordered slice-by-slice build, not a parallel section fan-out. Parallelism is reserved for genuinely independent work (e.g. generating several assets at once), each writing to its own subdir.
- **The `/opt/game-template` scaffold lives only inside the `webgamedev` image** — a `sandbox_agent` runs a *different* image and cannot see it. The **orchestrator** copies it once via a `webgamedev` `sandbox_script` (step 5); implement agents then **edit the already-scaffolded `/workspace/game/src/` directly with Write/Edit**, never through a `sandbox_script` grandchild (which can run away, and is uncancellable if its `job_id` is lost).
- **Never advance past a phase whose build or scenario playtest is failing.** A red slice poisons everything layered on it.

## Stack and the baked image

- **Image: `webgamedev`** — Playwright/Chromium for the playtest + Node + a warm Phaser + Vite + TypeScript template at `/opt/game-template` (`node_modules` baked). The template is inside the image filesystem — reachable only from a `webgamedev` `sandbox_script`, not from an agent image, so the orchestrator does the scaffold (step 5). Build and playtest run `egress=none`. Fallback if unavailable: hand-scaffold Phaser+Vite+TS, build on `image=node` + `egress=package-managers` (`npm install` ~10–20s), playtest on `image=browser` (it has Chromium; `node` does not).
- **Engine: Phaser** by default; pick another in the design only with a stated reason.
- **Bundler: Vite** → static `dist/` with `base: './'` so it runs over `file://` (offline playtest) and at any host root.

## The test API + scenario contract (what makes phased validation scale)

A single scripted "press right for N frames" playtest does not scale to a multi-system game. Instead the game exposes a deterministic control surface and the playtest drives a *growing suite of scenarios* through it:

- **`window.__game`** — an always-present, harmless-when-idle object: `getState()` plus the per-system getters and force-transitions the design names (e.g. `getScore()`, `getLevel()`, `start()`, `loadLevel(n)`, `giveItem(id)`, `forceWin()`, `forceLose()`). Each phase **extends** it for the systems it adds. This lets the playtest set up and assert complex state headlessly, without reading pixels.
- **`<html data-game-state>`** reflects lifecycle across the full set the design names (e.g. `boot`/`menu`/`playing`/`paused`/`levelcomplete`/`won`/`lost`); **`#game-ready`** marks first paint. The playtest waits on markers, never a settle-timeout.
- **`scenarios.json`** (in `/workspace/game/`) — the suite, in **one canonical schema** the design emits and the harness consumes (do not invent a second). Each scenario:
  ```
  { "name": "m3-stomp-defeats-enemy",
    "setup": ["start", "call:spawnEnemyAt(ENEMY_X)"],     // tokens run to reach the precondition
    "steps": ["hold:right:20", "press:jump", "wait:30"],   // ordered action tokens
    "check": "g.getEnemyCount() === 0 && state === 'playing'" }  // a JS boolean expression
  ```
  **Tokens:** `hold:<input>:<frames>`, `press:<input>`, `release:<input>`, `wait:<frames>`, and `call:<method>(<args>)` (a `window.__game` call); `<input>` names come from the design's control scheme, and symbolic args like `ENEMY_X` are page globals the game exposes, substituted before the run. Note `press:<input>` is a **one-frame tap** (keydown → 1 frame → keyup), so for a game with variable-height / jump-cut a scripted `press:jump` is a **short hop** — use `hold:jump:<frames>` for a full-height jump. **`check`** is one JavaScript boolean expression evaluated in global scope with `g` = `window.__game` and `state` = the current `data-game-state`; **prefer outcome-class checks** (the player scored a stomp *and* kept all lives; `g.getScore() >= <value consistent with the cavern's content>`) over brittle exact-state absolutes (`=== 0`) that other content the milestone introduces (a cavern's own enemies) can violate. **Each phase appends its systems' scenarios; the playtest runs the whole suite every round**, so later phases regression-test earlier ones — the game analogue of build-widget's `journey.json` matrix.

## Procedure

1. **Host prep.** Derive a lowercase-hyphenated `<slug>`. Launch the orchestrator (slow tier) — no repo mount; it scaffolds from the baked template under `/workspace` and delivers to `/out`.

2. **Research** (`sandbox_research`, isolated, open egress) — genre mechanics and control conventions, a feasible scope for the ambition, and an asset approach. Returns `/out/FINDINGS.md`. A well-understood game can skip it.

3. **Game design (the spec that drives everything)** — one medium-tier `sandbox_agent` turns the brief (plus any research) into `/workspace/DESIGN.md`: the core fantasy and the moment-to-moment loop; the full **mechanics and systems** the game needs (movement, combat, inventory, progression, economy, hazards, scoring — whatever applies); the entities and how they interact; the control scheme (keyboard + touch); the complete **`data-game-state` set** and the **`window.__game` surface**; win/lose plus progression/content (levels, waves, difficulty curve); the **asset list**; and — the keystone — the **system decomposition into an ordered list of vertical-slice milestones** (a thin playable core first, then each system layered onto it), with the **scenarios each milestone must satisfy**. This phase **right-sizes the build**: a one-mechanic toy yields a single milestone; a complex game yields many. The decomposition must be **dependency-consistent**: every `window.__game` method and `data-game-state` value a scenario references must be introduced no later than that scenario's milestone, and each scenario's `check` must be satisfiable by the behaviour the spec defines for that milestone — **including the *other* content the same milestone introduces** (a `check` of `getEnemyCount() === 0` is unsatisfiable in a cavern that milestone fills with enemies). Prefer outcome-class checks over brittle absolutes, and avoid internal contradictions (a step the spec says awards no score must not assert the score rose). Everything downstream is driven by `DESIGN.md`.

4. **Plan the phases** — from `DESIGN.md`'s decomposition the orchestrator writes `/workspace/PLAN.md`: the ordered, numbered implementation phases, each with its scope, the `data-game-state` values and `window.__game` methods it introduces, the scenarios it must make pass, and the assets it needs. **Phase 1 is always a thin playable vertical slice** (move + one core interaction + a reachable win/lose) so there is something real to build on and review from the start.

5. **Scaffold, then assets.** First the **orchestrator scaffolds**: a `sandbox_script`, `image=webgamedev`, `egress=none` runs `cp -a /opt/game-template/. /workspace/game` — the template lives only inside the `webgamedev` image, so an agent cannot copy it, and this creates `/workspace/game` for everything downstream. Then generate any assets (image-gen MCP, drawn-primitive generators, or WebAudio) into `/workspace/game/src/assets/` with a manifest (parallelisable; skip for primitive-art games, which fold trivial assets into the phases).

6. **Build the phases — the core loop.** For **each** phase in `PLAN.md` order:
   - **a. Implement** — one medium-tier `sandbox_agent` (`background: true`, polled) **edits the already-scaffolded `/workspace/game/src/` directly with Write/Edit** — it does not scaffold and does not spawn build/playtest grandchildren. It extends `src/` for its slice, wiring the slice's `data-game-state` values and `window.__game` methods, and **writing/updating that phase's scenarios** in `scenarios.json`.
   - **b. Build correctness check** — `sandbox_script`, `image=webgamedev`, `egress=none`: `npm run build` must exit 0.
   - **c. Scenario playtest** — `sandbox_script`, `image=webgamedev`, `egress=none`: run the harness over the **full scenario suite so far**; exit 0. A regression in an earlier system surfaces here, not three phases later.
   - **d. Interspersed review** — one or a small fan-out of medium-tier `sandbox_agent` reviewers (`background: true`, polled) critique **this slice**: does the new system work and feel right, does it integrate cleanly with what's already there, and is the code a sound base for the phases still to come. They propose ranked improvements (not a verdict). The orchestrator applies the worthwhile ones and re-runs **b–c**. Bounded to diminishing returns (≈2–3 rounds per phase); defer the rest to the backlog.
   - **e. Advance** only when this slice's invariants hold and its review is addressed. Log the phase to `/workspace/` for relay into `/out/SUMMARY.md`.

7. **Final cohesion pass** — once all phases land, a whole-game pass over the finished build + the full scenario gallery: the three lenses (**visual/feel**, **controls/feedback**, **scope/completeness**) plus a **cross-system lens** (do the systems combine well; balance, pacing, emergent or full-playthrough issues). Apply worthwhile fixes, re-build + re-playtest, to diminishing returns. This deliberately catches only what the interspersed reviews could not see before assembly — the bulk of review already happened per slice. Log to `/out/IMPROVEMENTS.md`.

8. **Deliver** — the orchestrator itself `cp`s the bundle to `/out` (editable `game/` project minus `node_modules`, plus `dist/`), writes `/out/README.txt` in plain language (open `dist/index.html` or upload `dist/` to itch.io; `npm install && npm run dev` to edit), and `/out/CHANGES.md` (slug, engine, systems, the phases built, scenario count, where each phase's and the final review stopped — diminishing returns vs budget — and the backlog). Print `DONE`.

## The playtest harness (`/workspace/game/playtest.cjs` — the orchestrator writes it once, before phase 1)

The harness is **deterministic by construction**: it drives the game's clock and input itself rather than waiting on wall-time. It is authored up front and does not change per phase — only `scenarios.json` grows. Under-specifying the determinism is the main source of flaky red runs, so the required mechanics are spelled out:

- **Before the game loads** (`addInitScript`), install into the page: a **manual `requestAnimationFrame` queue** (the harness steps frames; the game never free-runs), a **virtual `performance.now()`/`Date.now()`** advanced a fixed `dt` per stepped frame, and a **seeded `Math.random`**.
- **Boot** by stepping the rAF queue until `#game-ready` is attached — never a settle-timeout.
- The game exposes its tuning **constants and entity positions as page globals**, so scenario tokens (`ENEMY_X`, …) substitute symbolically.
- **Reload the page per scenario** (`page.goto` *inside* the loop) so each starts from a clean game and a clock reset to 0. Running the whole suite on one shared page lets accumulated physics bodies and the ever-growing virtual clock perturb later scenarios — the most common cause of a scenario that passes alone but fails after a prefix. `addInitScript` re-installs the deterministic env on every navigation; clear the per-scenario error list after each `goto`.
- After each reload, re-boot to `#game-ready`, apply `setup` then `steps` (set the input, step the declared frames), then evaluate **`check`**: put `g`/`state` on `globalThis` *first*, then indirect-eval `(0, eval)(expr)` so the bound names **and** the game's page globals resolve in one global scope. (Binding `g`/`state` as evaluate-locals while indirect-eval'ing is the own-goal — indirect eval runs in global scope and can't see them.) Screenshot to `/workspace/gallery/<name>.png`.
- **Timing-sensitive scenarios need a probe.** Landing a scripted action on a moving target (a top-stomp on a patrolling enemy) usually needs a short deterministic frame-probe to pick the spawn x / frame counts; budget for it, and assert the **outcome class** (scored a stomp *and* took no damage) rather than exact post-bounce kinematics.
- **Exit non-zero** on any false `check`, unreached marker, or console/page error, naming the failing scenario.

Skeleton (the orchestrator adapts the token→input/`__game` mapping from the design's control scheme):
```js
// playtest.cjs — run under image=webgamedev (Playwright + Chromium). GAME_DIR + OUT are env vars.
const { chromium } = require('playwright');
const scenarios = require('./scenarios.json');
(async () => {
  const browser = await chromium.launch({ args: ['--allow-file-access-from-files', '--no-sandbox'] });
  const page = await browser.newPage();
  const errors = [];
  page.on('pageerror', e => errors.push(String(e)));
  page.on('console', m => { if (m.type() === 'error') errors.push(m.text()); });
  await page.addInitScript(() => {                               // re-runs on EVERY navigation
    let t = 0; const cbs = [];                                   // manual rAF queue + virtual clock from 0
    window.requestAnimationFrame = cb => (cbs.push(cb), cbs.length);
    performance.now = () => t; Date.now = () => t;
    let s = 12345; Math.random = () => (s = (s * 1103515245 + 12345) & 0x7fffffff) / 0x7fffffff;
    window.__step = (n = 1, dt = 1000 / 60) => { for (let i = 0; i < n; i++) { t += dt; cbs.splice(0).forEach(f => f(t)); } };
  });
  for (const sc of scenarios) {
    errors.length = 0;                                           // this scenario's errors only
    await page.goto('file://' + process.env.GAME_DIR + '/dist/index.html');   // fresh state + clock per scenario
    while (!(await page.$('#game-ready'))) await page.evaluate(() => window.__step(1));
    // apply sc.setup then sc.steps via window.__game + input + window.__step(frames) — token mapping here
    const ok = await page.evaluate(expr => {
      globalThis.g = window.__game;                              // bind onto global scope so indirect eval sees them
      globalThis.state = document.documentElement.dataset.gameState;
      return !!(0, eval)(expr);                                  // indirect eval → global scope (g/state + page globals)
    }, sc.check);
    await page.screenshot({ path: process.env.OUT + '/gallery/' + sc.name + '.png' });
    if (!ok || errors.length) { console.error('FAIL', sc.name, errors.join('|')); process.exit(1); }
  }
  console.log('playtest-ok', scenarios.length); await browser.close();
})();
```
The orchestrator relays `/workspace/gallery/` into `/out/gallery/` after each run.

## Launching the orchestrator

- Tier: **slow** orchestrator; **medium** design/implement/review/fix children; `build`/`playtest` are `sandbox_script` (`image=webgamedev`, `egress=none`). No `directories:` mount.
- Tell it explicitly: **do NOT build or playtest yourself** (the agent image has no toolchain or browser); **the orchestrator scaffolds the template once via a `webgamedev` `sandbox_script`** before phase 1 (it lives only in that image), and implement agents then edit `src/` directly; **`DESIGN.md` drives how many phases there are** — right-size it, don't force a fixed count.
- Brief it as a complete document: the concept and what a good version at this ambition looks like; the test API + the **one canonical scenario schema**; the **deterministic `playtest.cjs` skeleton** (manual rAF/clock, indirect-eval `check`); the per-phase loop with the interspersed review and the sequential/one-src-writer rule; `background: true` + `sandbox_wait` for all agent children; and the output contract.

## Host-side delivery

No branch or PR — the bundle in `/out` is the deliverable, and the in-sandbox build+playtest is authoritative; the host does not rebuild.

1. Read `/out/CHANGES.md` and `/out/IMPROVEMENTS.md`; confirm `playtest-ok` and check why the final pass stopped — diminishing returns is the goal; if it stopped on a budget, surface the backlog.
2. Surface the bundle: give the user the `<output_dir>` path and offer to copy it where they want. Show a screenshot or two from `/out/gallery/`. Note that the built game's module script needs serving over HTTP (e.g. a local static server), not `file://`, for the recipient to play it in Chrome.
3. Tell them how to **play** (serve `dist/` or upload to itch.io) and how to **edit** (`npm install && npm run dev`).

## Output contract

```
/out/
  game/               # the editable TypeScript project (src/ incl. scenarios.json + playtest.cjs, package.json, vite/ts config, assets) — minus node_modules
  dist/               # the built, runnable game (offline, mobile-ok, itch.io-ready)
  gallery/            # per-scenario frames across the build
  FINDINGS.md         # research notes (only if step 2 ran)
  DESIGN.md           # the detailed spec: mechanics, systems, state set, window.__game surface, asset list, milestone decomposition
  PLAN.md             # the ordered phases derived from DESIGN.md, each with scope, markers, scenarios, assets
  SUMMARY.md          # per implementation phase
  IMPROVEMENTS.md     # interspersed per-phase reviews + the final cohesion pass + the remaining-opportunities backlog
  README.txt          # plain-language: how to play, how to edit
  CHANGES.md          # slug, engine, systems, phases built, scenario count, review stop reasons + backlog
```
