# Founder's Playbook activity coverage

One row per numbered activity in *The Founder's Playbook: Building an AI-Native
Startup* (29 activities across Idea / MVP / Launch / Scale), mapping each to the
skill that implements it. The skills live beside this file, one subfolder per stage
(`idea/`, `mvp/`, `launch/`, `scale/`, plus `meta/`); the "Covered by" column names
each by its directory name. Merges and boundary decisions are noted inline. See
[`README.md`](README.md) for stage-by-stage chaining.

| # | Stage | Activity (playbook one-liner) | Covered by |
|---|-------|-------------------------------|------------|
| 1 | Idea | Sharpen the problem statement into a testable hypothesis; argue against it | `sandbox-hypothesis-stress-test` |
| 2 | Idea | Map the competitive landscape by tier and pressure-test each tier's threat | `sandbox-market-landscape` — merged with 4 and 5: one research fan-out and one compiled report serve all three; competitor tiers, sizing, and trends draw on the same comparables and hypothesis context |
| 3 | Idea | Synthesize competitor customer feedback for unresolved complaints | `sandbox-competitor-complaint-mining` — kept separate from 2/4/5: distinct deliverable (ranked unresolved complaints scored against the hypothesis) and distinct mechanics (review-corpus mining with citation audit) |
| 4 | Idea | Build and pressure-test TAM/SAM/SOM market-sizing models | `sandbox-market-landscape` (merged; see 2) |
| 5 | Idea | Run trend analysis for tailwinds/headwinds | `sandbox-market-landscape` (merged; see 2) |
| 6 | Idea | Design and audit a customer-discovery interview framework | `sandbox-interview-kit-design` |
| 7 | Idea | Run customer outreach and interview scheduling as an operational pipeline | `sandbox-outreach-pipeline` — hybrid: sandboxed prospect research + drafts + cadence + tracking sheet; email send / calendar booking / reply management are host-side finishing (sandboxes cannot reach Gmail/Calendar) |
| 8 | Idea | Run post-interview synthesis with a confirmation-bias check | `sandbox-interview-synthesis` |
| 9 | Idea | Design and pressure-test the final solution concept | `sandbox-solution-concept-pressure-test` |
| 10 | Idea | Build a single-interaction lightweight prototype and get real reactions | `sandbox-prototype-sprint` — the build lands on a branch; the five-person reaction test itself is a human activity, supported by the skill's reaction-test kit whose completed sheets feed `sandbox-interview-synthesis` |
| 11 | MVP | Define the MVP architecture before building and persist it as context | `sandbox-mvp-architecture-charter` |
| 12 | MVP | Define and enforce a written MVP scope document | `sandbox-mvp-scope-guardrail` (CHARTER + AMENDMENT modes) |
| 13 | MVP | Run a pre-launch security review | `sandbox-prelaunch-security-review` |
| 14 | MVP | Build the measurement framework before launch | `sandbox-metrics-framework` |
| 15 | MVP | Run the ongoing discovery/feedback operational loop | `sandbox-feedback-loop-ops` — hybrid: intake/triage/weekly-raw-synthesis sandboxed with a mandatory human-review gate; contact upkeep, outreach sends, and session scheduling are host-side finishing |
| 16 | MVP | Run a product-market-fit diagnostic (Sean Ellis test + effort test) | `sandbox-pmf-diagnostic` |
| 17 | MVP | Run a stalled-PMF pivot diagnostic | `sandbox-pivot-diagnostic` |
| 18 | Launch | Run a full architectural/technical-debt audit and remediation sequencing | `sandbox-tech-debt-audit` — kept separate from 19: different evidence (structure/coverage vs control mapping), different consumers (sprint planning vs buyer compliance review) |
| 19 | Launch | Run a security/compliance review oriented to the target market's frameworks | `sandbox-compliance-review` |
| 20 | Launch | Audit founder operational load and design its replacement automation | `sandbox-founder-load-audit` |
| 21 | Launch | Design and stand up a lightweight, repeatable PM operating system | `sandbox-product-ops-system` — hybrid: the four operating artifacts are designed in-sandbox; the recurring layer (ceremonies, scheduled routing/compilation) is host-side finishing |
| 22 | Scale | Build a founder-bottleneck map and an unavailability stress test | `sandbox-bottleneck-stress-test` — kept separate from 20: Scale-stage scope (all workflows/approvals, absence simulation, fix designs), and it consumes 20's output as a mounted cross-reference |
| 23 | Scale | Convert institutional knowledge into enterprise procurement infrastructure | `sandbox-enterprise-procurement-pack` — hybrid: doc pack + observability/hardening plan sandboxed (build handed to `sandbox-feature-work`); the ongoing support-ops layer (ticket routing, renewal tracking, reporting cadences) is host-side finishing |
| 24 | Scale | Run an enterprise-readiness gap analysis for named target accounts | `sandbox-enterprise-gap-analysis` |
| 25 | Scale | Build a real go-to-market engine from scratch | `sandbox-gtm-engine` — the product-marketing infrastructure it specifies (demo environments, sandbox tenants, API references) is emitted as a build backlog for `sandbox-feature-work` |
| 26 | Scale | Turn domain expertise and institutional knowledge into reusable AI context/skills | `sandbox-domain-knowledge-codify` |
| 27 | Scale | Build a competitive-moat test suite from real edge cases | `sandbox-moat-test-suite` |
| 28 | Scale | Audit user-interaction data for a compounding feedback loop; draft a moat narrative | `sandbox-data-flywheel-audit` |
| 29 | Scale | Run a workflow-integration/lock-in audit across top customers | `sandbox-lockin-audit` |

## Process notes

- Every skill above went through a design → fresh-context critique → (where
  flagged) one fix round with re-critique, capped at two rounds total, before
  landing. 20 of 28 drafts passed critique on the first round; 8 required one fix
  round (all fix-round outcomes recorded in the run summary).
- `sandbox-playbook-skill-forge` (meta) additionally persists the pipeline that
  produced this library, parameterized over any prescriptive source document.
- No activity was excluded. Activities 2/4/5 are the only merge; everything else
  is one skill per activity.
