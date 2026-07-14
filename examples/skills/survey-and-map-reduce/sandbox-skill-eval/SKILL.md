---
name: sandbox-skill-eval
status: alpha
description: "Evaluate a supplied SKILL.md with a report-only preflight. Use when one skill needs skillcheck and four parallel Demesne reviews, yielding an evidence-backed repair plan for paths, portability, tool contracts, and imperative instructions."
---

Evaluate one skill, report-only. Require one explicit target path relative to the mounted repository root, ending in `SKILL.md`; permit only `[A-Za-z0-9._/-]+` and reject zero targets, more than one target, a directory, a glob, an absolute path, or a path containing `..`. Do not inspect or evaluate the target outside Demesne. Run this skill separately for each target.

Launch one `sandbox_agent(prompt=..., directories=[...])` orchestrator with `directories` containing the target repository's absolute host path. Do not name a model: use the credential-aware default so the pipeline works with Codex and Claude. The orchestrator must runtime-discover its one repository mount: enumerate `/in/*`, require exactly one directory, copy it to `/workspace/repo`, and resolve the supplied relative target under that copy. Never assume a mount basename, repository name, home directory, provider path, or machine path.

## Procedure

1. **Freeze the target.** After copying the repository, resolve the supplied relative target to its real path and require that it remains below `/workspace/repo/`. Verify the resolved file exists and its basename is `SKILL.md`. Copy it to `/workspace/evidence/TARGET-SKILL.md`; record the supplied relative path and SHA-256 digest there. Copy only the authoritative checked-in reference files needed to verify commands or tool signatures named by the target, with their digests; if none are available, record that gap rather than inventing a contract. This is the evidence packet. Do not edit the target, its repository, or the packet.

2. **Run the deterministic gate first.** Before creating any qualitative reviewer, call `sandbox_script(name="skillcheck", command=..., image="python", egress="package-managers")`. In that sandbox run these commands against the one resolved target, capturing combined raw output and numeric statuses. Ignore only the repository-required maturity-field warning. Shell-quote the resolved path when building the command:

   ```sh
   python -m pip install skillcheck==1.4.1
   skillcheck "$resolved_target" --format json --no-color --strict --ignore frontmatter.field.unknown
   ```

   Write the install output/status and `skillcheck` raw output/status to that child’s `/out`. Preserve the `skillcheck` status even when it is nonzero; make the wrapper complete so the report can distinguish a finding from an execution failure. Copy those files into `/workspace/evidence/`, add their SHA-256 digests, then make the evidence directory read-only. A failed install or unavailable executable is itself deterministic evidence; do not substitute another checker or version.

3. **Review the same evidence in parallel.** Read [lenses.md](lenses.md). Create one call per lens: `sandbox_agent(name="lens-paths", prompt=..., egress="none", background=true)`, `sandbox_agent(name="lens-portability", prompt=..., egress="none", background=true)`, `sandbox_agent(name="lens-tool-contracts", prompt=..., egress="none", background=true)`, and `sandbox_agent(name="lens-imperative-focus", prompt=..., egress="none", background=true)`; issue all four calls before any `sandbox_wait` call. Give every reviewer only `/workspace/evidence/` as its evaluation input, its exact lens, and a distinct output path under `/workspace/reviews/`. Reviewers must not modify evidence or the target, run commands, use network access, or evaluate another skill. Each report must state `clean` when appropriate and otherwise give: severity (`blocker`, `high`, `medium`, `low`), evidence filename and target line(s), issue, rationale, and concrete imperative repair. Separate a tool/checker execution failure from a defect in the target.

4. **Collect and synthesise.** After all four jobs were dispatched, use `sandbox_wait(job_id=..., timeout_seconds=...)` on every returned job ID until terminal. Record failed/cancelled reviewers as coverage gaps; do not silently replace their conclusions. Read all reports and write `/out/EXECUTIVE_SUMMARY.md`, then copy the immutable evidence and individual reports to `/out/evidence/` and `/out/reports/`.

   Rank findings by impact and confidence. Merge duplicate findings into one item with every supporting lens; preserve material disagreement by naming the disagreeing lenses and their rationale. Keep distinct concerns distinct. State the deterministic command statuses, reviewer coverage, clean lenses, evidence paths, and any execution/coverage limitations. End with the smallest ordered repair list. Do not modify the evaluated skill.

## Required orchestrator briefing

State the explicit relative target path and require the procedure above verbatim in substance: one discovered `/in` repository mount; target snapshot; pinned `skillcheck==1.4.1` before reviewers; four background reviewer calls issued before waiting; no provider-specific model selection; report-only output. Instruct every reviewer to cite `TARGET-SKILL.md` line numbers and evidence filenames, not host paths.

## Output contract

```
/out/
  EXECUTIVE_SUMMARY.md
  evidence/                     # target snapshot, digests, install/checker raw output and statuses
  reports/
    paths.md
    portability.md
    tool-contracts.md
    imperative-focus.md
```

The host reads `/out`; all inspection, checker execution, and qualitative evaluation occur in Demesne sandboxes.
