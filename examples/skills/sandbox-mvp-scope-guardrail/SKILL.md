---
name: sandbox-mvp-scope-guardrail
description: The written forcing-function that replaces engineering cost as the thing that says "no" to MVP scope creep, in two modes. CHARTER mode — an orchestrator drafts an MVP scope doc (what the product DOES, what it deliberately DOES-NOT do, and the feature-amendment criteria = the specific user evidence that would justify each class of later addition), then fans out 3 adversarial debaters with distinct priors to attack the draft, and a judge compiles the hardened `/out/scope.md`. AMENDMENT mode — given an existing scope doc plus one proposed feature and the founder's evidence, it runs a prosecutor-vs-defender debate on the playbook's single test — genuine user signal, or founder enthusiasm dressed up as product thinking? — checks the proposal against the criteria the founder pre-committed BEFORE they were invested, and a judge issues `/out/amendment-verdict.md` (ADD-NOW / DEFER-WITH-TRIGGER / REJECT). Deliverable is docs in `/out`; no code landing. Apply when the request is "write the MVP scope doc", "what should the MVP NOT do", "define what we're deliberately not building", "should we add this feature", "is this feature scope creep", "pressure-test this feature idea", "gut-check this against our scope". Skip when the decision is architecture/patterns/dependencies rather than product surface (use sandbox-mvp-architecture-charter), when you are still developing or attacking the solution concept itself (use sandbox-solution-concept-pressure-test), or when you need to triage a whole recurring inbox of feature requests rather than adjudicate one (use sandbox-feedback-loop-ops, which scores its requests against the scope.md THIS skill produces).

allowed-tools: Read, Glob, Grep, Bash, Write, Edit, mcp__demesne__sandbox_agent, mcp__demesne__sandbox_research, mcp__demesne__sandbox_script
---

At the MVP stage engineering cost — the traditional forcing function that used to make features expensive enough to refuse — is gone; every addition is individually defensible, so the product sprawls past its boundary and loses momentum unless a written guardrail becomes the new "no". This skill builds and enforces that guardrail. You author one orchestrator prompt, launch a single slow-tier `sandbox_agent`, and it runs whichever of two modes the host names. **CHARTER**: draft the scope doc, fan out 3 adversarial debaters to attack it, judge compiles `/out/scope.md`. **AMENDMENT**: adjudicate one proposed feature against an existing scope doc via a 2-role debate, judge issues `/out/amendment-verdict.md`. Both deliver documents to `/out`; there is no code landing.

**Watch out (cross-cutting):** the two failure modes that silently ruin a run pull in opposite directions — a CHARTER run that lets a "wouldn't-it-be-cool" feature stay on the DOES list ships a bloated MVP, and one that cuts so hard the core loop can't produce a retention/revenue/referral signal ships a demo that proves nothing; the debate roles exist to catch both, so do not collapse them. In AMENDMENT mode the whole point is checking the proposal against the criteria the founder committed to BEFORE they fell for the feature — so `scope.md` is a **mandatory mount**; without it AMENDMENT degenerates into unanchored opinion and cannot run. The judge is dispatched only after every debater job reaches a terminal state, and the orchestrator `cp`s deliverables into its own `/out/` itself.

## Procedure

Step 1 branches on the mode the host states in the orchestrator prompt.

1. **FRAME.** The orchestrator reads its mounts and writes the debate inputs to `/workspace/`.
   - *CHARTER:* read the validated-concept / discovery evidence (mounted from `sandbox-solution-concept-pressure-test` or `sandbox-hypothesis-stress-test` output if present). Write `/workspace/thesis.md`: the one validated problem, the specific identifiable user group, and which PMF signal (retention / revenue / referral) the MVP must generate. Then spawn a `charter-drafter` medium-tier `sandbox_agent` that writes `/workspace/scope-draft.md` (DOES / DOES-NOT / amendment-criteria per the rubric in "Writing the orchestrator prompt").
   - *AMENDMENT:* read the mandatory `scope.md` mount and the proposed feature (in the prompt). Write `/workspace/proposal.md` (the crisp one-line feature) and `/workspace/evidence-index.md`. If the founder's evidence includes structured exports (a feature-request CSV, a usage table), dispatch an `evidence-inventory` `sandbox_script` (`image=python`, `egress=none`) that tallies **distinct requesting users, blocked/churn mentions, and willingness-to-pay signals** into `/workspace/evidence-index.md` — deterministic counting is not an LLM's job. Log any unparseable file rather than dropping it silently; messy real-world formats are expected.

