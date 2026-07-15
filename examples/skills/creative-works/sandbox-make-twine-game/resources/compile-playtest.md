# Twine compile and playtest contract

Read this file before the vertical slice and before final validation. `SKILL.md` owns orchestration.

## Repository and evidence

Work only in `/workspace/repo`. Use Twee 3 as editable master. Greenfield work must create `DESIGN.md`, `PASSAGE-GRAPH.md`, `WORKLOG.md`, one or more `story/*.twee` files, `test/routes.json`, `test/build-story.cjs`, `test/validate-graph.cjs`, `test/playtest.cjs`, and `test/assert-production-clean.cjs`.

Require exactly one `StoryTitle`, one `StoryData`, and one start passage. Give `StoryData` a stable IFID and a format/version verified by `tweego --list-formats`. Use Harlowe by default. Choose SugarCube only for a named requirement verified as unsupported or materially impractical in the pinned Harlowe version; record the requirement and verified versions in `DESIGN.md`.

Use `:: Passage Name` and `[[label->Target]]`; document tags/macros. A test build may inject stable passage/choice IDs and fixture adapters; production must derive from the same story sources with all test-only material disabled. Never use rendered-text hashes as identity. Store media locally with provenance/license; published HTML must not need a network.

Record exact commands and final `tweego --version`, `tweego --list-formats`, `node --version`, Playwright package/browser/Chromium versions, selected story format/version, and immutable `twine` image/build identifier when exposed in `WORKLOG.md` and `/out/SUMMARY.md`.

Declare every Node dependency used by the test scripts in `package.json` and the repository's single recognized lockfile. Prefer tooling already pinned in the `twine` image; if a declared dependency is absent, perform one lockfile-pinned restore with `egress: "package-managers"`, then return to `egress: "none"`. Never install `latest` or rely on an unrecorded global package.

## Static graph report

Maintain `PASSAGE-GRAPH.md` and always atomically write `artifacts/graph-report.json`, including parse, metadata, validation, or Tweego compile failures:

```json
{
  "schemaVersion": 1,
  "pass": true,
  "errors": [],
  "compile": {"pass": true, "exitCode": 0, "error": null},
  "start": "Start",
  "planned": ["Start", "Decision", "Ending A"],
  "endings": ["Ending A"],
  "passages": [{"id":"p-start","name":"Start","outgoing":["Decision"],"incoming":[]}],
  "dynamicAnnotations": [{"source":"Decision","targets":["Ending A"],"condition":"$hasKey is true","fixtures":["locked-route"],"covered":true}],
  "duplicatePassages": [],
  "malformedMetadata": [],
  "brokenTargets": [],
  "unreachable": [],
  "unapprovedDeadEnds": [],
  "inescapableCycles": [],
  "allReachableCanReachEnding": true,
  "routeFixtures": [{"name":"locked-route","covered":true,"pass":true,"errors":[]}]
}
```

Make `pass` true only when compile passes; `errors`, duplicates, malformed metadata, broken targets, unreachable passages, unapproved dead ends, and inescapable cycles are empty; every reachable passage can reach an ending; and every referenced route fixture is present and statically valid. Exit nonzero after writing on false `pass`.

Parse every `.twee` source. Reject duplicate names and malformed required metadata; resolve static targets; compare plan and implementation; compute reachability; find undeclared terminals and strongly connected components without ending routes. Never omit an unresolved edge silently.

Dynamic navigation has one canonical source grammar: a standalone Twee comment in the source passage, `<!-- demesne-edge {JSON} -->`, where `{JSON}` is exactly `{"targets":["Passage Name"],"condition":"human-readable predicate","fixtures":["route-name"]}`. All three keys are required; arrays are nonempty with unique strings; no other keys are allowed. The containing passage supplies `source`; each target and fixture must resolve. Place one comment immediately before each macro/script expression whose target cannot be statically resolved, and one comment per distinct condition when target sets differ. The validator normalizes these into `dynamicAnnotations` entries with `source`, sorted `targets`, exact `condition`, sorted `fixtures`, and `covered`; unknown/malformed annotations, dynamic navigation without an immediately preceding annotation, unused annotations, or annotations without passing fixture coverage fail validation.

## Canonical routes

Use one versioned `test/routes.json`:

```json
{
  "schemaVersion": 1,
  "routes": [{
    "name": "locked-route",
    "format": "harlowe",
    "initial": {"variables": {"hasKey": true}, "history": [{"choiceId":"choice-take-key","from":"p-start","to":"p-decision"}]},
    "actions": [{"choiceId": "choice-open-door", "label": "Open the door"}],
    "expect": {"passageIds": ["p-start", "p-vault"], "endingId": null},
    "screenshot": true,
    "stateProjection": ["hasKey"]
  }]
}
```

