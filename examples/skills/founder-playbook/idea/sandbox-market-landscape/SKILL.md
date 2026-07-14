---
name: sandbox-market-landscape
status: alpha
description: Produce a cited market landscape for a stated customer hypothesis, including independent adversarial review. Use for research, not product implementation.
---

Use native Demesne sandbox capabilities; omit `model` unless the host supplies a concrete schema-valid value. A child writes only in its own `/out`; the parent later sees it at `/out/child/<name>/` and a subsequently spawned sibling sees it at `/in/previous-jobs/<name>/`.

## Procedure

1. Enumerate `/in` excluding `/in/previous-jobs`; find exactly one hypothesis input by declared contents. Record the selected mount in `/workspace/intake.json`. On ambiguity/absence, write `/out/FAILURE.md` and stop.
2. Run exactly four research avenues: competitors, market size, trends, and analogous markets. Dispatch `research-<avenue>` children; each writes `/out/finding.md`. Keep at most 8 in flight. Add no avenue unless the prompt explicitly lists it; record that reason in `intake.json`.
3. Poll to terminal. Accept only `status == succeeded`, `exit_code == 0`, and existing `finding.md`; Retry once using `research-<avenue>-r2`. On second failure, write job details to `/out/FAILURE.md` and stop.
4. Dispatch a fresh `critic-<avenue>` for every accepted research child, including `critic-analogous`. Each writes `/out/critique.md`; apply the same gate and one-retry rule.
5. Dispatch `compiler` only after all eight accepted outputs exist. It writes `/out/landscape.md`, `/out/metadata.json`, and `/out/appendices/`, with a finding and critique for each avenue. Gate all outputs.
6. The parent copies those outputs into its top-level `/out`, verifies them, and writes `SUMMARY.md` containing accepted job IDs and source counts. End at delivery.

## Output contract

Deliver `/out/landscape.md`, `/out/metadata.json`, `/out/appendices/`, and `/out/SUMMARY.md`. Appendices preserve finding plus critique for every avenue. Every substantive claim is cited; no output invents evidence for a failed avenue.
