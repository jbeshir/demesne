---
name: sandbox-mvp-architecture-charter
description: Turns a validated MVP concept (and, if it exists, the prototype repo) into the build's first persistent artifact — a slow-tier orchestrator inventories the real stack, an architect child drafts every architectural decision with its tradeoff and the WHY behind it, four adversary children attack the draft along the MVP stage's failure lenses (compounding agentic tech debt, zero-friction scope creep, insecure-by-inexperience, one-way-door irreversibility), a fresh compiler resolves each attack into a final CLAUDE.md-style charter, and a scaffold child emits the session template and per-session log format the founder drops into the repo. Apply when a founder is about to start an MVP build and wants the architecture decided and written down before the first production code — "define our MVP architecture", "write the CLAUDE.md before we build", "what patterns and deps should we commit to", "pressure-test our architecture decisions", "set up context docs so the codebase doesn't drift". Skip when you want the code built rather than the charter (sandbox-feature-work); the does / deliberately-doesn't-do scope boundary (sandbox-mvp-scope-guardrail); a code-level security audit rather than design-time flags (sandbox-prelaunch-security-review); backfilling decisions into CLAUDE.md after a codebase already exists or auditing accrued debt (sandbox-tech-debt-audit); or attacking the solution concept itself before any architecture exists (sandbox-solution-concept-pressure-test).
allowed-tools: Read, Glob, Grep, Bash, Write, Edit, mcp__demesne__sandbox_agent, mcp__demesne__sandbox_script
---

Produce the MVP build's first artifact: the architecture charter every later session reads. You author one orchestrator prompt and launch a single slow-tier `sandbox_agent`. It inventories the mounted prototype (if any), has an architect child draft every architectural decision with its tradeoff and the reasoning behind it, fans out four adversary children that attack the draft along the MVP stage's four failure lenses, runs a fresh compiler that resolves each attack into a final `CLAUDE.md`-style charter, and a scaffold child that emits the session template and per-session log format. The deliverables are docs in `/out` — there is **no code branch**; the host drops the files into the repo (see Host-side landing). The whole point per the playbook: architectural decisions that aren't written down *with their why* silently drift across dozens of AI-generated sessions, and the engineering-cost forcing function that used to catch sprawl is gone.

**Watch out (cross-cutting):** The adversaries must be dispatched only after the architect child reaches a terminal state (`sandbox_wait` on its `job_id` first) — the `/in/previous-jobs/architect/` mount registers at child-create but the draft file appears only once the architect finishes writing; adversaries started early read an empty mount and attack nothing. A charter that records *what* was chosen without *why* is the exact failure this skill exists to prevent — the compiler must reject any decision that survives with a bare "what". And the orchestrator must `cp` every deliverable into its own `/out/` itself; delegating that to a `sandbox_script` child strands the files under `/out/child/<name>/`.

## Procedure

