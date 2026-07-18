---
name: sandbox-pivot-diagnostic
status: alpha
description: "Decide whether repeated MVP evidence supports continuing or a defined pivot. Use after three comparable cycles; do not use it to choose a pivot from a single weak signal."
allowed-tools: mcp__demesne__sandbox_agent, mcp__demesne__sandbox_script, mcp__demesne__sandbox_wait
---

Use host-default models. The host must provide agent, script, wait, and copy capabilities and map their names before launch.

## Procedure

1. Discover cycle mounts by enumerating `/in/*`; match them to the launch labels and require three comparable cycles. Write resolved paths and the benchmark to `/workspace/inputs.md`.
2. A trigger is met only when, in at least two of the three cycles, the primary PMF metric misses its predeclared target by at least 10 percentage points or declines by at least 10 percentage points from the first cycle, and no secondary metric improves by 10 points or more. Otherwise produce only `/out/pivot-diagnostic.md` with sections `Verdict: KEEP-ITERATING`, `Benchmark Table`, `Evidence Gaps`, and `Next Measurement`; do not create lenses, advocates, Pivot Specification, or Dissent.
3. When triggered, dispatch three independent lenses with `background:true`: evidence, customer/problem, and option economics. Each writes `/out/lens.md`. Wait while running; accept only succeeded, exit code zero, and existing output. Retry once as `<name>-r2`; on a second failure write `/out/FAILURE.md` and stop.
4. Dispatch pivot advocate and status-quo advocate only after accepted lenses. Apply the same gate. Dispatch a fresh judge only after both cases exist; it writes `/out/pivot-diagnostic.md` and, for `PIVOT`, `/out/validated-hypothesis.md` containing problem, user, evidence, falsifier, and next test. This is the explicit handoff input for interview-kit design.
5. Copy accepted child artifacts from `/out/child/<name>/` into parent `/out/supporting/`; record states, retries, missing evidence, and the trigger calculation in `SUMMARY.md`.

Lens checklists: evidence—verify denominators and cohort comparability; customer/problem—separate repeated pain from founder interpretation; economics—state acquisition, retention, and switching-cost assumptions. Preserve the losing case as Dissent in a pivot verdict.
