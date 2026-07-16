---
name: sandbox-mvp-architecture-charter
status: alpha
description: "Create and pressure-test an MVP architecture charter before implementation. Use with a validated concept and, when applicable, a mounted prototype repository; do not use it to build product code."
allowed-tools: mcp__demesne__sandbox_agent, mcp__demesne__sandbox_script, mcp__demesne__sandbox_wait
---

Use host-default models. Required capabilities are agents, deterministic scripts, waits, and parent-side copies; map them on the host before launch.

## Procedure

1. Discover concept and optional repository mounts by enumerating `/in/*` and matching labels/content; fail on ambiguity. A repository mount is mandatory when the request says a prototype exists. Copy it with `cp -a <resolved>/. /workspace/repo`; otherwise write `/workspace/GREENFIELD`.
2. If a repo exists, run `stack-inventory` with one valid image selected from `node`, `python`, `go`, or `anaconda`; if none applies, write `/workspace/stack-unsupported.md`. Write `/workspace/decisions.json` with `id`, `question`, `options`, and `axis`.
3. Dispatch `architect` in the background. It writes `/out/draft-charter.md`. Wait while running; accept only succeeded, exit code zero, and the file exists. Retry once as `architect-r2`; otherwise write `/out/FAILURE.md` and stop.
4. Dispatch the four named adversaries after the draft passes the gate. Each writes `/out/attacks.md`; apply the same one-retry gate. Dispatch `charter-compiler-r1` only with accepted draft/attack paths. It writes `/out/ARCHITECTURE_CHARTER.md` and `/out/change-log.md`.
5. A revision is material only if it changes three or more existing decisions or adds a decision point. For one material r1 result, run `adversary-<lens>-r2` and `charter-compiler-r2`, passing the explicit accepted child paths from r1; never use wildcard or assumed sibling mounts. Stop after r2.
6. Dispatch `scaffold` only after the accepted final compiler output. It writes `/out/SESSION_TEMPLATE.md` and `/out/SESSION_LOG_FORMAT.md`. Apply the same gate.
7. In the orchestrator copy accepted child output from `/out/child/<name>/` into parent `/out`: `ARCHITECTURE_CHARTER.md`, `CLAUDE.md`, `AGENTS.md` (thin adapters that point to the charter), scaffold files, draft, attacks, and change log. Write `metadata.json` with resolved mounts, rounds, decisions, retries, and failures.

Every charter decision states choice, tradeoff, why, reversibility, and revisit trigger. The compiler rejects a decision missing why. Do not create a code branch or edit product code.
