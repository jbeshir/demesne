---
name: sandbox-problem-discovery
description: Discover diverse, evidence-grounded customer problems for a strong technical founder, evaluate first-value feasibility and commercial evidence against a frozen gate contract, close authorized evidence gaps through bounded sandbox research, and emit standalone finalist briefs plus a machine-checkable handoff. Use for broad problem discovery, opportunity exploration, finalist selection, or preparing inputs for a later hypothesis stress test; stop before solution building, outreach, payment, private-data acquisition, or downstream stress testing.
---

# Sandbox Problem Discovery

Own discovery through evidence-aligned finalist selection. Do not build, market, contact buyers, spend money, obtain private data, give legal clearance, invoke another skill, or run the downstream hypothesis-stress-test workflow.

## Load the contract

Read [references/gate-contract-v1.md](references/gate-contract-v1.md) completely before planning or dispatch. Copy `assets/gate-contract-v1.json` into the run workspace as `gate-contract.json`, compute its SHA-256, and write the hash to `run-spec.json` before research. Never mutate or silently reinterpret the frozen contract. Validate artifacts with `node scripts/validate.mjs`.

## Apply the execution contract

- Use native sandbox jobs only when they are available and authorized. Give every child an explicit concrete `gpt-*` Codex model; use `gpt-5.6-sol` by default. Never use tier labels or omit the model.
- Put this exact constraint in every child prompt: `Do not delegate, spawn descendants, or invoke other agents. Work only on this assigned artifact.`
- Give each child one bounded artifact contract. Do not delegate the end-to-end workflow.
- Accept an artifact only when the job succeeded, exited zero, produced the required nonempty file, passed its schema, and had resolvable citations. Retry once only for provider/execution failure, nonzero exit, missing/empty output, malformed schema, or citation corruption. Treat a completed negative search as a valid unknown artifact, never as a retry reason.
- Preserve raw artifacts, accepted job IDs, attempts, model names, hashes, and rejection reasons in `execution-ledger.jsonl`.

## Run workflow A–K

### A. Intake and authorization

Enumerate mounted inputs while excluding `previous-jobs`. Use `/out` as the canonical caller-visible run root; stage elsewhere only if the fully validated tree is atomically copied to `/out` and revalidated there. Copy every accepted input source needed by a claim into `/out/raw/` without changing its bytes, record its original path and SHA-256 in `run-spec.json`, and cite the delivered `raw/` path. Record the objective, founder profile, geography, time horizon, allowed evidence/actions, forbidden actions, and canonical output path in `run-spec.json`. Default missing authority to offline/public read-only research; mark outreach, payments, counsel, and private-data access unauthorized.

### B. Freeze the decision system

Freeze the copied contract and its SHA-256 before any proposal or research job. Use only its seven score dimensions, weights, direction, normalized threshold, required G6/G7/G8 A states, and no-E rule; copy those values into `run-spec.json` instead of restating or replacing them. Give an unsupported dimension 0 and the appropriate unknown state, never an optimistic inferred score. Apply the strong-builder prior without inventing positive evidence. Never let a score override a hard red line.

### C. Propose independent territories

Create 6–8 territory jobs whose preambles differ in at least two named assumptions such as buyer type, pain cadence, workflow substrate, geography, or acquisition route. Require each to output schema-valid candidate records and claim citations. Keep territories independent; do not expose sibling proposals. Seek broad diversity rather than five generic research summaries.

### D. Aggregate, challenge outliers, and reject explicitly

Run one non-researching aggregator only after accepted territory artifacts pass the barrier. Deduplicate by customer, triggering event, and first-value job. Scrutinize every score at least 20% above the valid-candidate median and every unique high-evidence outlier. Preserve all removals in `rejection-ledger.jsonl` with the frozen criterion, evidence state, and reason. Preserve diversity across customer, event, and route without lowering the threshold.

### E. Select provisional finalists and research them

Select 2–4 finalists only from validated candidates. Apply the contract's narrow hard red lines first, then its scored threshold and evidence policy. A proven hard red line rejects; C/D/E may block selection but never becomes B. Retain promising unresolved candidates as `evidence-insufficient`, not disproven.

