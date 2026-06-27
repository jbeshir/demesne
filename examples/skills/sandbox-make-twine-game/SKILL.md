---
name: sandbox-make-twine-game
description: Build a playable branching interactive-fiction game (Twine / choose-your-own-adventure) from a concept through a demesne pipeline and deliver a durable bundle the recipient can play and keep editing. A slow-tier orchestrator runs research → plan the passage graph → write the Twee 3 source → compile to a self-contained HTML with Tweego → an authoritative offline playtest gate that walks the whole link graph (every choice resolves, no orphan or unreachable passages, every path reaches an ending) → an open-ended story-quality improvement cycle that iterates until improvements run dry, then a host-side delivery of the editable .twee source plus the published HTML. Apply when the user wants a text/narrative game, gamebook, or interactive story made for them. Triggers include "make me a choose-your-own-adventure about X", "build an interactive story", "a text adventure where the player decides Y", "a Twine game". Skip for coded real-time/arcade games (use sandbox-make-ts-game), a slide deck (use sandbox-make-slide-deck), and plain non-interactive prose (a chat box serves it).
allowed-tools: Read, Glob, Grep, Bash, Write, Edit, mcp__demesne__sandbox_agent, mcp__demesne__sandbox_research, mcp__demesne__sandbox_script
---

Turn a concept into a finished, playable Twine interactive-fiction game and hand the recipient something they can both **play** (a self-contained `index.html` that opens offline and on a phone) and **keep editing** (the `.twee` source, importable into the Twine editor). A slow-tier orchestrator runs research → plan → write → compile → continuous correctness checks → an open-ended quality improvement cycle, copies the bundle to `/out`, and the host delivers it. The deliverable is a durable artifact bundle, not a repo branch.

The output is *useful to the recipient*, not a demo of demesne: a real story they can publish (itch.io, Neocities, any static host) and a source they can grow. Foreground the story; keep the toolchain hidden.

## The improvement cycle (not a gate)

Quality here is *pursued*, not merely passed. Keep two things distinct:

- **Correctness invariants** — it compiles; no broken links; no orphan or unreachable passages; every path reaches an ending; no console errors. These are hard constraints: a violation means the artifact is *broken*, not merely unpolished. They must hold after every change — but holding them is the floor, never the finish line.
- **Quality** — prose, choice design, branch richness, pacing, tone, presentation. Pursue it through an open-ended improvement cycle: each round actively hunts for the highest-value improvements and applies them. Do **not** ask "is this good enough?" and stop. End the loop on *diminishing returns* (a round that finds nothing worth doing, or two rounds with nothing materially new), or on a round budget reached as a safety stop — in which case deliver the outstanding-improvements backlog rather than calling it done. Stop because little is left worth improving, not because a minimum bar was cleared.

**Watch out (cross-cutting):** the orchestrator must `cp` the final bundle to its own `/out` itself — a `sandbox_script` child writes only to `/out/child/<name>` and would strand the artifact there. `/workspace` is torn down on exit; only `/out` persists. The playtest correctness check is non-negotiable: a story with a dead link or an unreachable passage is broken, no matter how good the prose — never deliver a bundle whose playtest exited non-zero.

## Story format and tooling

- **Compiler: Tweego**, baked into the `twine` image with the standard story formats on `TWEEGO_PATH` — so compile runs at `egress=none`. (The `twine` image is the Playwright base + Tweego + story formats; it shares the browser base with `webgamedev` but carries none of the TS toolchain.) Compile the source tree with `tweego -o /workspace/dist/index.html /workspace/story` (Tweego reads `:: StoryData` for the format; pin it there).
- **Story format: Harlowe** by default (the Twine-editor default, friendliest for a non-coder to keep editing). Choose **SugarCube** only when the game needs saves, inventory, variables, or stats — say which in the plan and set it in `:: StoryData`.
- **Source: Twee 3** (`.twee`) — one passage per `:: PassageName`, links as `[[choice->Target]]`. This is the editable master; it imports into the Twine desktop/web editor for visual editing.

## Procedure

1. **Host prep.** Derive a lowercase-hyphenated `<slug>` from the concept. Launch the orchestrator (slow tier) — no repo mount is needed; it authors everything under `/workspace` and delivers to `/out`.

