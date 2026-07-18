---
name: sandbox-hypothesis-stress-test
status: alpha
description: Test a customer-problem hypothesis against independent disconfirming evidence and deliver a revised, evidence-cited hypothesis. Do not build or market a solution.
---

Use native Demesne sandbox capabilities. Omit `model` unless the host provides a concrete schema-valid value; tier labels are never model values.

## Procedure

1. Enumerate `/in` excluding `/in/previous-jobs`; select exactly one hypothesis/brief by declared contents. On ambiguity or absence, write `/out/FAILURE.md` and stop. Copy it to `/workspace/hypothesis.md`.
2. Define exactly four avenues: customer behaviour, alternatives, economics, and adoption constraints. Dispatch background hunters named `disconfirm-<avenue>`; each writes `/out/finding.md`. A parent sees it at `/out/child/disconfirm-<avenue>/finding.md`; a later sibling sees it at `/in/previous-jobs/disconfirm-<avenue>/finding.md`.
3. Poll each hunter until terminal. Accept only `status == succeeded`, `exit_code == 0`, and an existing `finding.md`. Retry once with `disconfirm-<avenue>-r2`. On another failure, write an object with avenue, job ID, status, exit code, and reason to `/workspace/missing-avenues.jsonl`.
4. If any avenue is missing, copy the missing record to `/out/missing-avenues.jsonl`, write `/out/FAILURE.md`, and stop. Otherwise dispatch `compiler` after all four accepted hunters complete; it writes `/out/hypothesis.md`, `/out/counter-case.md`, `/out/discovery-tests.md`, and `/out/evidence.jsonl`.
5. Gate the compiler. Dispatch `sharpness-auditor-r1` to write `/out/audit.md`. On `REVISE`, dispatch `sharpener-r2` to rewrite the compiler outputs, then `sharpness-auditor-r2` to write the authoritative audit. Permit two audit rounds.
6. The parent copies the last accepted draft and audit/evidence files into its own `/out`, verifies them, and writes `SUMMARY.md`. A parent or a dedicated collector child may perform this copy, but the final files must be in this run's top-level `/out`.

## Output contract

Deliver `/out/hypothesis.md`, `/out/counter-case.md`, `/out/discovery-tests.md`, `/out/evidence.jsonl`, `/out/audit.md`, and `/out/SUMMARY.md`. `SUMMARY.md` names all four avenues and their accepted job IDs; every counterclaim cites its finding.