2. **RESEARCH (CHARTER, optional).** Only to settle whether a proposed DOES-NOT exclusion is table-stakes that would block a user from even trying the core loop (e.g. you cannot ship the loop without auth). Spawn a `sandbox_research` child (open egress; fresh private `/workspace`, **no `/in` mounts** — pass the exclusion list in its prompt) and fold its findings into `/workspace/thesis.md` before the fan-out. `sandbox_agent` children have no web access; skip this step by default — benchmarking against competitors' feature lists is itself a scope-creep vector.

3. **DEBATE (fan-out).** Dispatch every debater with `background: true`, collect `job_id`s, poll `sandbox_wait` (`timeout_seconds: 120`) until all are terminal; ≤8 in flight (both modes fit). Blocking calls are issued one per turn and run sequentially — that serialises the debate and lets later debaters read earlier positions, destroying independence, so background dispatch is mandatory. Inject each role's prior verbatim via the `preamble` parameter (an empty preamble yields a generic agent). Medium-tier. Each debater reads its inputs from `/workspace/` and writes `/out/position.md`.
   - *CHARTER:* three roles with genuinely distinct priors — `debater-minimalist` (every feature is guilty until proven load-bearing for the PMF signal; cut aggressively), `debater-evidence-advocate` (is the surviving core loop still complete enough for a user to PRODUCE a retention/revenue/referral signal? guard against over-cutting), `debater-creep-detector` (hunt the DOES list for founder enthusiasm already smuggled in, and the amendment-criteria for vague or un-measurable bars). Each scores every DOES / DOES-NOT line and every amendment criterion.
   - *AMENDMENT:* two roles — `prosecutor-creep` (argue this is founder enthusiasm dressed as product thinking: single loud user, competitor-has-it, founder aesthetic, hypothetical future user) and `defender-signal` (argue genuine signal: unprompted requests from multiple distinct users, users blocked or churning without it, willingness to pay). Each cites `/workspace/evidence-index.md` — claims unsupported by the mounted evidence must be flagged as such.

4. **JUDGE.** After all debater jobs are terminal, spawn one slow-tier `sandbox_agent` that reads `/in/previous-jobs/debater-*/position.md` (siblings are complete, so their files are present) and the framing docs it needs.
   - *CHARTER:* `scope-judge` compiles the hardened `/out/scope.md`, keeping any DOES→DOES-NOT demotion a debater justified and rewriting every amendment criterion the creep-detector marked un-measurable into a concrete, countable bar.
   - *AMENDMENT:* `amendment-judge` weighs both positions against the pre-committed amendment criterion in `scope.md` for the proposal's class and issues the verdict; a deferral MUST name the exact future evidence that would flip it to ADD. Dissent is preserved, not homogenised.

5. **DELIVER.** In the orchestrator's own process — not a `sandbox_script` child, which writes only to `/out/child/<name>/` and would strand the files — `cp` the judge's deliverable and every debater's `position.md` into the orchestrator's own `/out/`, and write `/out/metadata.json` (mode, roles, tiers, run date).

## Writing the orchestrator prompt

Brief the orchestrator as a complete document; terse prompts produce shallow guardrails that wave features through.