For each finalist, create `finalists/<id>/avenues/` and `finalists/<id>/index.json`. Research five bounded lanes: customer, market, competitor/alternatives/stagnation, technical/operations, and risk/procurement. These are internal lanes, not an invocation of `sandbox-product-research`. Assign gate ownership from the contract. Permit at most two optional lanes only when justified in `run-spec.json`. Require `finding.schema.json` records with exact propositions, attempts, sources, citations, evidence states, and missing propositions.

### F. Review gates independently

### G. Close authorized gaps within bounds

Give a fresh non-researching reviewer only the frozen contract and validated findings. Require exactly G1–G8 in `gate-review.json`. The reviewer may identify contradictions and E states but may not invent requirements, turn missing evidence into B, or acquire new evidence. Distinguish artifact coverage from gate-evidence coverage.

Create gap jobs only for C, D, or repairable E cells whose evidence is obtainable within authorization and could change the decision. Assign one to three related propositions, named observables, allowed sources/actions, and a stop rule. Run at most four jobs per finalist. Keep unauthorized outreach/payment/private-data/counsel gaps unresolved.

Re-review validated round-1 findings. Run at most two additional jobs per finalist only for a newly exposed dependency or repairable E. Stop after two rounds and six targeted jobs per finalist total. Stop earlier on a decisive hard-red-line B, when remaining gaps are unauthorized, or when more evidence cannot change the decision. Recompile against the original hash; emit the eight-state vector and counts without unsupported transitions.

### H. Recompile the frozen contract

Reconcile every status transition against accepted evidence and the original contract hash. Do not introduce a new criterion, mutate a threshold, or convert a budget/authorization limit into negative product evidence. Make the final decision unchangeable after the second review round.

### I. Write standalone problem reports

Write `finalists/<id>/problem-report.md` for every finalist, including its decision; topic; actors and current workflow; pain magnitude and recurrence; investigation and attempt log; claim ledger; competitors, alternatives, stagnation, and whitespace; technical/operational constraints; risks; unknowns; decision logic; source provenance; and optional depth clearly separated from required evidence. Make each report understandable without sibling artifacts.

### J. Write compatible selected briefs

For each `advance` decision, write `selected/<id>/hypothesis-brief.md` using `assets/finalist-brief.md`. Include a falsifiable customer-problem hypothesis and enough standalone context for the existing `sandbox-hypothesis-stress-test` input selector. In the manifest, set `next_skill` and require the host to mount exactly one named brief per downstream run; multiple advancing briefs are separate invocations, never one ambiguous mount. Reference that skill only as the intended next consumer. Do not reproduce its four avenues, call it, spawn it, or perform its work.

### K. Validate and hand off

Write `handoff-manifest.json` using its schema. Include `next_skill: sandbox-hypothesis-stress-test`, `mount_one_brief_per_run: true`, the contract hash, all finalist decisions, report paths and hashes, selected brief paths and hashes where applicable, eight evidence states, hard-red-line findings, unresolved gaps, job counts, and authorization limits. Run the validator against the complete top-level `/out` tree. Deliver only after schema validation, hash reconciliation, exact G1–G8 review coverage, 4+2 budget enforcement, citation-bearing A/B cells, report existence, and selected-brief existence checks pass. On failure, place defects and invalid artifacts under `/out/failures/quarantine/`, write `/out/FAILURE.md`, and do not claim a valid handoff.

## Required run outputs

Produce `raw/` provenance inputs, `run-spec.json`, frozen `gate-contract.json` plus hash, `territories/`, `aggregate/` with ranking and outliers, `execution-ledger.jsonl`, `candidates.jsonl`, `rejection-ledger.jsonl`, `finalists/` indexes/avenues/reviews/gap-plans/attempts/problem reports, `selected/` hypothesis briefs, `handoff-manifest.json`, `REPORT.md`, `SUMMARY.md`, and `failures/quarantine/`. Report artifact coverage separately from gate-state counts.