Require unique fixture names and stable choice IDs injected only in test builds. `format` must match the story. `initial.variables` contains JSON values. `initial.history` is an ordered array of transitions, each exactly `{choiceId, from, to}` using existing stable IDs. Starting from the declared start passage, replay each real choice; require its observed source and destination to equal `from` and `to`, then apply `initial.variables`. `actions` uses stable `choiceId` with `label` only as a diagnostic cross-check. Expected IDs must exist; `endingId` is null or declared. `screenshot` is required: when true, capture nonempty `artifacts/gallery/route-<fixture-name>.png`; when false, require no fixture-specific screenshot. Record the validated history transitions and observed replay in both reports.

Implement format adapters in the page realm: for Harlowe, reach setup state through a dedicated test-only startup passage/hook using documented Harlowe variable operations; for SugarCube, use a test-only startup passage/hook that applies `State.variables` and history. Never mutate undocumented runtime internals or use adapters in production builds. Record each fixture in both reports and fail uncovered annotations, setup errors, action mismatches, or expectation failures.

Define each visited-state identity as stable passage ID plus a canonical JSON projection of all variables and history that can affect branching. Declare that projection in `DESIGN.md` and each fixture's `stateProjection`; sort object keys and preserve array/history order. Text hashes are diagnostic only.

## Browser traversal

Use Playwright in `twine`; launch Chromium with `--no-sandbox`, `--disable-setuid-sandbox`, `--disable-dev-shm-usage`, `--disable-gpu`, and `--allow-file-access-from-files`; freeze `Date`, seed `Math.random`, and capture page/console errors.

1. Load the instrumented `file:///workspace/repo/artifacts/test-build/index.html` and wait with bounded DOM conditions.
2. Read stable passage/state identity and discover accessible actionable choices (`tw-link`, `[data-passage]`, or format equivalent), including injected stable choice IDs.
3. Explore breadth-first. For every route prefix, create a new page, clear storage, reload the built story, apply its fixture adapter, and deterministically replay the prefix from stable choice IDs. Do not restore opaque browser snapshots.
4. Cap traversal at 2,000 clicks and fail explicitly if exceeded. Fail unchanged/error targets, undeclared terminals, page/console errors, fixture failures, or identity collisions.
5. Compare rendered reachability/endings with the graph report. Exercise every planned passage, ending, fixture, conditional route, and cycle exit.
6. Capture start and one nonempty screenshot per distinct ending.
7. Always atomically write `artifacts/playtest-report.json`, even on fatal failure, with `schemaVersion`, `pass`, visited state identities, choices, endings, fixture results/coverage, errors, and screenshots. Print `playtest-ok` only on complete success.

## Commands and acceptance

Run separate nested scripts with `image: "twine"`, `egress: "none"`. `TWINE_TEST_BUILD=1` is the sole build-mode switch: the source preprocessing/build step must include stable IDs and adapters only when it equals `1`, and must remove/reject them otherwise. Clear stale test output/reports first, capture Tweego's status, and run the validator even when compilation fails:

```sh
cd /workspace/repo && rm -rf artifacts/test-build artifacts/graph-report.json artifacts/tweego-exit-code && mkdir -p artifacts/test-build && (TWINE_TEST_BUILD=1 node test/build-story.cjs artifacts/test-build/index.html; printf '%s\n' "$?" > artifacts/tweego-exit-code) ; node test/validate-graph.cjs
```

`test/build-story.cjs` must deterministically preprocess the canonical `story/` sources for the selected mode and invoke pinned-image Tweego; it may write temporary generated source only beneath `artifacts/test-build/` and must not edit `story/`. The validator must read `artifacts/tweego-exit-code`, include compile failure in the graph report, and exit nonzero when Tweego or graph validation fails.

```sh
cd /workspace/repo && rm -rf artifacts/gallery artifacts/playtest-report.json && mkdir -p artifacts/gallery && node test/playtest.cjs
```

After the instrumented graph/browser gates pass, build a clean production artifact and gate absence before smoke-testing real choices:

```sh
cd /workspace/repo && rm -rf dist && mkdir -p dist && TWINE_TEST_BUILD=0 node test/build-story.cjs dist/index.html && node test/assert-production-clean.cjs dist/index.html && TWINE_SMOKE_ONLY=1 node test/playtest.cjs
```