1. **Mode** — CHARTER or AMENDMENT, stated first. In AMENDMENT, state plainly that `scope.md` is mounted and the run aborts with a clear message if it is absent.
2. **CHARTER inputs** — the validated problem, the specific identifiable user group, and the target PMF signal. The **DOES rubric**: a feature earns its place ONLY if cutting it stops a user completing the core loop or emitting a retention/revenue/referral signal — apply that test line by line, and demote every survivor to DOES-NOT-for-now. The **DOES-NOT rubric**: enumerate the *tempting* adjacent features explicitly (not just obvious omissions) and state why each is excluded now — an unnamed exclusion is one nobody defended. The **amendment-criteria rubric**: a table over addition classes (e.g. new integration, new user segment, workflow depth, polish/UX, admin/reporting) × the specific, countable user-evidence bar that would justify it (e.g. "≥N distinct users hit this exact wall in usage or discovery", "a paying user blocked from renewing") × how that evidence is measured. Vague bars ("users want it") are the failure mode — they authorise everything.
3. **AMENDMENT inputs** — the one proposed feature stated crisply; the founder's evidence directory (feature requests, usage/analytics exports, interview notes, support tickets — children must handle messy files and log unparseable ones). The **test to apply verbatim**: is this genuine signal from users, or founder enthusiasm dressed up as product thinking? Signal markers vs enthusiasm markers as listed in the debater roles. The decisive check: does the evidence clear the bar the founder pre-committed in `scope.md` for this feature's class — the bar they set before they were emotionally invested?
4. **The roles** — the fixed sets above; state that priors must stay genuinely distinct (identical priors produce an echo chamber and a false-confidence verdict).
5. **The pipeline contract** — the five steps; emphasise background dispatch + `sandbox_wait` for the fan-out (blocking children run sequentially, one per turn), the barrier before the judge, and that the orchestrator copies deliverables into its own `/out/`.
6. **Output contract** — the tree below and the section orders below.

## Output contract

```
# CHARTER mode                         # AMENDMENT mode
/out/                                  /out/
  scope.md            # deliverable      amendment-verdict.md   # deliverable
  metadata.json                          metadata.json
  pressure-test/                         debate/
    debater-minimalist/                    prosecutor-creep/
      position.md                            position.md
      transcript.jsonl                       transcript.jsonl
    debater-evidence-advocate/             defender-signal/
      position.md                            position.md
      transcript.jsonl                       transcript.jsonl
    debater-creep-detector/
      position.md
      transcript.jsonl
```

`scope.md` sections in order: **MVP Thesis** (the one validated problem, the specific identifiable user group, the target PMF signal), **In Scope (DOES)** (each feature + why it is load-bearing for that signal), **Deliberately Out of Scope (DOES-NOT)** (each tempting feature + why excluded *now*), **Amendment Criteria** (the addition-class × countable-evidence-bar × measurement table), **The Forcing Function** (one paragraph: this document is what now says "no" in engineering cost's place).

`amendment-verdict.md` sections in order: **Verdict** (ADD-NOW / DEFER-WITH-TRIGGER / REJECT — one line), **The Proposal**, **Evidence Assessment** (what the mounted evidence actually shows: distinct-user count, blocked/churn signal, willingness-to-pay — not what either side claimed), **Signal vs Enthusiasm** (scored against the markers), **Against the Pre-Committed Bar** (does it clear the `scope.md` criterion for its class?), **If Deferred: The Trigger** (the exact future evidence that flips it to ADD), **Dissent** (the losing role's strongest surviving point).

## Launching the orchestrator

- **`files:`/`directories:`** — CHARTER: mount the validated-concept / discovery evidence (optional but recommended; without it the drafter anchors only on the prompt). AMENDMENT: mounting `scope.md` is **mandatory** — forget it and the run has no bar to check against and must abort; also mount the founder's evidence directory (children log unparseable files rather than skipping them).
- **Tier**: slow-tier for the orchestrator and both judges; medium-tier for the drafter and all debaters; `sandbox_script` (no LLM) for the evidence tally.
- **Child-naming rule**: lowercase letters, digits, interior hyphens only, ≤40 chars, unique within the parent — `debater-minimalist`, `debater-creep-detector`, `scope-judge`, `prosecutor-creep`, `amendment-judge` — never `Debater_Minimalist` or `scope.judge`.
