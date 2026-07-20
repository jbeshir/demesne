# Tool reference

Demesne exposes eleven MCP tools over its stdio JSON-RPC interface. Four categories cover the full lifecycle: a single-shot script runner, a persistent-sandbox lifecycle group (create / exec / upload / download / destroy), two AI agent runners, and three async/job-control tools for background runs. Each tool page documents its parameters (sourced from the Go registration in `internal/server/server.go`), MCP annotations, a sample JSON-RPC request/result, and an error table grounded in `internal/sandbox/`.

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
| [`sandbox_status`](sandbox_status.md) | Get the current status of a background sandbox job. | async/job-control |
| [`sandbox_wait`](sandbox_wait.md) | Block until a background sandbox job reaches a terminal state or the timeout elapses. | async/job-control |
| [`sandbox_cancel`](sandbox_cancel.md) | Cancel a background sandbox job and its entire descendant subtree. | async/job-control |

## Background jobs

Pass `background: true` on `sandbox_script`, `sandbox_agent`, or `sandbox_research` to start a run without blocking the MCP tool-call. The tool returns immediately with `{job_id, status: "running"}`. Use the following tools to manage the job:

- **`sandbox_status`** — non-blocking snapshot: current status, elapsed time, a tail of captured stdout, and cost/exit-code once terminal.
- **`sandbox_wait`** — blocking poll: waits up to `timeout_seconds` (default 30, hard-capped at 120) for the job to reach a terminal state. Returns the final result or `{status: "running", message: "still running; call sandbox_wait again"}` on timeout. Call repeatedly to implement a poll loop.
- **`sandbox_cancel`** — cancels the job and its entire descendant subtree depth-first (child jobs cancelled before parent), then tears down their sandboxes. Idempotent: already-terminal jobs return their final status without error.

**Typical poll idiom:**
1. Call `sandbox_agent` or `sandbox_research` with `background: true` → get `job_id`.
2. Call `sandbox_wait` with `timeout_seconds: 120` in a loop until `status` is not `"running"`.
3. Inspect `result_text`, `output_path`, and `cost_usd` from the final `sandbox_wait` response.

Synchronous calls may run to completion (subject to the explicit 48h runtime limit) and remain cancellable. Background mode automatically attempts exactly one advisory MCP logging notification (`notifications/message`) at a succeeded, failed, or cancelled transition; there is no separate notification opt-in, and synchronous work emits none. Delivery/display is best-effort and does not replace polling: use `sandbox_status` or repeat `sandbox_wait` when the client does not surface notifications or when wake/context-injection behavior is unknown. See [background notification compatibility](../background-notifications.md). `sandbox_wait` remains a 30s-default, 120s-maximum bounded poll. The job registry is in-memory; jobs do NOT survive MCP-server restarts (a stale job_id returns an error after restart); completed jobs are retained ~1h via a TTL reaper.

All three async/job-control tools are dual-registered on the in-sandbox child surface (the `demesne` MCP server available to containerised agents), so child agents can launch and manage their own background sub-jobs.

## Structured output

Every tool returns its result as both a human-readable text payload and a `structuredContent` object validated against a declared [`outputSchema`](https://modelcontextprotocol.io/specification/2025-11-25/server/tools#output-schema). Clients that support structured output — including Claude Code and the Codex CLI — consume the structured object and ignore the text block, which remains as a fallback for clients that don't. Each tool page lists its specific `structuredContent` fields.

## Other reference pages

- [Host requirements](../requirements.md) — container runtime, OpenSandbox configuration, rootless podman pipe-page cap.
- [Configuration](../configuration.md) — environment variables and container image allowlist.
- [Nested sandboxes](../nested-sandboxes.md) — child output layout, `name` rules, `/in/previous-jobs/<name>`, task-prompt structure, copy-to-`/out` gotcha.
- [transcript.jsonl](../transcript-jsonl.md) — NDJSON written to `<output_dir>/transcript.jsonl` by every agent run; event format, `ResultText` extraction, and relationship to the MCP `stdout` field (32 KiB cap).
