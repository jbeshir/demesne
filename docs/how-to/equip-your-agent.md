# Equip your agent for demesne

Paste the block below into your top-level agent's system prompt (Codex, Claude Code, or another) so it knows demesne's tools and when to reach for each. Keep it short: the sub-agents demesne spawns get their own instructions automatically, so this only needs to orient the agent that *calls* demesne.

## System prompt block

```
## demesne tools

Run shell commands, scripts, and AI sub-agents in isolated containers via demesne:

- **sandbox_script** — one-shot: run a shell command in a fresh container. For quick build / test / transform steps.
- **sandbox_create / sandbox_exec / sandbox_destroy** — a persistent container: run several commands that share a filesystem (e.g. install deps, then use them), then tear it down.
- **sandbox_agent** — delegate a task to a sub-agent running in a container against a prompt. For work needing its own reasoning loop; it reads the inputs you mount. Cannot use open egress.
- **sandbox_research** — like sandbox_agent but with open internet and no input mounts; for web research.

Egress per call: `none` (vendor proxy only) or `package-managers` (npm/PyPI/conda); `open` is sandbox_research-only.

For sandbox_agent / sandbox_research, give the sub-agent a clear prompt, and optionally `output_path` / `output_format` / `success_criteria` — these become a definition-of-done the sub-agent must satisfy. Each run's files come back under the returned `output_dir`.
```

The sub-agents can themselves spawn children and surface results up the tree; demesne instructs them on that, so it's out of scope here. See the [Nested sandboxes reference](../reference/nested-sandboxes.md) for those mechanics.
