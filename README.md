# Demesne

<!-- mcp-name: io.github.jbeshir/demesne-mcp -->

An **agent-agnostic, local, containerised agent-orchestration MCP server you drive from your agent of choice.** It runs untrusted shell, scripts, and AI coding agents in disposable containers, decoupling agent reasoning from execution effects. Host mounts are read-only; outbound network access is governed by egress allowlists.

[![License](https://img.shields.io/github/license/jbeshir/demesne)](LICENSE)
[![Latest Release](https://img.shields.io/github/v/release/jbeshir/demesne)](https://github.com/jbeshir/demesne/releases/latest)
[![CI](https://github.com/jbeshir/demesne/actions/workflows/ci.yml/badge.svg)](https://github.com/jbeshir/demesne/actions/workflows/ci.yml)

> [!WARNING]
> **Alpha — best-effort.** demesne is early software, and is largely built using itself (its own containerised agents do much of the work). Expect rough edges, gaps, and breaking changes between versions. Treat it as alpha and best-effort, and review what it does before relying on it.

## What you can do

Ask your agent to run through demesne:

- **One-off scripts** — execute a shell command in a fresh sandbox and collect output. [Example](examples/hello-script/)
- **Headless React-widget rendering** — render and screenshot a React widget inside a sandbox via the demesne-built `browser` image (Playwright + Chromium + Node 22, works at `egress=none`). [How-to](docs/how-to/render-react-ui.md)
- **Video / audio / image conversion** — transcode, convert, and manipulate media inside a sandbox via the demesne-built `media` image (ffmpeg + ImageMagick + libvips + audio tooling, works at `egress=none`). [How-to](docs/how-to/convert-media.md)
- **Offline interactive-fiction build/playtest** — compile a Twine story and playtest it headlessly via the demesne-built `twine` image (Tweego + Twine story formats + Chromium, works at `egress=none`). [Reference](docs/reference/configuration.md#container-images)
- **Offline HTML5-game build/playtest** — build and playtest a web game via the demesne-built `webgamedev` image (a warm Phaser + Vite + TypeScript template + Chromium, works at `egress=none`). [Reference](docs/reference/configuration.md#container-images)
- **Long-running research with open internet** — spawn a research agent with unrestricted outbound access. [Reference](docs/reference/tools/sandbox_research.md)
- **Delegated coding-agent tasks** — hand off a prompt to a sub-agent running inside a sandbox. [Example](examples/sandbox-agent-hello/)
- **Persistent sessions** — create a sandbox, run multiple commands, upload/download files, then destroy it. [Example](examples/persistent-session/)
- **Multi-agent orchestration** — the orchestrator agent is itself a containerised run that spawns child sandboxes for its workers and verifier, dispatching tasks and judging results across the tree. [Example](examples/sandbox-agent-verifier/)
- **Ready-made orchestration skills** — a library of `SKILL.md` pipeline definitions (migration sweeps, corpus map-reduce, document ETL, and more) to drop into your agent and adapt. Pre-alpha: in principle ready to use but largely untested — regard them as examples of what could be tried. [Example skills](examples/skills/)

Together these let your agent take on larger tasks more autonomously: you can push security-review-awkward script execution, autonomous research, and entire multi-agent pipelines into containers that run with no permission prompts — much of the autonomy you'd otherwise reach for `--dangerously-skip-permissions` to get, but with the host kept at arm's length by a container boundary, read-only mounts, and egress allowlists. (That boundary is container-level isolation, not a hard security guarantee — see [SECURITY.md](SECURITY.md).) And you don't pre-declare the pipeline: your agent composes the orchestration prompt itself for the task at hand, and the containerised orchestrator adapts the layout and subagents as it runs.

## How it works

Containerised agents can themselves spawn sandboxes, and — with appropriate configuration — get a read-only subset of your host's MCP server tools proxied in through a per-sandbox tunnel. See [docs/reference/nested-sandboxes.md](docs/reference/nested-sandboxes.md).

## Get started

### a. Install the binary

Download a release binary from the [GitHub releases page](https://github.com/jbeshir/demesne/releases). Builds are available for `linux/amd64`, `darwin/amd64`, `darwin/arm64`, and `windows/amd64`.

To build from source instead, see [CONTRIBUTING.md](CONTRIBUTING.md).

### b. Run a local OpenSandbox

demesne needs a running OpenSandbox instance. See [docs/reference/requirements.md](docs/reference/requirements.md) for prerequisites; Step 2 of the [Quickstart](docs/tutorial/quickstart.md) walks through launching one locally.

### c. Wire into Claude Code or Codex

See [docs/how-to/wire-into-mcp-client.md](docs/how-to/wire-into-mcp-client.md) for the per-client config snippets and full env var reference.

For the full walkthrough, see [Quickstart](docs/tutorial/quickstart.md).

## Tools

| Tool               | Description                                                                                                                                                                                                                                | Reference |
|--------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----|
| `sandbox_script`   | Run a shell command in a fresh sandbox and tear it down. Returns exit code, stdout, stderr, and the `/out` host path.                                                                                                                     | [ref](docs/reference/tools/sandbox_script.md) |
| `sandbox_create`   | Create a persistent sandbox. Returns a `sandbox_id` handle and the `/out` host path. TTL is 48h, refreshed by each `sandbox_exec`.                                                                                                          | [ref](docs/reference/tools/sandbox_create.md) |
| `sandbox_exec`     | Run a shell command in an existing sandbox. Refreshes TTL. Returns exit code, stdout, and stderr.                                                                                                                                           | [ref](docs/reference/tools/sandbox_exec.md) |
| `sandbox_upload`   | Copy a host file into an existing sandbox.                                                                                                                                                                                                  | [ref](docs/reference/tools/sandbox_upload.md) |
| `sandbox_download` | Copy a file out of an existing sandbox; written under `<output_dir>/downloads/<basename>`. Returns the host path.                                                                                                                           | [ref](docs/reference/tools/sandbox_download.md) |
| `sandbox_destroy`  | Kill an existing sandbox. Host output dir is preserved.                                                                                                                                                                                     | [ref](docs/reference/tools/sandbox_destroy.md) |
| `sandbox_agent`    | Run an AI coding agent in a fresh sandbox against a caller-supplied prompt. Provider is inferred from `model` (defaults to codex/gpt-5.5 when Codex credentials are configured, otherwise claude-code/sonnet). Outbound HTTPS is restricted to the vendor proxy. Returns exit code, stdout, stderr, the `/out` host path, and the (indicative) cost summary. | [ref](docs/reference/tools/sandbox_agent.md) |
| `sandbox_research` | Run a long-running research agent with no input mounts and unrestricted outbound internet access. Returns exit code, stdout, stderr, the `/out` host path, and the (indicative) cost summary.                                              | [ref](docs/reference/tools/sandbox_research.md) |

### Background / async jobs

All three spawn tools (`sandbox_script`, `sandbox_agent`, `sandbox_research`) accept an optional `background: true` parameter. When set, the tool returns immediately with `{job_id, status: "running"}` instead of blocking. Use the complementary job-control tools to manage the run:

| Tool | Description |
|------|-------------|
| `sandbox_status` | Non-blocking snapshot of status, elapsed time, stdout tail, and cost. |
| `sandbox_wait` | Block up to `timeout_seconds` (default 30, max 120) for a terminal state; returns the final result or a `"still running"` sentinel if the timeout elapses. Call in a loop to poll. |
| `sandbox_cancel` | Cancel the job and its entire descendant subtree; idempotent on already-terminal jobs. |

Use `background: true` when a run might exceed the ~240s client tool-call timeout — for example, a multi-hour research agent or a long compilation. The job registry is in-memory; jobs do NOT survive MCP-server restarts (a stale job_id returns an error after restart); completed jobs are retained ~1h via a TTL reaper.

For a step-by-step walkthrough of the persistent-sandbox lifecycle, see the [Quickstart](docs/tutorial/quickstart.md) and the [`sandbox_create`](docs/reference/tools/sandbox_create.md) / [`sandbox_exec`](docs/reference/tools/sandbox_exec.md) reference pages.

## Docs

| | |
|---|---|
| [Quickstart](docs/tutorial/quickstart.md) | Five steps to your first `sandbox_script` call |
| [Docs](docs/) | Tutorials, how-to recipes, reference, explanation |
| [Examples](examples/) | Runnable example calls |
| [Example skills](examples/skills/) | Ready-to-use orchestration pipelines you can adapt (pre-alpha) |

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for building from source, linting, and tests.

## Status

See [CHANGELOG.md](CHANGELOG.md) for milestone history.