1. **GROUND** (orchestrator's own process). Concatenate the mounted concept/discovery inputs into `/workspace/concept.md` — real founder inputs are messy (a doc, chat exports, discovery notes); log anything unparseable rather than dropping it silently. If a prototype repo is mounted, copy it whole (including `.git`) with `cp -a /in/<repo>/. /workspace/repo` so decisions are grounded in what actually exists. If none, write `/workspace/GREENFIELD` and note in the charter that decisions are forward-looking.

2. **STACK INVENTORY** (`sandbox_script`, `name=stack-inventory`, `image=<repo lang: go|node|python|anaconda>`, `egress=none`). Deterministic dump only — never an LLM for this: manifest/lockfile contents, resolved dependency lists, `git -C /workspace/repo log --oneline -30`, and a two-level directory tree, written to `/workspace/stack.md`. Skip this step for greenfield. The architect reads this so the charter names the *actual* stack, not a guessed one.

3. **ENUMERATE** (orchestrator's own process). From `concept.md` + `stack.md`, write `/workspace/decisions.json`: an array of decision points to settle, one object each (`id`, `question`, `options`, `axis`). Cover the playbook's three axes — **patterns to follow** (code organization, state/data model, API shape, error handling), **dependencies** (which frameworks/services to adopt *and which to deliberately not add*), **tradeoffs** (each choice's cost and reasoning). Under-enumerating here is the common failure: an axis with no decision point produces a charter silent on it, and the first session invents its own convention.

4. **DRAFT** (one slow-tier `sandbox_agent`, `name=architect`). Reads `concept.md`, `stack.md`, `/workspace/repo`, and `decisions.json`; drafts every decision as: choice, what it trades away, **the WHY a future session must not relitigate**, reversibility (one-way vs two-way door), and a revisit-trigger. Writes `/out/child/architect/draft-charter.md`. This is a draft, not the deliverable — it goes to attack before anything is written down.

5. **PRESSURE-TEST** (fan-out, medium tier — **barrier: dispatch only after the architect job is terminal**). Dispatch four adversaries with `background: true`, collect `job_id`s, poll `sandbox_wait` (`timeout_seconds: 120`) until all reach a terminal state; ≤8 in flight so all four dispatch at once. Blocking calls are issued one per turn and run sequentially, serialising the stage — always background. Each reads the draft at `/in/previous-jobs/architect/draft-charter.md` and writes `/out/child/adversary-<lens>/attacks.md` citing specific decision `id`s. The four lenses (genuinely distinct priors — an echo chamber gives false confidence):

   | `name` | Lens (playbook challenge) |
   |--------|---------------------------|
   | `adversary-debt` | *Agentic technical debt* — which decisions are recorded as *what* without a durable *why*, or will compound into unmaintainable sprawl once many AI sessions build on them? |
   | `adversary-scope` | *Zero-friction scope creep* — where is this over-built beyond the smallest iteration that generates real PMF evidence (premature abstraction, microservices, speculative generality)? |
   | `adversary-security` | *Insecure by inexperience* — which decisions bake in attack surface (auth/session shape, secret handling, data exposure in responses) a first-time founder won't see? Design-time flags only; the code audit is sandbox-prelaunch-security-review. |
   | `adversary-reversibility` | Which decisions are one-way doors mislabelled two-way — costly to undo if wrong — and should be deferred or swapped for a cheaper reversible choice? |

6. **SYNTHESIZE** (one fresh slow-tier `sandbox_agent`, `name=charter-compiler`, after all adversaries are terminal). Reads the draft plus all four `/in/previous-jobs/adversary-*/attacks.md`. Resolves each attack explicitly: accept → revise the decision, or reject → record the documented reason. It must be a fresh context — never the architect scoring its own draft. Writes the final `/out/child/charter-compiler/CLAUDE.md` and `/out/child/charter-compiler/change-log.md` (per attack: accepted/rejected + why). **Cap the loop at 2 rounds:** if this pass materially revises ≥3 decisions or introduces new decision points, run one more pressure-test round (`adversary-<lens>-r2`, distinct names) against the revised charter, then re-synthesize once; do not loop further.

7. **SCAFFOLD** (one medium-tier `sandbox_agent`, `name=scaffold`, after the compiler is terminal). Reads the final `CLAUDE.md`; emits the two drop-in files the playbook calls for — `/out/child/scaffold/SESSION_TEMPLATE.md` (context pointer + specific task + constraints + definition-of-done) and `/out/child/scaffold/SESSION_LOG_FORMAT.md` (the per-session entry format — *what was built, what decisions were made, what assumptions were introduced, follow-ups* — with one worked example entry). Both must reference the charter by name so sessions actually load it.

8. **DELIVER** (orchestrator's own process — not a child). `cp` the final `CLAUDE.md`, `SESSION_TEMPLATE.md`, `SESSION_LOG_FORMAT.md`, `change-log.md`, the architect draft, and each adversary's attacks into the orchestrator's own `/out/` per the tree below. Write `/out/metadata.json` (concept title, repo-or-greenfield, decision count, rounds run, run date). Print `DONE`.

## Writing the orchestrator prompt

The orchestrator starts cold with only your prompt and the mounts. Brief it as a complete document:

1. **The concept + the user** — the validated problem, the single core value, and the *specific identifiable user group* the MVP must retain/monetise/spread (the playbook's PMF exit criteria). Without this the architect anchors decisions to nothing.
2. **Ground rules for the charter** — every decision records **the WHY, not just the what**; name deps deliberately *not* adopted and why; mark each decision one-way vs two-way door with a revisit-trigger. This is the load-bearing content — over-specify it.
3. **The four adversary lenses verbatim** (table above) as the pressure-test, with the instruction that priors must stay genuinely distinct.
4. **The pipeline contract** — the eight steps, the child-naming rule, background-dispatch + `sandbox_wait` for the fan-out (blocking children run sequentially, one per turn), the barrier before pressure-test, the fresh-context compiler, and the 2-round cap.
5. **Charter section order** (below) and the two scaffold files' required contents.
6. **Repo grounding** — if a repo is mounted, decisions must cite the actual stack from `stack.md`; forbid the architect from proposing a rewrite the founder didn't ask for. If greenfield, say so.
7. **Non-goals boundary** — the charter names non-goals but the enforceable scope doc is sandbox-mvp-scope-guardrail's job; cross-reference, don't duplicate.

Terse prompts produce a charter that reads like generic best-practice. Over-specify the concept and the why-discipline; under-specify which options should win.

## Output contract

```
/out/
  CLAUDE.md                          # the architecture charter — drop at repo root
  SESSION_TEMPLATE.md                # context pointer + task + constraints scaffold
  SESSION_LOG_FORMAT.md              # per-session log-entry format + one worked example
  change-log.md                      # per attack: accepted/rejected + why
  metadata.json                      # concept title, repo|greenfield, decisions, rounds, date
  charter/
    draft-charter.md                 # architect's pre-attack draft
    attacks/
      adversary-debt.md
      adversary-scope.md
      adversary-security.md
      adversary-reversibility.md
```

`CLAUDE.md` sections in order: **Product context** (one paragraph: what the MVP is, the single core value, the specific user group); **Architecture at a glance** (the stack as decided, grounded in `stack.md` if a repo exists); **Patterns to follow** (conventions every session obeys); **Dependencies** (adopted — with why; and *deliberately avoided* — with why not); **Decisions & tradeoffs** (a table: decision · choice · traded-away · WHY · one-way/two-way door · revisit-trigger); **Non-goals** (what the MVP will not do — pointer to the scope doc); **Open assumptions** (introduced decisions still to validate against real users).

## Launching the orchestrator

- **`files:` for the validated concept is mandatory.** Without it the orchestrator wakes with nothing to ground decisions in and produces a generic charter. Mount the discovery notes/concept doc; messy formats are expected and the GROUND step logs unparseable pieces.
- **`directories: ["<abs path to prototype repo>"]` when one exists** — strongly recommended; a mounted repo makes the charter grounded rather than aspirational. Omit it only for a true greenfield build (the pipeline handles that path).
- **Tier:** slow for the orchestrator, the architect, and the compiler (decision quality); medium for the adversaries and scaffold; `sandbox_script` for the stack inventory.
- **Child names** are DNS-1123 labels — lowercase letters, digits, interior hyphens, ≤40 chars: `stack-inventory`, `architect`, `adversary-security`, `adversary-debt-r2`, `charter-compiler`, `scaffold`. Never `Adversary_Debt` or `charter.compiler` — bad names break the sibling mounts silently.

## Host-side landing

No validated code branch is produced — the deliverables are the charter and its two scaffold files. Read `/out/CLAUDE.md` and `change-log.md` first (the rejected attacks are often where the sharpest reasoning sits). To land: drop `CLAUDE.md` at the repo root, `SESSION_TEMPLATE.md` and `SESSION_LOG_FORMAT.md` under `docs/` (or wherever the team keeps context), `git add` and commit — a documentation drop-in, not a build. From here, every build session opens with the session template and closes with a log entry; feed the resulting non-goals into sandbox-mvp-scope-guardrail and hand the charter to sandbox-feature-work as settled design.
</content>
</invoke>
