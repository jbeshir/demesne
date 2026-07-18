---
name: sandbox-product-research
description: Research a product decision across defined evidence avenues and deliver a prioritized report.
---

# Product research

1. Enumerate `/in`, exclude `previous-jobs`, require one mounted repository directory, and derive all paths from it. Define five mandatory avenues: customer, market, competitor, technical, and risk; use 0–2 optional avenues, for a total of 5–7. Use ICE only: score impact, confidence, and effort 1–5; rank by `(impact × confidence) / effort`.
2. If comparables research is requested, `comparables` writes `/out/comparables.md`. After a success barrier, copy `/out/child/comparables/comparables.md` to `/workspace/comparables.md`; only children spawned afterward may instead read `/in/previous-jobs/comparables/comparables.md`.
3. For every avenue, wait repeatedly while running. Accept only `succeeded`, exit 0, and nonempty `finding.md`. Retry once with a unique name; then cancel abandoned work and deliver a partial report that names missing avenues and coverage percentage.
4. The compiler reads validated artifacts and writes `REPORT.md`. The parent copies it to `/out/REPORT.md`. A collector script is allowed but is a workflow choice, not a tool limitation.

Pass this procedure as the prompt; do not duplicate it in launch instructions.
