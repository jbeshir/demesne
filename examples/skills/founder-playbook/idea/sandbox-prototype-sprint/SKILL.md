---
name: sandbox-prototype-sprint
status: alpha
description: Land a throwaway single-interaction prototype on a branch and produce a
  5-person reaction-test kit, via a demesne pipeline. A slow-tier orchestrator defines
  the ONE core interaction plus a not-building list, builds only that (scaffolding
  greenfield or inside a mounted starter repo), gates it on "it actually runs" in a
  sandbox_script smoke check, has a fresh child write a neutral reaction-test script
  and observation-capture sheet, reviews for scope creep and leading prompts, commits
  on pipeline/<slug> in /out/repo, and delivers the kit as files. Apply when the
  concept is validated and the founder wants a disposable prop to put in front of real
  users — "build a quick prototype to test the core interaction", "throwaway prototype
  for user reactions", "prototype sprint", "build the demo I'll show five people". Skip
  when the concept itself is unproven (pressure-test it first with
  sandbox-solution-concept-pressure-test); when you are building the real product, not
  a conversation prop (that is sandbox-feature-work); or when you are synthesising what
  the five people said afterwards (that is sandbox-interview-synthesis).
allowed-tools: Read, Glob, Grep, Bash, Write, Edit, mcp__demesne__sandbox_agent, mcp__demesne__sandbox_research, mcp__demesne__sandbox_script
---

Build a disposable single-interaction prototype through a demesne pipeline. A slow-tier
orchestrator locks the ONE core interaction, builds only that, gates it on actually
running, then writes a 5-person reaction-test kit. It commits the prototype on a branch
in `/out/repo` (this skill **lands code**) and delivers the kit as files in `/out`. The
prototype is a pressure-testing prop for conversations, **not evidence** — that reframe
is load-bearing and belongs in the kit itself.

**Watch out (cross-cutting):** Scope creep is the failure mode that ruins this run — a
prototype that grows auth, persistence, settings, and three flows is no longer testable
in one sitting and wasted the build. The not-building list is enforced by a fresh
scope-critic and the reviewer, not trusted to the builder. Second: every child writes to its
own `/out/child/<name>/`, which does not reach the caller — the orchestrator must copy
**every** deliverable (the repo, the reaction kit, SMOKE/REVIEW/SUMMARY/FINDINGS/SCOPE) into
its own `/out` in step 9; `/workspace` is torn down on exit, only `/out` persists. Third: a
prototype that doesn't run burns a scarce validated-profile
conversation — the smoke gate is mandatory, not optional polish.

## Procedure

This pipeline is **sequential by construction** — every step edits the shared
`/workspace/repo`, so phases run as blocking calls one after another; there is no
parallel fan-out to background-dispatch. (Want two rival stacks compared instead? That's
`sandbox-tournament-search`, not this skill.)

1. **Stage.** If a starter repo/scaffold is mounted, copy it whole (incl. `.git`) so
   reviewers can `git diff`: `cp -a /in/<repo>/. /workspace/repo`. If greenfield (common
   at Idea stage — no product exists yet), the orchestrator scaffolds a minimal app in
   the stack named in the prompt and `git init`s `/workspace/repo`.

2. **Scope lock.** In the orchestrator's own process, read the mounted concept +
   discovery evidence and write `/workspace/SCOPE.md`: the single core interaction (one
   happy path, one screen/command) and the explicit **not-building list**. Then spawn a
   fresh medium-tier `sandbox_agent` (`name=scope-critic`) that reads `SCOPE.md` and
   attacks it — is this genuinely ONE interaction, or three smuggled in? Is anything on
   the build side not load-bearing for that interaction? It writes
   `/out/child/scope-critic/CRITIQUE.md`; the orchestrator tightens `SCOPE.md` once. 1
   round — do not loop.

3. **Research (optional).** Only if the interaction needs an unfamiliar library/API,
   spawn one `sandbox_research` child (open egress, fresh private workspace, NO `/in`
   mounts — put every fact it needs in the prompt). It writes `/out/FINDINGS.md`. Skip
   for anything you can build from known tools.

4. **Plan.** Orchestrator writes `/workspace/PLAN.md`: the numbered build phases, each
   scoped strictly to `SCOPE.md`. A prototype is usually 1–3 phases; if you're writing
   more, scope crept.

