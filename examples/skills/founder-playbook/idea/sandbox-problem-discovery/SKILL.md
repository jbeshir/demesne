---
name: sandbox-problem-discovery
description: Discover diverse, evidence-grounded customer problems for a strong technical founder, compare them against a mounted archive, evaluate feasibility and commercial evidence against a frozen gate contract, and emit standalone reports plus an immutable integration-ready bundle and machine-checkable one-brief handoff. Use for broad problem discovery, archive-aware opportunity exploration, finalist selection, or preparing inputs for a later hypothesis stress test; stop before solution building, repository integration without authorization, outreach, payment, private-data acquisition, or downstream stress testing.
---

# Sandbox Problem Discovery

Own discovery through evidence-aligned selection. Do not build, market, contact buyers, spend money, obtain private data, give legal clearance, integrate into a repository without authorization, invoke another skill, or run downstream stress testing.

## Load the contracts

Read [references/gate-contract-v1.md](references/gate-contract-v1.md) and [references/archive-integration-reporting.md](references/archive-integration-reporting.md) completely. Copy `assets/gate-contract-v1.json` to the run as `gate-contract.json`, hash it into `run-spec.json` before research, and never mutate or reinterpret it. Validate with `node scripts/validate.mjs`.

## Apply the execution contract

- Use native sandbox jobs only when available and authorized. Give every child an explicit `gpt-*` model; default to `gpt-5.6-sol`.
- Put this exact constraint in every child prompt: `Do not delegate, spawn descendants, or invoke other agents. Work only on this assigned artifact.` Give each child one bounded artifact.
- Accept only a succeeded, zero-exit, nonempty, schema-valid artifact with resolvable citations. Retry once only for execution, artifact, schema, or citation failure. A completed negative search is valid unknown evidence.
- Preserve raw artifacts, accepted job IDs, attempts, models, hashes, and rejection reasons in `execution-ledger.jsonl`.

## Run workflow A–K

### A–B. Intake, authorization, and freeze

Inventory mounted inputs excluding `previous-jobs`. Use `/out` as the canonical root. Copy cited inputs byte-for-byte to `raw/` and record original paths/hashes. Record objective, founder profile, geography, horizon, actions, canonical path, archive presence, integration authorization, and any discovered integration contract in `run-spec.json`. Default to public/offline read-only authority.

Freeze the contract hash and its seven scores, weights, direction, threshold, required G6/G7/G8 A states, and no-E rule before proposals. Unsupported dimensions score 0. Never let score override a hard red line.

### C. Propose independent territories

Create 6–8 independent territory jobs differing in at least two named assumptions. Require schema-valid candidates and claim citations. Do not expose sibling proposals.

### D. Compare archive, aggregate, and transition

Before ranking, write a typed inventory for every mounted archive record, including inactive records, with stable IDs, source hashes, statuses, and an inventory digest. Compare every candidate to every inventory record over all nine mechanism dimensions, mark every distance-minimizing neighbor, and link comparisons to that digest. Append sequence- and prior-digest-linked `keep`/`dedup`/`supersede` novelty decisions whose comparison reference and neighbor set resolve exactly; labels alone never establish novelty. Do not mutate archive state.

Then aggregate. Deduplicate by mechanism, scrutinize every score at least 20% above the valid-candidate median and every unique high-evidence outlier, preserve removals in `rejection-ledger.jsonl`, and append a causal selection transition for every candidate.

### E. Select and investigate provisional finalists

Write each unordered pair once in the pairwise nine-dimension diversity matrix, using only the named dimensions and the declared adjacency threshold. Retaining adjacent finalists requires a waiver naming non-identical mechanism-delta dimensions plus resolving claim and citation evidence. Select 2–4 validated finalists. Apply narrow hard red lines, then frozen score/evidence policy. C/D/E may block but never becomes B.

Research customer, market, competitor, technical, and risk lanes for each finalist. Require findings with claim IDs, exact propositions, dated query/locator logs, named source classes and access limits, citations, bounded results, states, and remaining propositions. Link score/gate bases to claims and citations. Apply the competitive-stagnation anchors and six-part barrier decomposition in the archive/reporting reference; apply the strong-builder prior only to build complexity.

### F–G. Review before gaps, then close bounded gaps

Give a fresh non-researching reviewer only the frozen contract and validated findings. For initial, post-round-1, and final reviews, record the hash, independent-reviewer role, independence attestation, and acceptance timestamp in the execution ledger. Write and accept exactly G1–G8 in `reviews/initial.json` before planning or starting gaps. Derive round 1 from it and record its path/hash in the plan and each gap-job ledger entry. Target only decision-relevant C, D, or repairable E cells using one to three propositions, observables, allowed actions, and a stop rule; run at most four jobs.

Write/hash `reviews/post-round-1.json`; derive round 2 from it and record its path/hash. Use at most two more jobs for newly exposed dependencies or repairable E, then write `reviews/final.json`. Stop after two rounds/six jobs, a decisive hard-red-line B, unauthorized gaps, or when evidence cannot change the decision. Never turn bounded search into universal absence.

### H. Recompile the frozen contract

Reconcile every state and selection transition against accepted evidence and the original hash. Do not add criteria or convert authorization/budget limits into negative evidence. Make the final decision unchangeable after final review.

### I. Write standalone reports

Write `reports/<id>.md` for every meaningfully investigated candidate, retaining negative searches, competitors, claim-linked reasoning, transition history, and reconsideration evidence. Write `reports/<id>.attestation.json` with resolving finding IDs, transition IDs, and citations for each required section. Also write a richer standalone `finalists/<id>/problem-report.md`; its attestation additionally covers decision, actors/workflow, pain/cadence, attempts, claims, alternatives/stagnation, constraints, risks, unknowns, decision logic, provenance, and separately labeled optional depth.

### J. Preserve selected hypothesis handoff

Only for reevaluated `advance` decisions, write `selected/<id>/hypothesis-brief.md` from `assets/finalist-brief.md`. Set `next_skill: sandbox-hypothesis-stress-test` and `mount_one_brief_per_run: true`. Each advancing brief is a separate downstream invocation. Do not call or reproduce that skill.

### K. Package, validate, and hand off

Write repository-neutral `catalog.json`, the existing `handoff-manifest.json`, then immutable `bundle-manifest.json`. Model stable track/alias inputs as typed external identities and reject any value colliding with any fresh run candidate ID. Never claim integration without explicit authorization, a real hash-verified contract file, and an integration completion ledger whose completed actions and output hashes resolve. Validate the full `/out` tree and hashes. On failure quarantine defects, write `FAILURE.md`, and claim neither valid bundle nor handoff.

## Required outputs

Produce `raw/`, `run-spec.json`, `gate-contract.json`, `territories/`, `archive/` inventory/comparisons/novelty decisions, `aggregate/` ranking/outliers/diversity matrix, `execution-ledger.jsonl`, `candidates.jsonl`, `rejection-ledger.jsonl`, `selection-transitions.jsonl`, `reports/`, `finalists/` indexes/avenues/reviews/gap-plans/attempts/problem reports, `selected/` briefs, `catalog.json`, `handoff-manifest.json`, `bundle-manifest.json`, `REPORT.md`, `SUMMARY.md`, and `failures/quarantine/`. Report artifact coverage separately from gate-state counts.
