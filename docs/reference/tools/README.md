# Tool reference

Demesne exposes eight MCP tools over its stdio JSON-RPC interface. Three categories cover the full lifecycle: a single-shot script runner, a persistent-sandbox lifecycle group (create / exec / upload / download / destroy), and two AI agent runners. Each tool page documents its parameters (sourced from the Go registration in `internal/server/server.go`), MCP annotations, a sample JSON-RPC request/result, and an error table grounded in `internal/sandbox/`.

| Tool | Summary | Category |
|------|---------|----------|
| [`sandbox_script`](sandbox_script.md) | Run a single shell command in a fresh sandbox and return its stdout and stderr. | single-shot |
| [`sandbox_create`](sandbox_create.md) | Create a persistent sandbox and return its handle. | persistent |
| [`sandbox_exec`](sandbox_exec.md) | Run a shell command in an existing sandbox. | persistent |
| [`sandbox_upload`](sandbox_upload.md) | Copy a host file into an existing sandbox. | persistent |
| [`sandbox_download`](sandbox_download.md) | Copy a file out of an existing sandbox to the host. | persistent |
| [`sandbox_destroy`](sandbox_destroy.md) | Destroy an existing sandbox. | persistent |
| [`sandbox_agent`](sandbox_agent.md) | Run an AI agent inside a fresh sandbox against the caller's prompt. | agent |
| [`sandbox_research`](sandbox_research.md) | Run a long-running research agent in a fresh sandbox with unrestricted outbound internet access. | agent |

## Structured output

Every tool returns its result as both a human-readable text payload and a `structuredContent` object validated against a declared [`outputSchema`](https://modelcontextprotocol.io/specification/2025-11-25/server/tools#output-schema). Clients that support structured output — including Claude Code and the Codex CLI — consume the structured object and ignore the text block, which remains as a fallback for clients that don't. Each tool page lists its specific `structuredContent` fields.

## Other reference pages

- [Host requirements](../requirements.md) — container runtime, OpenSandbox configuration, rootless podman pipe-page cap.
- [Configuration](../configuration.md) — environment variables and container image allowlist.
- [Nested sandboxes](../nested-sandboxes.md) — child output layout, `name` rules, `/in/previous-jobs/<name>`, task-prompt structure, copy-to-`/out` gotcha.
- [transcript.jsonl](../transcript-jsonl.md) — NDJSON written to `<output_dir>/transcript.jsonl` by every agent run; event format, `ResultText` extraction, and relationship to the MCP `stdout` field (32 KiB cap).
