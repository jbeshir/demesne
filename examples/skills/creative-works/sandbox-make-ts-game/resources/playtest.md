# TypeScript game build and playtest contract

Read this file before creating validation and before final validation. `SKILL.md` owns orchestration.

## Repository and evidence

Work only in `/workspace/repo`. Preserve the native package manager and lockfile. For greenfield work copy the pinned `/opt/game-template` from a `webgamedev` child, or create an equivalent project containing `package.json`, a lockfile, TypeScript/Vite configuration, `src/`, `public/`, and reproducible `build`/`playtest` scripts.

Maintain two self-contained, non-module build modes from the same source. The instrumented test build is `dist-test/index.html`; it may expose the test surface below but must inline all JavaScript, CSS, and runtime assets as classic/IIFE content or data URLs so the harness can execute it directly over `file://`. It is evidence, not the player-facing artifact. The clean production build is `dist/index.html`: configure `base: './'` and a lockfile-pinned single-file bundling plugin or equivalent transform. It must contain no test surface, module-script element, external script/stylesheet/media URL, or required network fetch. Both exact files must execute at their documented `file:///workspace/repo/...` paths; localhost or a preview server is not an acceptance substitute.

Record exact build/playtest commands plus Node, package-manager, Playwright package, Chromium, TypeScript, Vite, bundler/plugin, and immutable `webgamedev` image/build identifiers when exposed. Never depend on unrecorded globals or unpinned network installs.

## Runtime surface

Gate all instrumentation behind one explicit build-time flag (for example `GAME_TEST_BUILD=1`) that defaults off. Only `dist-test/index.html` exposes `#game-ready` after the first usable frame, lifecycle state in `document.documentElement.dataset.gameState`, and `window.__game.getState()` plus the deterministic setup/query methods named in `DESIGN.md`. Hooks may arrange setup but must not replace real DOM/canvas input. Production must omit `window.__game`, `#game-ready`, `data-game-*`, test-only selectors/adapters, virtual-clock entry points, and the flag name/value or other reserved test markers from its emitted HTML and executable text; dead or disabled hook code is not acceptable.

## Canonical scenario schema

Use one versioned JSON object at `test/scenarios.json`:

```json
{
  "schemaVersion": 1,
  "scenarios": [{
    "name": "slice-01-reaches-win",
    "kind": "win",
    "start": "fresh",
    "setup": ["call:start()"],
    "actions": ["hold:right:30", "press:interact", "tap:#pause", "wait:2"],
    "expect": {
      "state": "won",
      "check": "g.getScore() >= 1",
      "visible": ["canvas", "#game-ready"]
    },
    "screenshot": "slice-01-reaches-win.png"
  }]
}
```