`test/assert-production-clean.cjs` must fail on every reserved test passage/tag, adapter global, test startup hook, `data-demesne-passage-id`, `data-demesne-choice-id`, or other marker enumerated by `test/build-story.cjs`; it also requires a self-contained, network-independent HTML. Smoke mode loads the exact production file URL, disables fixture adapters, follows at least one real choice from start to a declared ending through rendered links, captures no test-only IDs, and fails on page/console errors. It writes `artifacts/production-smoke-report.json` atomically with `pass`, visited passage names, chosen visible labels, ending, errors, and test-marker scan results.

After each exact final nested call, copy its captured combined stdout/stderr (without changing the command's exit status) to the corresponding manifest log: instrumented build/graph to `artifacts/logs/test-build.log`, traversal to `artifacts/logs/playtest.log`, and production build/absence/smoke to `artifacts/logs/production-build-smoke.log`.

For a synchronous script, require `exit_code == 0`; for a background script, first require terminal `status == "succeeded"`, then `exit_code == 0`. Also require all three reports to parse with `pass: true`, every graph invariant to hold, every fixture to pass, and every applicable manifest entry below. Route infrastructure/tool failures through exactly one fresh-name retry. Route nonzero commands and failing reports through the bounded preserve/fix/retest loop.

Final validation must delete `dist/`, `artifacts/test-build/`, all three reports, and gallery; reproduce the instrumented test build and its graph/browser evidence first, then separately reproduce, absence-gate, and smoke-test clean production. Deliver the complete repository, not only HTML; never deliver `artifacts/test-build/index.html` as the game.

## Final-delivery manifest

Every path is relative to `/workspace/repo` unless it begins with `/out`. A file entry must exist and be nonempty; a directory entry must contain every listed child. Conditions are closed and explicit.

| Path | Condition | Gate |
|---|---|---|
| `DESIGN.md`, `PASSAGE-GRAPH.md`, `WORKLOG.md`, `README.md` | Always | Nonempty; `README.md` contains play and editing/recompile instructions. |
| `package.json`, the repository's single recognized lockfile | Always | Nonempty; exactly one lockfile; every Node dependency used by `test/*.cjs` is pinned. |
| `story/` | Always | Contains one or more nonempty `.twee` files and no generated test source. |
| `test/routes.json`, `test/validate-graph.cjs`, `test/playtest.cjs`, `test/build-story.cjs`, `test/assert-production-clean.cjs` | Always | Nonempty canonical validation/build inputs. |
| `dist/index.html` | Always | Clean production HTML; absence gate and production smoke pass. |
| `artifacts/graph-report.json`, `artifacts/playtest-report.json`, `artifacts/production-smoke-report.json` | Always | Valid JSON with `pass: true`. |
| `artifacts/gallery/start.png` | Always | Nonempty PNG. |
| `artifacts/gallery/ending-<endingId>.png` | For every distinct declared ending ID in `DESIGN.md` | One nonempty PNG per ending; `<endingId>` is the exact stable ID using only `[A-Za-z0-9_-]+`. |
| `artifacts/gallery/route-<fixture-name>.png` | For every route fixture with `screenshot: true` | One nonempty PNG; `<fixture-name>` is the exact fixture name using only `[A-Za-z0-9_-]+`. |
| `artifacts/reviews/final.md` | Always | Nonempty independent final review and stop reason. |
| `artifacts/reviews/round-1.md`, `round-2.md`, `round-3.md` | Exactly for each review/fix round actually run, numbered contiguously from 1 | Nonempty; no later-numbered file without all earlier files. |
| `artifacts/logs/test-build.log`, `artifacts/logs/playtest.log`, `artifacts/logs/production-build-smoke.log` | Always | Nonempty logs from the three exact final commands. |
| `THIRD_PARTY.md` | Always | Nonempty format, dependency, and media provenance/license inventory; explicitly states `none` for each empty category. |
| `CONTENT-NOTES.md` | Iff the story includes violence, horror, substance use, sexual content, or other audience-sensitive material identified in `DESIGN.md` | Nonempty and names the applicable material; otherwise the path is not required. |
| `/out/SUMMARY.md`, `/out/repo` | Always | Summary is nonempty; `/out/repo` is a complete copy containing every applicable repository entry above. |

`artifacts/test-build/index.html` is required only while running validation; remove `artifacts/test-build/` after reports and logs are finalized and before copying `/out/repo`. The manifest is exhaustive; generated caches and intermediate files are not delivery requirements.
