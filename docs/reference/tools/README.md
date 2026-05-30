# Tool reference

Demesne exposes eight MCP tools over its stdio JSON-RPC interface. Three categories cover the full lifecycle: a single-shot script runner, a persistent-sandbox lifecycle group (create / exec / upload / download / destroy), and two AI agent runners. Each tool page documents its parameters (sourced from the Go registration in `internal/server/server.go`), MCP annotations, a sample JSON-RPC request/result, and an error table grounded in `internal/sandbox/`.

| Tool | Summary | Category |
|------|---------|----------|
| [`sandbox_script`](sandbox_script.md) | Run a single shell command in a fresh sandbox and return its stdout. | single-shot |
| [`sandbox_create`](sandbox_create.md) | Create a persistent sandbox and return its handle. | persistent |
| [`sandbox_exec`](sandbox_exec.md) | Run a shell command in an existing sandbox. | persistent |
| [`sandbox_upload`](sandbox_upload.md) | Copy a host file into an existing sandbox. | persistent |
| [`sandbox_download`](sandbox_download.md) | Copy a file out of an existing sandbox to the host. | persistent |
| [`sandbox_destroy`](sandbox_destroy.md) | Destroy an existing sandbox. | persistent |
| [`sandbox_agent`](sandbox_agent.md) | Run an AI agent inside a fresh sandbox against the caller's prompt. | agent |
| [`sandbox_research`](sandbox_research.md) | Run a long-running research agent in a fresh sandbox with unrestricted outbound internet access. | agent |
