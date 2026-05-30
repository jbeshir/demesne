# Architecture

```mermaid
flowchart TD
    Client["MCP Client<br/>(AI agent)"]
    subgraph Demesne["demesne-mcp"]
        Cmd["cmd/demesne-mcp<br/>main"]
        Server["internal/server<br/>MCP handlers"]
        Sandbox["internal/sandbox<br/>Runner"]
    end
    OS["OpenSandbox<br/>lifecycle server"]
    Docker["Docker<br/>container runtime"]
    Host["Host filesystem<br/>(AllowedPaths, /out root)"]

    Client -->|"JSON-RPC<br/>over stdio"| Cmd
    Cmd --> Server
    Server -->|"RunScript"| Sandbox
    Sandbox -->|"OpenSandbox SDK<br/>HTTP"| OS
    Sandbox -->|"mounts"| Host
    OS --> Docker
```

`cmd/demesne-mcp` loads configuration from the environment, builds a `sandbox.Runner`, and serves MCP over stdio. `internal/server` registers the `sandbox_script` tool and parses arguments, then delegates to the runner. `internal/sandbox` validates mounts, resolves images, builds the network policy, creates the sandbox via the OpenSandbox SDK, runs the command, and tears the sandbox down.

## Data flow

```mermaid
sequenceDiagram
    participant Client as MCP client
    participant Handler as server.handleSandboxScript
    participant Runner as sandbox.Runner
    participant OS as OpenSandbox
    participant SB as Sandbox container

    Client->>Handler: tools/call sandbox_script
    Handler->>Runner: RunScript(req)
    Runner->>Runner: ResolveImage / BuildNetworkPolicy
    Runner->>Runner: validate mounts<br/>(symlink-resolve vs AllowedPaths)
    Runner->>Runner: mkdir OutputRoot/jobID
    Runner->>OS: CreateSandbox(image, volumes, netpol)
    OS->>SB: start container
    Runner->>SB: RunCommandWithOpts(command, cwd=/out)
    SB-->>Runner: exit_code, stdout
    Runner->>OS: Kill (deferred)
    Runner-->>Handler: ScriptResult
    Handler-->>Client: text: exit_code, output_dir, job_id, stdout
```

The deferred `Kill` runs against a fresh `context.Background()` with a 30-second timeout, so the sandbox is torn down even if the request context was cancelled. Commands run with `cwd=/out` and a 12-hour timeout so long-running data jobs aren't capped artificially.
