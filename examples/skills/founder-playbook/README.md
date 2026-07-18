# The Founder's Playbook, as runnable skills

This document chains the founder-playbook skills that implement the activities in
Anthropic's *The Founder's Playbook: Building an AI-Native Startup* across its four
stages. The skills live in this folder, one subfolder per stage — [`idea/`](idea/),
[`mvp/`](mvp/), [`launch/`](launch/), [`scale/`](scale/) — plus [`meta/`](meta/) for
the skill-forge that produced the family. Each stage below lists the skills in
invocation order, what flows from one to the next, and the playbook's own exit
criteria for moving on. Coverage of every numbered playbook activity is tabulated in
[`COVERAGE.md`](COVERAGE.md); the wider skills library (and its other categories)
is indexed in [`../README.md`](../README.md).

Two ground rules carried through every skill:

- **Sandboxes can't touch your mail, calendar, or CRM.** Skills whose real-world
  deliverable needs those (outreach sends, session scheduling, recurring
  ceremonies) are hybrids: the pipeline produces reviewed, structured drafts and a
  **Host-side finishing** section tells your host agent what to do with them using
  whatever tools it actually has connected.
- **Founder-private data arrives as mounted directories.** Interview notes,
  retention exports, support inboxes, customer evidence — the skills take them as
  `directories:` mounts and are built to handle messy real-world files honestly
  (logging what they can't parse rather than silently skipping it).

## Idea stage — validate before you build

> Exit criteria (problem-solution fit): the problem is real and specific (who, how
> often, how severely, what they currently do about it); your solution addresses
> the problem validation revealed, not the one you assumed; and you have enough
> qualitative signal that committing to an MVP is reasoned, not an act of faith.

1. **[`sandbox-problem-discovery`](idea/sandbox-problem-discovery/)** — investigate an audience, painful workflow, concept, competitors, demand, willingness to pay, channels, differentiation, and risks; compare prior reports and produce one concise human-readable decision report. Research effort scales with the question, and the skill stops with recommended next research rather than a generated audit package.
2. **[`sandbox-hypothesis-stress-test`](idea/sandbox-hypothesis-stress-test/)** — take one selected brief, sharpen the observation into a
   testable hypothesis and collect the strongest case *against* it before talking
   to anyone. Output: `hypothesis.md` + counter-case + discovery test list.
3. **[`sandbox-market-landscape`](idea/sandbox-market-landscape/)** — tiered competitor map with per-tier threat
   cases, pressure-tested TAM/SAM/SOM, buyer landscape, and three 2-year trends
   judged tailwind/headwind for this hypothesis.
4. **[`sandbox-competitor-complaint-mining`](idea/sandbox-competitor-complaint-mining/)** — what competitors' own customers say
   is still broken, scored against your hypothesis. If your hypothesis addresses a
   top unresolved complaint, that's problem-solution-fit signal.
5. **[`sandbox-interview-kit-design`](idea/sandbox-interview-kit-design/)** — target profile, reachability map, and
   per-persona question sets audited against leading/socially-desirable framing.
   Takes the stress-tested hypothesis as input.
6. **[`sandbox-outreach-pipeline`](idea/sandbox-outreach-pipeline/)** *(hybrid)* — prospect list with verified-vs-
   guessed contact flags, per-prospect personalized drafts, day-7 follow-up
   cadence, tracking sheet. You review every draft; the host session sends.
7. **[`sandbox-interview-synthesis`](idea/sandbox-interview-synthesis/)** — after interviews accumulate: per-interview
   debriefs (confirmed / challenged / surprised) and the every-5-interviews
   supporting-vs-challenging audit with a confirmation-bias flag.
8. **[`sandbox-solution-concept-pressure-test`](idea/sandbox-solution-concept-pressure-test/)** — once validation holds, develop
   the concept and attack it from four angles; isolate the three load-bearing
   assumptions, each with a cheap test and a failure blast-radius.
9. **[`sandbox-prototype-sprint`](idea/sandbox-prototype-sprint/)** — build only the single core interaction, plus a
   five-person reaction-test kit. The prototype is a conversation prop, not
   evidence; the completed reaction sheets feed back into
   `sandbox-interview-synthesis`.

## MVP stage — the smallest product that generates real PMF evidence

> Exit criteria: genuine evidence of product-market fit — a specific, identifiable
> group of users finds the product valuable enough to return to it (retention),
> pay for it (revenue), or tell others about it (referral).

1. **[`sandbox-mvp-architecture-charter`](mvp/sandbox-mvp-architecture-charter/)** — before production code: the
   architecture decisions with their WHY, as a CLAUDE.md-ready charter plus a
   session template and per-session log format. The first artifact of the build.
2. **[`sandbox-mvp-scope-guardrail`](mvp/sandbox-mvp-scope-guardrail/)** (CHARTER mode) — what the MVP does, what it
   deliberately does not do, and the evidence bar a new feature must clear.
   Re-run in AMENDMENT mode every time a feature idea surfaces.
3. *Build with your coding agent* — [`sandbox-feature-work`](../build-and-land/sandbox-feature-work/) (in this repo's
   `build-and-land/` category) drives substantial changes, governed by the charter
   and scope doc.
4. **[`sandbox-metrics-framework`](mvp/sandbox-metrics-framework/)** — metrics that matter, activation and D7/D30
   benchmarks, and this product's false-positive signatures — defined before the
   first user shows up.
5. **[`sandbox-prelaunch-security-review`](mvp/sandbox-prelaunch-security-review/)** — auth/session, data exposure, input
   validation, dependency vulnerabilities; anything touching auth, secrets, or
   data handling is flagged for human review. Run before real users arrive.
6. **[`sandbox-feedback-loop-ops`](mvp/sandbox-feedback-loop-ops/)** *(hybrid, recurring)* — weekly: intake and
   triage the raw feedback inbox, score feature requests against the scope doc,
   produce a factual raw synthesis that a human reads before any pattern analysis.
7. **[`sandbox-pmf-diagnostic`](mvp/sandbox-pmf-diagnostic/)** — Sean Ellis test (>40% "very disappointed" among
   active users) plus the push-vs-pull founder-effort test, with an adversarial
   bull/bear verdict. A pattern across cycles, never a single data point.
8. **[`sandbox-pivot-diagnostic`](mvp/sandbox-pivot-diagnostic/)** — only after 3+ iteration cycles without
   movement: segment divergence, positioning-vs-product gap, and
   what-would-have-to-be-true, ending in ADJUST / PIVOT-<type> /
   RETURN-TO-IDEA-STAGE with dissent preserved.

## Launch stage — repeatable growth, hardened product, no founder bottlenecks

> Exit criteria: (1) growth is repeatable and channel-driven with understood unit
> economics (CAC, LTV, payback); (2) the product handles production workloads —
> infrastructure hardened, security and compliance in order; (3) operations run
> without founder bottlenecks — the founder no longer personally handles support,
> triage, sprint planning, or reporting.

1. **[`sandbox-tech-debt-audit`](launch/sandbox-tech-debt-audit/)** — the MVP codebase's debt, sequenced across
   sprints (must-fix / can-wait / acceptable), plus a backfill of the architecture
   decisions that lived only in the founder's head. Remediation executes via
   `sandbox-quality-improvement` / `sandbox-feature-work` (both in `build-and-land/`).
2. **[`sandbox-compliance-review`](launch/sandbox-compliance-review/)** — code-level findings mapped to the target
   market's frameworks (SOC 2 / GDPR / HIPAA), a remediation sequence, and the
   controls-and-documentation workstream an enterprise buyer's review will demand.
3. **[`sandbox-founder-load-audit`](launch/sandbox-founder-load-audit/)** — every recurring task, decision, and
   only-happens-if-the-founder-remembers workflow, triaged into automate /
   delegate / genuinely-founder, with workflow-logic designs for the automation
   candidates.
4. **[`sandbox-product-ops-system`](launch/sandbox-product-ops-system/)** *(hybrid)* — sprint cadence, minimum spec
   template, bug-triage decision tree (wired to `sandbox-feedback-loop-ops`), and
   a weekly metrics brief naming real data sources; the host wires the recurring
   layer.

## Scale stage — systematic growth, enterprise readiness, a defensible moat

> Exit criteria (threshold event): the company is sustainable without the founder
> running day-to-day operations; growth is systematic and auditable; governance,
> compliance posture, and strategic narrative satisfy the most demanding external
> reviewers; and there's a solid answer to "if a well-funded incumbent copied your
> product today, would your users stay?" — reached as sustainable profitability,
> IPO-readiness, or acquisition.

1. **[`sandbox-bottleneck-stress-test`](scale/sandbox-bottleneck-stress-test/)** — the founder-bottleneck map plus a
   one-week-absence simulation per workflow; cross-references the Launch-stage
   load audit; fix designs (handoff criteria, escalation, exception handling) per
   failure point.
2. **[`sandbox-enterprise-procurement-pack`](scale/sandbox-enterprise-procurement-pack/)** *(hybrid)* — the written pack
   procurement expects (docs, support playbooks, SLA drafts traced to observed
   capability, questionnaire answer library) plus a design-only
   observability/drift-detection plan handed to `sandbox-feature-work`.
3. **[`sandbox-enterprise-gap-analysis`](scale/sandbox-enterprise-gap-analysis/)** — for three named target accounts (or
   ICPs): what that buyer's procurement demands vs what exists, sequenced by
   accounts-unblocked per effort with lead-time overrides for hard gates.
4. **[`sandbox-gtm-engine`](scale/sandbox-gtm-engine/)** — segmentation, messaging architecture, and playbooks
   per audience (users, investors, enterprise buyers, analysts), a coherence pass
   forcing one core story, and the tactical execution layer designs.
5. **[`sandbox-domain-knowledge-codify`](scale/sandbox-domain-knowledge-codify/)** — the proprietary knowledge substrate:
   a searchable context library plus codified workflow routines, each
   replay-verified to run the same way every time.
6. **[`sandbox-moat-test-suite`](scale/sandbox-moat-test-suite/)** — vertical edge cases a generic competitor gets
   wrong, landed as an accreting scenario-test suite whose coverage map *is* the
   moat map.
7. **[`sandbox-data-flywheel-audit`](scale/sandbox-data-flywheel-audit/)** — what interaction data exists (honest
   thin-data verdicts included), the usage→improvement loop design, and a
   one-page moat narrative that survives an adversarial competitor's red-team.
8. **[`sandbox-lockin-audit`](scale/sandbox-lockin-audit/)** — per-customer integration and switching-cost
   profiles across the top ten customers, lock-in patterns, and the native
   integration/API/webhook/SDK backlog that deepens them.

## Regenerating or extending this library

**[`sandbox-playbook-skill-forge`](meta/sandbox-playbook-skill-forge/)** is the pipeline that produced this family:
mount any prescriptive playbook and the skills repo, and it decomposes activities
into a roster, fans out designers against a shared conventions brief, gates every
draft through fresh-context critics with one capped fix round, and lands the
result as a branch with a coverage table.
