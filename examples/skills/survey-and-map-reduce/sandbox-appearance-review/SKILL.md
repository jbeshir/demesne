---
name: sandbox-appearance-review
status: alpha
description: "Review a mounted interface's rendered appearance and propose visual improvements without changing code."
---

Review only what a screenshot can establish. Do not test behavior or accessibility beyond visible contrast, signalling, focus indicators, and target size. `lenses.md` is the authority for lens rules and simulations.

1. The host mounts the front-end directory and embeds this skill's complete `lenses.md` in the orchestrator prompt. The orchestrator writes it verbatim to `/workspace/lenses.md`, enumerates `/in`, ignores `previous-jobs`, and requires exactly one intended front-end directory. On ambiguity, write `/out/SUMMARY.md` with `status: input-invalid` and stop.
2. Copy the mount to `/workspace/repo`; inspect committed build and harness instructions. Write `/out/RENDER-PLAN.md` with routes, states, viewports, themes, readiness signals, and unreachable cells. Choose the build image and command from those instructions, not a fixed Node assumption.
3. Run named build and capture scripts. Default build egress to `none`; use `package-managers` only for a named missing locked dependency. For either stage, require terminal `succeeded`, exit 0, and declared nonempty artifacts; retry once using a new name. On a second failure write the diagnostic and stop rather than review stale output.
4. Capture with one selected browser engine and recorded version, DPR, viewport, color scheme, and resolved fonts. Wait for font readiness. Put those values, console errors, overflow, and every unavailable cell in `/workspace/gallery/capture-report.json`; unavailable fonts/resources make a cell `not-captured`. Produce the lens-required colour variants in the same capture pass.
5. Dispatch the ten `lens-<NN>-<slug>` reviewers, at most 8 in flight, with their exact lens section, overlap rule, gallery, render plan, and required JSON schema. Use the provider default model unless the host supplies a concrete allowed model. Wait until terminal; require `succeeded`, exit 0, nonempty `/out/child/<name>/findings/lens-<NN>-<slug>.json`, and valid JSON shape. Retry once under a new name; otherwise record a coverage gap.
6. Copy every accepted findings file to parent `/out/findings/`. Synthesize only validated findings, applying `lenses.md` overlap rules. Copy `/workspace/gallery` to `/out/gallery`; write the review and summary with captured/not-captured cells, per-lens accepted coverage, gaps, and proposal counts. Print `DONE` only after validating the output contract.

## Output contract

```
/out/RENDER-PLAN.md
/out/gallery/capture-report.json
/out/gallery/*.png
/out/findings/lens-<NN>-<slug>.json
/out/APPEARANCE-REVIEW.md
/out/SUMMARY.md
```
