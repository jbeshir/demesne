---
name: sandbox-mvp-scope-guardrail
status: alpha
description: "Create or pressure-test a narrow MVP scope charter against evidence. Use for a new charter or a proposed amendment; do not use it to approve implementation."
allowed-tools: mcp__demesne__sandbox_agent, mcp__demesne__sandbox_script, mcp__demesne__sandbox_wait
---

Use host-default models. Required capabilities are agents, scripts, waits, and parent-side copies; verify the host maps them before launch.

## Procedure

1. Choose `CHARTER` or `AMENDMENT`. Discover all requested mounts by enumerating `/in/*` and matching prompt labels/content; fail on ambiguity. AMENDMENT requires a resolved `scope.md`. Create `/workspace/evidence-index.md`, logging unreadable evidence.
2. In CHARTER mode, write a draft scope with thesis, core loop, DOES, DOES-NOT-for-now, and measurable amendment criteria. A feature passes DOES only if removing it prevents the core loop or its retention/revenue/referral signal; move every feature that fails to DOES-NOT-for-now.
3. Dispatch independent role-named debaters in the background. Each writes `/out/position.md`. Keep a role-to-`job_id` map. Wait while running; accept a role only if succeeded, exit code zero, and `position.md` exists. Retry once as `<role>-r2`; otherwise write a role-specific failure to `/out/FAILURE.md` and stop.
4. Build an explicit role-to-output-path map from accepted terminal results; do not assume a `/in/previous-jobs` layout. Dispatch the fresh judge with those paths. Apply the same gate to `scope.md` or `amendment-verdict.md`.
5. In the orchestrator, create `/out/pressure-test/<role>/` (CHARTER) or `/out/debate/<role>/` (AMENDMENT); copy each accepted `position.md` and `transcript.jsonl` from its child output, failing delivery when required artifacts are absent. Copy the judge result and write `metadata.json` with roles, paths, retries, and mode.

`scope.md` must contain MVP Thesis, DOES, DOES-NOT, and Amendment Criteria. An amendment verdict must be `ADD-NOW`, `DEFER-WITH-TRIGGER`, or `REJECT`, cite evidence, and preserve dissent.