Require unique `name` and `screenshot` values. `kind` is one of `boot`, `core`, `win`, `loss`, `input`, `pause`, or `regression`; require exactly one `boot` and at least one `core`, all with no `call:` setup/action. `start` is either `fresh` (new navigation with cleared origin storage, cookies, service workers, and browser context state) or `reload` (reload the scenario's initial URL after the same clearing); both reinstall deterministic init scripts, reset the virtual clock/RNG, and wait for readiness before setup. Do not carry state between scenarios.

Define input names and their keyboard `KeyboardEvent.code`/key mapping in `DESIGN.md`. Support these closed tokens:

- `hold:<input>:<frames>`: dispatch mapped keydown, step frames, then keyup.
- `press:<input>`: dispatch mapped keydown, step one frame, then keyup.
- `release:<input>`: dispatch the mapped keyup.
- `tap:<target>`: pointer down/up and click at the center of a visible selector, with one frame between phases.
- `pointer:<down|move|up>:<pointerId>:<target>`: dispatch a primary pointer with that ID at a visible selector center or normalized canvas position `@<x>,<y>` where each coordinate is in `[0,1]`.
- `touch:<start|move|end|cancel>:<touchId>:<target>`: dispatch a touch with that ID at the same selector/normalized-position grammar; retain active touches so multi-touch tokens are representable.
- `wait:<frames>`: step the deterministic clock/frame queue.
- `call:<window.__game method>(<JSON-compatible arguments>)`: invoke a declared hook.

Reject malformed targets, unknown inputs/tokens, duplicate active IDs, or unmatched pointer/touch endings. Add boot, core interaction, win, loss, keyboard, every supported pointer/touch mode, pause/resume when present, and every slice regression. Exercise at least one real DOM/canvas event route for each supported control mode; a `call:` hook cannot satisfy input coverage. The itemized coverage is the minimum; combine categories in one scenario only when every applicable assertion remains explicit.

## Deterministic harness

Implement `test/playtest.cjs` with Playwright. Launch Chromium with these required rootless and local-file arguments (additional game-specific arguments are allowed): `['--no-sandbox', '--disable-setuid-sandbox', '--disable-dev-shm-usage', '--disable-gpu', '--allow-file-access-from-files']`. Before application code loads, use `page.addInitScript` to install seeded `Math.random`, virtual `Date.now`/`performance.now`, and one manual 60 Hz scheduler for `requestAnimationFrame`, `setTimeout`, `setInterval`, and their cancellation APIs; reinstall it on navigation.

The scheduler assigns every RAF/timer a monotonically increasing registration sequence. One stepped frame advances the clock by exactly `1000/60` ms, runs all due timers in `(dueTime, registrationSequence)` order (newly due zero-delay timers join that same drain; impose and fail on a finite runaway cap), then snapshots and runs that frame's RAF callbacks in registration order. Timer callbacks scheduled by RAF wait until the next frame; intervals reschedule from their prior due time and cancellation takes effect immediately. `wait:<frames>` and all input tokens that step frames use only this scheduler; no game timer may depend on wall time.

For every scenario, clear errors; establish its `start` by navigating to the exact `file:///workspace/repo/dist-test/index.html`; bound readiness by stepped frames; apply setup/actions; assert state and visible selectors; capture its PNG; and append `{scenario, pass, state, error, consoleErrors, screenshot}`.

Evaluate `expect.check` wholly in the browser page realm with `page.evaluate`. Inside that callback bind local constants `g = window.__game` and `state = document.documentElement.dataset.gameState`, execute the supplied expression via a function whose only explicit parameters are `g` and `state`, and require a boolean result. Do not bind Node `globalThis`, expose Node objects, or serialize executable values across realms. Catch page-realm syntax/runtime exceptions and record them as assertion failures. Expressions may read only page globals and the two bindings and must not mutate state; prefer narrow query methods and outcome assertions.

Navigate/reload per scenario so bodies, timers, storage, and clocks cannot leak. Exit nonzero for any token, timeout, assertion, selector, screenshot, page, or console failure. Always atomically write `artifacts/playtest-report.json`, including fatal harness errors, as `{schemaVersion: 1, pass, scenarios, error}` where `scenarios` contains the per-scenario records above, `pass` is true only when every declared scenario ran and passed, and `error` is null on success or the fatal harness error. Print `playtest-ok <count>` only when `pass` is true.

## Commands and acceptance

Provide lockfile-backed package scripts `build:test` (flag on, output `dist-test/`) and `build` (flag off, output `dist/`), plus dedicated `test/validate-production.cjs`. Run these three separate nested `sandbox_script` calls with `image: "webgamedev"`, `egress: "none"`:

```sh
cd /workspace/repo && rm -rf dist-test artifacts/gallery artifacts/playtest-report.json && mkdir -p artifacts/gallery artifacts/logs && npm run build:test
```

```sh
cd /workspace/repo && node test/playtest.cjs
```

```sh
cd /workspace/repo && rm -rf dist artifacts/production-validation-report.json && npm run build && node test/validate-production.cjs dist/index.html
```

Substitute lockfile-backed equivalents and record them. If dependencies are absent, perform one pinned restore with `egress: "package-managers"`; never install `latest`.

After each exact final nested call, copy its captured combined stdout/stderr without changing the exit status: instrumented build to `artifacts/logs/test-build.log`, playtest to `artifacts/logs/playtest.log`, and the chained clean build/validation call to `artifacts/logs/production-validation.log`.

For a synchronous script, require `exit_code == 0`; for a background script, first require terminal `status == "succeeded"`, then `exit_code == 0`. Also require both HTML outputs, every declared screenshot, and valid reports with top-level `pass: true` (and every playtest scenario passing). Route infrastructure/tool failures through the skill's exactly-one fresh-name retry. Route nonzero commands and failing reports through its bounded preserve/fix/retest loop.

`test/validate-production.cjs` has two mandatory phases. First, parse the exact bytes of its path argument and fail on any forbidden test marker above, `<script type="module">`, non-inline script/style, external or non-`data:` stylesheet/media reference, or runtime network URL. Second, load and validate the scenario schema, select the `boot` and every `core` scenario by `kind`, and fail if their required count or hook-free constraint is violated. Launch Chromium with the harness's required arguments in a fresh context, disable networking before navigation (`context.setOffline(true)` and abort every `http:`/`https:` request), navigate to the exact absolute `file://` path, and wait for a player-visible boot condition identified in `DESIGN.md` without using test hooks. Replay those scenarios' real-input actions, assert their player-visible DOM/canvas outcome (DOM state or screenshot/pixel predicate specified in `DESIGN.md`), and require zero page/console errors and zero network requests. The smoke may use bounded Playwright waits because production has no virtual-clock hook.

The validator must atomically write `artifacts/production-validation-report.json` even on fatal failure, with `{pass, file, sha256, static:{pass, violations}, smoke:{pass, boot, coreInput, networkRequests, pageErrors, consoleErrors}, error}`; print `production-validation-ok <sha256>` only on success and exit nonzero otherwise. Final validation deletes `dist-test/`, `dist/`, gallery, and both reports, then runs the three commands above in order. Preserve the production file after validation and deliver the complete repository, not the instrumented artifact alone.

## Final-delivery manifest

Every path is relative to `/workspace/repo` unless it begins with `/out`. A file entry must exist and be nonempty; a directory entry must contain every listed child. Conditions are closed and explicit.

| Path | Condition | Gate |
|---|---|---|
| `DESIGN.md`, `PLAN.md`, `WORKLOG.md`, `README.md` | Always | Nonempty; `README.md` contains offline play and editing instructions. |
| `package.json`, the repository's single recognized lockfile, `src/`, `test/scenarios.json`, `test/playtest.cjs`, `test/validate-production.cjs` | Always | Nonempty; exactly one lockfile; `src/` contains at least one source file; package scripts implement distinct test and production modes. |
| `dist-test/index.html` | Always | Nonempty self-contained non-module instrumented artifact; full scenario suite passes against this exact `file://` file. |
| `dist/index.html` | Always | Nonempty clean self-contained non-module file; dedicated static absence gate and networking-disabled exact-`file://` boot/core-input smoke pass. |
| `artifacts/playtest-report.json` | Always | Valid JSON, `pass: true`, and every scenario passes. |
| `artifacts/production-validation-report.json` | Always | Valid JSON with `pass`, `static.pass`, and `smoke.pass` true; `file` identifies exact `dist/index.html`, SHA-256 matches it, `error` is null, and `violations`, `networkRequests`, `pageErrors`, and `consoleErrors` are empty. |
| `artifacts/gallery/<scenario.screenshot>` | For every scenario object in `test/scenarios.json` | Nonempty PNG; no other screenshot path is required. |
| `artifacts/reviews/final.md` | Always | Nonempty independent final review and stop reason. |
| `artifacts/reviews/round-1.md`, `round-2.md`, `round-3.md` | Exactly for each review/fix round actually run, numbered contiguously from 1 | Nonempty; no later-numbered file without all earlier files. |
| `artifacts/logs/test-build.log`, `artifacts/logs/playtest.log`, `artifacts/logs/production-validation.log` | Always | Nonempty logs from the three exact final commands. |
| `THIRD_PARTY.md` | Always | Nonempty dependency and asset provenance/license inventory; explicitly states `none` for each empty category. |
| `CONTENT-NOTES.md` | Iff the game includes flashing, intense motion, violence, horror, substance use, sexual content, or other audience-sensitive material identified in `DESIGN.md` | Nonempty and names the applicable material; otherwise the path is not required. |
| `/out/SUMMARY.md`, `/out/repo` | Always | Summary is nonempty; `/out/repo` is a complete copy containing every applicable repository entry above. |

The manifest is exhaustive. Generated caches, dependency directories, and intermediate files are not delivery requirements.
