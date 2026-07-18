---
name: sandbox-docs-quality
status: alpha
description: "Audit mounted documentation against its codebase without rewriting either."
---

Audit completeness, accuracy, placement, and consistency. This is report-only.

1. The host launches one `sandbox_agent` with the repository in `directories`; omit `model` unless it supplies a concrete allowed value. The orchestrator enumerates `/in`, ignores `previous-jobs`, requires one repository directory, and stops with `/out/EXECUTIVE_SUMMARY.md` `status: input-invalid` if ambiguous.
2. Copy that mount to `/workspace/repo`. Define the code ground-truth map and four lenses: completeness, accuracy, information architecture, and consistency.
3. Dispatch `detect-<lens>` children, at most four total. Each reads docs and code and writes `/out/REPORT.md`; every finding cites the document and ground-truth `file:line`. Clean is valid.
4. For each child, wait through `running` results. Accept only `succeeded`, exit code 0, and nonempty `/out/child/<name>/REPORT.md`; retry once as `<name>-retry`, then write a coverage gap. Do not synthesize an unvalidated lens.
5. Copy accepted reports to `/out/reports/NN-<lens>.md`. Write `/out/EXECUTIVE_SUMMARY.md` with findings, confirmed archetypes, a concrete information-architecture recommendation, remediation order, and coverage gaps. Print `DONE` only after all lenses are accepted or marked gaps.

## Output contract

```
/out/EXECUTIVE_SUMMARY.md
/out/reports/NN-<lens>.md
```
