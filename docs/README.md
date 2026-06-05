# Demesne documentation

Demesne is an agent-agnostic, local, containerised agent-orchestration MCP server. You — the human running demesne — drive it through your agent of choice (Claude Code, Claude Desktop, VS Code, or any other MCP client). This doc tree is split by audience:

- **User docs** below cover wiring demesne into your agent, equipping that agent to use it, and the kinds of work you can ask for.
- **Agent reference** (`reference/nested-sandboxes.md` etc.) is consumed primarily by agents running inside a sandbox, but useful to humans who need the wire-level details.
- **Contributors** building from source / hacking on demesne itself: see [CONTRIBUTING.md](../CONTRIBUTING.md).

## Tutorials

- [Quickstart](tutorial/quickstart.md) — your first `sandbox_script` call in five steps.

## How-to guides

- [Wire demesne into your MCP client](how-to/wire-into-mcp-client.md) — config snippets for Claude Code, Claude Desktop, and VS Code, plus the env-var reference.
- [Equip your agent for demesne](how-to/equip-your-agent.md) — paste-into-your-system-prompt block so your top-level agent knows how to use demesne.
- [Kinds of work you can request](how-to/request-work.md) — what to ask your agent for: one-off scripts, research, delegated agent tasks, persistent sessions, multi-agent orchestration.
- [Develop demesne skills](how-to/develop-demesne-skills.md) — author reusable skills that compose demesne pipelines (research → plan → implement → verify, verifier/judge, checkpointing).
- [Share a host directory with a sandbox](how-to/share-a-host-directory.md) — operator-side allowlist on both sides.
- [Edit the host MCP allowlist](how-to/edit-host-mcp-allowlist.md) — control which host MCP tools are re-exposed to containerised agents.

## Reference

- [Host requirements](reference/requirements.md) — container runtime, OpenSandbox config, rootless podman pipe-page cap.
- [Configuration](reference/configuration.md) — environment variables and container image allowlist.
- [Tool reference](reference/tools/) — per-tool parameter tables, sample requests, error tables.
- [Nested sandboxes](reference/nested-sandboxes.md) — child output layout, `name` rules, `/in/previous-jobs/<name>`, task-prompt structure, copy-to-`/out` gotcha.
- [`results.json`](reference/results-json.md) — per-run results JSON written to `/out`.
- [`usage.json`](reference/usage-json.md) — per-run token usage and cost JSON.
- [`transcript.jsonl`](reference/transcript-jsonl.md) — agent reasoning trace NDJSON.

## Explanation

- [Architecture](explanation/architecture.md)
- [Key concepts](explanation/key-concepts.md)
- [Trust boundary](explanation/trust-boundary.md)
- [Dependencies](explanation/dependencies.md)