5. **Implement.** One medium-tier `sandbox_agent` per phase (`name=phase01`, `phase02`).
   Names are lowercase DNS-1123 labels (letters, digits, interior hyphens, ≤40 chars;
   `Phase_1` poisons the spawn). Each edits `/workspace/repo`, stubs everything on the
   not-building list (fake data, no real auth/storage, happy-path only), and writes
   `/out/SUMMARY.md`. Phases share `/workspace` and run sequentially — parallel writes
   corrupt the index.

6. **Smoke gate.** Spawn a `sandbox_script`, `egress=none`, `image=` matching the stack
   (`node`/`python`/`go`, or `browser` for a web UI — headless Chromium loads the page
   and asserts the core interaction renders and responds). Bar is **"the one interaction
   runs end-to-end on the happy path"**, not feature-work's full lint/test/build gate —
   but it MUST run, headlessly and deterministically. Write the result to
   `/out/child/<name>/SMOKE.md`. A red gate goes back to a fix phase (cap 2); if still red
   after 2 rounds, **halt and deliver a red-gate report** — do not ship a non-running
   prototype into a scarce conversation.

7. **Reaction-test kit.** Spawn a fresh medium-tier `sandbox_agent`
   (`name=reaction-kit`) that reads the built prototype and the target profile, and
   writes `/out/reaction-test-kit/` — the 5-person script + per-participant
   observation-capture sheet (schema below). Fresh context so it tests the interaction
   as a stranger would, not as its author.

8. **Review.** Fresh medium-tier `sandbox_agent` (`name=review01`) reads
   `git -C /workspace/repo diff`, `SCOPE.md`, and the kit at
   `/in/previous-jobs/reaction-kit/reaction-test-kit/` (a completed sibling), and judges
   three things: does it run and demonstrate the one interaction; did anything on the
   not-building list get built anyway; are the test prompts genuinely neutral (no
   leading/hypothetical framing). Writes `/out/REVIEW.md` ending in `PASS` or
   `CHANGES_NEEDED`. On `CHANGES_NEEDED`, fix and re-review. Cap 3 rounds. If a fix round
   changes the interaction, regenerate the reaction kit (step 7) so it doesn't go stale.

9. **Finalize.** Commit on `pipeline/<slug>` in `/workspace/repo` — **branched from the
   mounted repo's HEAD** so the host's `merge --ff-only` applies (greenfield: the branch
   is the whole history, nothing to merge into), authored as `Pipeline <pipeline@local>`.
   Then, in the orchestrator's own process, copy **every** deliverable into its own `/out`
   (each child wrote to `/out/child/<name>/`, which does not persist to the caller):
   - `cp -a /workspace/repo /out/repo`
   - `cp /workspace/SCOPE.md /out/SCOPE.md`
   - `cp -r /out/child/reaction-kit/reaction-test-kit /out/reaction-test-kit`
   - `cp /out/child/<smoke>/SMOKE.md /out/SMOKE.md`
   - `cp /out/child/review01/REVIEW.md /out/REVIEW.md`
   - `cp /out/child/phase01/SUMMARY.md /out/SUMMARY.md` (name per phase, or concatenate the phase summaries)
   - `cp /out/child/<research>/FINDINGS.md /out/FINDINGS.md` — only if a research child ran

   Do **not** delegate these copies to a `sandbox_script` child — its `/out` is
   `/out/child/<name>/` and the files would strand again. Write `/out/CHANGES.md` (branch,
   base commit, greenfield-or-mounted, what the interaction is, what was deliberately
   stubbed). Print `DONE`.

## Writing the orchestrator prompt

The orchestrator starts cold. Brief it as a complete document:

1. **The validated problem + solution concept** — from
   `sandbox-solution-concept-pressure-test` if mounted; enough that the orchestrator can
   pick the single interaction without guessing.
2. **How to isolate the ONE core interaction** — the minimum surface area the solution's
   value depends on, expressed as one happy path. Calibration example: for "in-house
   legal teams manage contract redlines across email threads", the core interaction is
   *paste two contract versions → see the redline diff in one view* — NOT accounts, NOT
   saved documents, NOT sharing, NOT export. Everything else is the not-building list.
3. **The not-building list is mandatory and enforced** — auth, persistence, onboarding,
   settings, secondary flows, error handling beyond the happy path, real integrations
   (stub them), visual polish. If it isn't the one interaction, it isn't built.
4. **The stack and greenfield-vs-mounted decision** — name the stack; say whether a
   starter repo is mounted or the orchestrator scaffolds fresh.
