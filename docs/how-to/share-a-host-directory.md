# Share a host directory with a sandbox

For demesne to mount a host directory into a sandbox, the path must be allowlisted on BOTH sides:

## 1. `DEMESNE_ALLOWED_PATHS` on the demesne process

Colon-separated list of host path prefixes; any candidate path must resolve (after symlink resolution) under one of them. Set it on the demesne process — environment or MCP-client config `env` block:

```bash
export DEMESNE_ALLOWED_PATHS=/home/alice/projects:/tmp/shared-data
```

Or in `.mcp.json`:

```json
{
  "mcpServers": {
    "demesne": {
      "type": "stdio",
      "command": "/usr/local/bin/demesne-mcp",
      "env": {
        "DEMESNE_ALLOWED_PATHS": "/home/alice/projects:/tmp/shared-data",
        "OPEN_SANDBOX_DOMAIN": "localhost:8080",
        "OPEN_SANDBOX_API_KEY": "your-key"
      }
    }
  }
}
```

## 2. OpenSandbox `[storage] allowed_host_paths` in `~/.sandbox.toml`

Must list the same paths:

```toml
[storage]
allowed_host_paths = ["/home/alice/projects", "/tmp/shared-data"]
```

If only one side is configured, every bind mount fails with `VOLUME::HOST_PATH_NOT_ALLOWED`.

## Symlink resolution

Both the candidate path and each allowed-paths entry are symlink-resolved before the containment check. A path that points outside the allowed prefix via a symlink is rejected. An allowed-paths entry that is itself a symlink is resolved to its real path; the candidate is then checked against that real path.

If you see `mount path <path> is not within DEMESNE_ALLOWED_PATHS` but the path looks correct, check for symlinks with `realpath`.

## How agents pass the path in

Agents use the `directories` parameter on tools that accept it — see [`sandbox_script`](../reference/tools/sandbox_script.md) for the per-tool shape. The mount appears at `/in/<basename>` inside the sandbox.
