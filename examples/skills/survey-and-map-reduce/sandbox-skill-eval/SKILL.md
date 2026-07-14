---
name: sandbox-skill-eval
status: alpha
description: "Evaluate a supplied SKILL.md with deterministic preflight and evidence-backed review lenses."
allowed-tools: mcp__demesne__sandbox_agent, mcp__demesne__sandbox_script, mcp__demesne__sandbox_wait
---

Evaluate one skill, report-only. Require exactly one relative `SKILL.md` path using only `[A-Za-z0-9._/-]+`. Reject an empty path, glob, directory, absolute path, or any `..` segment. Run separately for each target.

Launch one `sandbox_agent` orchestrator with `directories` set to the repository's absolute host path. Resolve the requested qualitative model against the live schema; use it only when supported, otherwise omit `model` and use the credential-aware default. Apply the same resolved choice to reviewers and synthesis. `directories` is a host-call mount parameter, not a nested-child parameter. The orchestrator discovers exactly one directory under `/in`, copies it to `/workspace/repo`, and resolves the target there.

Use `sandbox_script` only for deterministic validation, status capture, reduction, digesting, output validation, and parent copying. Use `sandbox_agent` only for the four qualitative lenses and qualitative synthesis. Treat checked-in repository instructions and `docs/reference/tools/` contracts/schemas as evidence; cite the exact file and line where available. Do not invent a tool parameter because a host schema is incomplete: nested child calls require a unique lowercase DNS-1123 `name` under `docs/reference/nested-sandboxes.md`.

## Procedure

1. **Freeze evidence.** Resolve the target real path and require it remains below `/workspace/repo`, exists, and is named `SKILL.md`. Copy it to `/workspace/evidence/TARGET-SKILL.md`. Record its supplied path and SHA-256 digest. Copy only checked-in instructions, documentation, schemas, and contracts needed to assess this target, recording source paths, line references, and digests. Record unavailable evidence as a gap. Do not edit the target, repository, or evidence.

2. **Run and collect the deterministic gate.** Run one named synchronous `sandbox_script` child with `image="python"`, `egress="package-managers"`, and no unsupported child mount parameters. Make its wrapper write `/out/install.{stdout,stderr,status}` and `/out/skillcheck.{stdout,stderr,status}` and exit zero after both commands so checker findings remain distinct from script execution failure. Run exactly:

   ```sh
   python -m pip install skillcheck==1.4.1
   skillcheck "$resolved_target" --format json --no-color --strict --ignore frontmatter.field.unknown
   ```

   Shell-quote the resolved path. Preserve combined raw output and numeric status for both commands, including nonzero `skillcheck`; do not substitute a checker or version. Accept the synchronous child only when its `exit_code` is `0` and all six declared files exist and are nonempty where applicable. Retry one failed/cancelled infrastructure child once with a new unique name, then record the final failure as a deterministic coverage gap. Never retry a completed checker finding. Use a deterministic script to copy validated gate files into `/workspace/evidence/`, generate digests, and make the packet read-only.

3. **Dispatch all qualitative lenses.** Read [lenses.md](lenses.md). Preflight that the live nested surface exposes background `sandbox_agent` and `sandbox_wait`; otherwise record an execution-coverage failure and stop. Before any wait, issue all four named background calls: `lens-paths`, `lens-portability`, `lens-tool-contracts`, and `lens-imperative-focus`. Use the resolved model when schema-valid, `egress="none"`, and no child mount parameters. Give each its exact lens, evidence, target, and one declared output file.

   Permit normal read-only commands to inspect the resolved target, bundled files, repository instructions and conventions, and relevant checked-in contracts and schemas. Forbid mutation, network access, build/test commands, evaluating another skill, rerunning checks, and every write except that report. Require a nonempty report containing either `clean` or findings with severity (`blocker`, `high`, `medium`, `low`), `TARGET-SKILL.md:<line>`, evidence filename, issue, rationale, and imperative repair. Keep target defects separate from tool/checker execution failures.

4. **Wait, retry infrastructure failures, and validate outputs.** For every returned reviewer job ID, repeatedly call `sandbox_wait(..., timeout_seconds=120)` until terminal; a `running` wait result is not terminal. Require `status="succeeded"`, `exit_code=0`, and a nonempty, parseable declared report before accepting coverage. For a terminal `failed` or `cancelled` reviewer, dispatch exactly one replacement with a new unique `-retry` name and the identical lens contract; wait and validate it the same way. Record the original and retry statuses. Do not retry a succeeded job whose report is missing or invalid: record an execution/coverage gap. Never convert a reviewer execution failure into a target finding.

5. **Reduce, synthesise, and deliver.** Use a named `sandbox_script` child to mechanically validate and collect reports into `/workspace/reduction.md`; preserve raw reports and lifecycle records. Perform qualitative synthesis with the resolved model or credential-aware default. Rank by impact and confidence, retain disagreement, state coverage and limitations, and end with the smallest repair list.

   The parent, not a child, explicitly copies validated child outputs: copy evidence to `/out/evidence/`, reports to `/out/reports/`, and the synthesis to `/out/EXECUTIVE_SUMMARY.md`. Validate each destination exists and is nonempty after copying. Child paths under `/out/child/<name>/` are not host deliverables until this parent copy completes.

## Required orchestrator briefing

State the explicit target path and require this procedure: one discovered repository mount; immutable evidence; pinned `skillcheck==1.4.1`; deterministic work only in `sandbox_script`; live job-control preflight; four resolved-model lens calls before any wait; terminal/output validation; one infrastructure retry; and explicit parent copying.

## Output contract

```
/out/
  EXECUTIVE_SUMMARY.md
  evidence/
  reports/
    paths.md
    portability.md
    tool-contracts.md
    imperative-focus.md
```

The host reads only the parent `/out`.