2. **Research** (`sandbox_research`, isolated, open egress) — only when the story leans on real material (a historical setting, a real place, a licensed world's tone). It returns setting/voice/factual notes to `/out/FINDINGS.md`. Pure fiction skips this step.

3. **Plan the passage graph** — the orchestrator writes `/workspace/PLAN.md`: premise and tone, the cast, the **state graph** (key passages, the choices out of each, the branch/merge structure, and every ending), the story format + why, and the target length (passage count). A flat "10 linked scenes" is fine for a first game; note any variables/inventory if SugarCube.

4. **Write the Twee source** — one medium-tier `sandbox_agent` writes `/workspace/story/story.twee`: a `:: StoryData` passage (format + IFID), a `:: StoryTitle`, and the passages with real branching per the plan. Distinct, meaningful choices; no choice that silently dead-ends; every branch reaches an ending. Optional styling via a `:: Story Stylesheet` passage and generated art via the image-gen MCP into `/workspace/story/`. Write `/out/SUMMARY.md`.

5. **Compile gate** — a `sandbox_script` child, `name=compile`, `image=twine`, `egress=none`. Run `tweego -o /workspace/dist/index.html /workspace/story`. It must exit 0 and produce a non-empty `index.html`. On failure (malformed Twee, unknown format) spawn a fix phase and recompile.

6. **Playtest correctness check** — a `sandbox_script` child, `name=playtest`, `image=twine`, `egress=none`. Run the link-graph harness (below) over `/workspace/dist/index.html`. It must exit 0, write `playtest-ok` and `playtest-report.json`, and screenshot the start plus each ending to `/out/gallery/`. A non-zero exit is a real defect — a broken link, an orphan or unreachable passage, or a path that never ends — so fix, recompile, and re-playtest until it holds. This is a correctness invariant that must stay true through every improvement round below — not the finish line.

7. **Improvement cycle** — keep making the story better until it stops being worth it. Each round, a medium-tier `sandbox_agent` plays the current build and the report and proposes the **best available improvements** — coherence, choice quality, branch richness, pacing, tone, presentation, dead-end *feel* (choices technically valid but pointless) — ranked by value, as proposals, not a pass/fail verdict. The orchestrator applies the worthwhile ones, re-runs compile + the correctness check, and goes again. Continue until a round finds nothing worth doing or only trivial nits (diminishing returns), or a safety budget (≈5 rounds) is reached — on which, record what remains. Log each round's changes and the final outstanding-improvements backlog to `/out/IMPROVEMENTS.md`.

8. **Deliver** — the orchestrator itself `cp`s the bundle to `/out` (see Output contract), writes `/out/README.txt` in plain language (what the game is, double-click `index.html` to play, import `story.twee` into Twine at twinery.org to edit), and `/out/CHANGES.md` (slug, story format, passage/ending counts, playtest summary, how many improvement rounds ran and whether the cycle stopped on diminishing returns or the budget, and any backlog). Print `DONE`.

## The playtest harness (Playwright, the orchestrator writes it to `/workspace/playtest.cjs`)

A published Twine story is a single HTML page that swaps passages in the DOM. The harness loads `file://…/index.html` (`--allow-file-access-from-files`), then does a graph crawl driven by the rendered links, not by reading the source:

- From the start passage, record the visible passage name and its choice links (the `<tw-link>` / `[data-passage]` anchors the active format renders).
- Breadth-first: click each unvisited choice, wait for the passage to change, record the new passage and its links; back up and continue until the reachable graph is exhausted (cap total clicks to a sane bound, e.g. 2000).
- **Fail (exit non-zero)** if any clicked choice changes nothing or errors (broken link), if the crawl reaches a non-ending passage with zero outgoing choices that the plan didn't mark as an ending (dead end), or if a console/page error fires. Write the set of reachable vs. planned passages to `playtest-report.json` so unreachable/orphan passages (in the source, never reached) surface as a diff.
- Screenshot the start passage and each ending passage to `/out/gallery/`.

Freeze `Date`/`Math.random` for stable shots. Keep it format-agnostic where possible (Harlowe and SugarCube render links slightly differently — select on the rendered anchor role, and branch on format only if needed).

## Launching the orchestrator

- No `directories:` mount required (greenfield authoring). Tier: **slow** orchestrator; **medium** write/review/fix phases; `compile`/`playtest` are `sandbox_script` (`image=twine`, `egress=none`).
- Tell it explicitly: **do NOT compile or playtest yourself** — the agent image has no Tweego or browser; those are `sandbox_script` children on `image=twine`.
- Brief it as a complete document: the concept and what a good version feels like; the story-format choice rule (Harlowe default, SugarCube for state); the Twee 3 conventions and the `:: StoryData` format pin; the eight steps with the child-naming rule; the playtest harness contract; and the output contract.

## Host-side delivery

There is no branch or PR — the bundle in `/out` is the deliverable, and the in-sandbox playtest is authoritative; the host does not recompile.

1. Read `/out/CHANGES.md` and `/out/IMPROVEMENTS.md`; confirm `playtest-ok` is present (the correctness invariant held) and check why the improvement cycle stopped — diminishing returns is the goal; if it stopped on the budget, surface the backlog to the user.
2. Surface the bundle: give the user the `<output_dir>` path, and offer to copy it somewhere convenient (`sandbox_download`, or a `cp` to a path they name). Show a screenshot or two from `/out/gallery/`.
3. Tell them, in one line, how to **play** (open `index.html`) and how to **edit** (import `story.twee` at twinery.org or in the Twine app).

## Output contract

```
/out/
  story.twee          # the editable master — imports into the Twine editor
  index.html          # the published, self-contained playable game (offline, mobile-ok)
  gallery/            # start + each ending screenshot
  playtest-report.json# reachable vs planned passages, choice count, any errors
  FINDINGS.md         # research notes (only if step 2 ran)
  PLAN.md             # premise, cast, state graph, endings, format choice
  SUMMARY.md          # per write/fix phase
  IMPROVEMENTS.md     # per-round changes + the remaining-opportunities backlog
  README.txt          # plain-language: how to play, how to edit
  CHANGES.md          # slug, format, passage/ending counts, playtest summary, improvement rounds + stop reason + backlog
```