5. **"Do NOT build/run/test yourself — smoke-test via a `sandbox_script` child"** — the
   agent image has no toolchain; give the exact `image=` and the smoke command (e.g.
   `browser` + a Playwright snippet driving the interaction, or `node`/`python` running
   a headless script that exercises the one path).
6. **The reaction-test kit spec** — a `sandbox_agent` writes, into
   `/out/reaction-test-kit/`:
   - **`recruit.md`** — 5 people from the validated target profile (reference
     `sandbox-interview-kit-design`'s profile if mounted); why 5, why this profile.
   - **`facilitator-script.md`** — neutral setup ("I want to watch you use something;
     there are no wrong moves; think aloud"), then the realistic scenario that triggers
     the interaction, told WITHOUT explaining how to do it or what it's for. Anti-leading
     rules verbatim: no "would you use this?", no feature-listing, no defending the
     prototype when they struggle — **struggle is data**.
   - **`observation-sheet.md`** — a per-participant template, one locked schema:
     what they tried first · where they hesitated or got stuck · verbatim spontaneous
     quotes · did they complete the interaction unaided (Y/N) · behavioral would-use
     signal (what they *did*, not "do you like it?") · one line "what this tells us vs
     what we cannot conclude".
   - A framing header on every file: **this prototype is a prop for pressure-testing
     conversations, not evidence the hypothesis is right** (playbook Idea-stage trap:
     mistaking building for validating). Completed observation sheets are the input
     corpus for `sandbox-interview-synthesis`.
7. **Pipeline contract** — the nine steps above, the child-naming rule, verifier
   separation (scope-critic, reaction-kit, and reviewer are fresh contexts, never the
   builder scoring itself), and the round caps (scope 1, smoke-fix 2, review 3).
8. **Output contract** — the file tree below; `DONE` last.

## Output contract

```
/out/
  repo/                     # full repo; host fetches pipeline/<slug> from here
  SCOPE.md                  # the one interaction + not-building list (final, tightened)
  FINDINGS.md               # only if a research child ran
  SUMMARY.md                # per build phase
  SMOKE.md                  # smoke-gate result (from the sandbox_script child)
  REVIEW.md                 # verdict: PASS or CHANGES_NEEDED
  reaction-test-kit/
    recruit.md
    facilitator-script.md   # neutral prompts + anti-leading rules
    observation-sheet.md    # locked per-participant schema (feeds interview-synthesis)
  CHANGES.md                # branch, base commit, greenfield/mounted, interaction, stubs
```

Workspace (read by the orchestrator; torn down on exit): `/workspace/repo`,
`/workspace/SCOPE.md`, `/workspace/PLAN.md`.

## Launching the orchestrator

- **Mounts.** `directories:` for a starter repo is **optional** — omit it for a
  greenfield prototype and the orchestrator scaffolds. Mount the **concept + target
  profile** (`directories:`/`files:` — the outputs of
  `sandbox-solution-concept-pressure-test` and `sandbox-interview-kit-design`); if you
  forget them, brief the concept and profile fully in the prompt or the orchestrator
  invents the interaction and the kit tests the wrong thing.
- **Tier:** **slow** orchestrator; **medium** for scope-critic, build phases,
  reaction-kit, and review; deterministic smoke via `sandbox_script`.
- **Child-name examples:** `scope-critic`, `phase01`, `smoke`, `reaction-kit`,
  `review01`.

## Host-side landing

The in-sandbox smoke gate is authoritative for "it runs"; the host does not re-derive it.

1. Read `/out/CHANGES.md` for branch, base, and whether it's greenfield or mounted.
2. **Mounted starter repo:** `git -C <repo> fetch <output_dir>/repo <branch>` then
   `git -C <repo> merge --ff-only FETCH_HEAD`; re-author with
   `git -C <repo> commit --amend --reset-author --no-edit`. **Greenfield:** `/out/repo`
   *is* the prototype — clone/copy it out; there is nothing to merge into.
   **Fallback:** if `/out/repo` is absent (copy delegated to a child), fetch from
   `<output_dir>/child/<name>/repo`.
3. Run the prototype once locally to confirm the interaction works on the host, then hand
   `/out/reaction-test-kit/` to the founder to run in the real world — recruit the five,
   facilitate neutrally, capture observations. Those completed sheets go into
   `sandbox-interview-synthesis`. This is a real-world handoff, not a connected-tool
   action; the sandbox cannot recruit or schedule anyone.
