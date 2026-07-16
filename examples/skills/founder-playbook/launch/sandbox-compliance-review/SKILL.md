---
name: sandbox-compliance-review
status: alpha
description: "Produce a report-only codebase compliance review for SOC 2, GDPR, or HIPAA. Do not implement fixes or certify compliance."
---

# Compliance review

Use this procedure as the orchestrator contract. The host supplies the complete procedure, output contract, product market, and absolute source directories in one `sandbox_agent` prompt. Omit `model` unless the host maps a requested capability tier to a valid installed model.

## Procedure

1. Discover mounts at runtime. List `/in`, exclude `previous-jobs`, identify the repository by the host-supplied basename, and fail with `/out/INCOMPLETE.md` on zero or multiple matches. Set `REPO=/workspace/repo` and run `cp -a "$MOUNT"/. "$REPO"`. Write `/workspace/scope.md` with selected frameworks, data inventory, and regulated surfaces.
2. Start `research-frameworks` only if current framework guidance is required. Put frameworks and inventory in its prompt; research has no `/in`. It writes `framework-ground.md`.
3. Dispatch the five `audit-*` agents and one `dep-scan` script in background, at most eight jobs. Each auditor writes `AUDIT-<dim>.md` with `file:line`, severity, exposure, control ID, and `HUMAN-REVIEW-REQUIRED` for auth, secrets, or data handling.
4. Run one dependency scanner only for a detected, locked ecosystem. Require its lockfile. Use one of: Node image: `npm exec --yes audit-ci@^7 -- --json > /out/dep-scan.tmp && mv /out/dep-scan.tmp /out/dep-scan.json`; Python image: `python -m pip install pip-audit==2.7.3 && pip-audit --format json > /out/dep-scan.tmp && mv /out/dep-scan.tmp /out/dep-scan.json`; Go image: `go run golang.org/x/vuln/cmd/govulncheck@v1.1.4 -json ./... > /out/dep-scan.tmp && mv /out/dep-scan.tmp /out/dep-scan.json`. Vulnerabilities are findings; install, lockfile, or scanner failures are `SCAN-FAILED`, not clean results.
5. At every barrier, repeat `sandbox_wait` while status is `running`. Accept a child only when `status == succeeded`, `exit_code == 0`, and its declared artifact exists and is nonempty at `/out/child/<name>/...`. On the first failure, preserve status, stdout, and stderr in `/out/failures/<name>.md`, retry once under `<name>-r2`, then cancel dependent jobs and write `INCOMPLETE.md` if retry fails. Do not synthesize or print `DONE` after incomplete coverage.
6. After ground succeeds, start auditors. After all audits and scan succeed, copy each child artifact into the orchestrator `/out` under the contract filename. Start fresh `synth-remediation` and `controls-workstream`; each must pass the same barrier. Copy their outputs to `/out`.
7. Write `/out/COMPLIANCE-REVIEW.md` with Scope, remediation sequence, controls workstream, recurring compliance checks, caveats, and incomplete/failure records. Verify every output below is nonempty before printing `DONE`.

## Output contract

```text
/out/COMPLIANCE-REVIEW.md
/out/scope.md
/out/framework-ground.md
/out/AUDIT-<dim>.md                 # five files
/out/dep-scan.json
/out/remediation-sequence.md
/out/controls-workstream.md
```

The remediation sequence uses `must-fix-before-enterprise-deal`, `next-sprint`, or `acceptable-with-documented-justification`. The controls workstream rates each buyer artifact `present`, `partial`, or `absent` against repository evidence.

## Launch inputs

Pass `directories: ["<absolute repository path>"]`; optionally pass evidence directories and `target-market.md`. State the repository basename, target market, data types, applicable frameworks, and any approved scanner command. This is audit-and-plan only.
