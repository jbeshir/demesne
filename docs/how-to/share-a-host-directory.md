# Share a host directory with a sandbox

When you want to make a directory on your host available as read-only input inside a sandbox, demesne needs two things: the path must be under an allowed host root, and you must pass it as a `directories` parameter when creating the sandbox.

## Steps

### 1. Add the directory to `DEMESNE_ALLOWED_PATHS`

`DEMESNE_ALLOWED_PATHS` is a colon-separated list of host path prefixes. Any path passed in `files` or `directories` must be inside (or equal to) one of these prefixes after symlink resolution. Set it on the demesne process — either in the environment or in your MCP client config's `env` block:

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

Note also that your OpenSandbox server's `~/.sandbox.toml` must list the same paths in `[storage] allowed_host_paths`; if OpenSandbox doesn't allow a path, every bind mount attempt returns `VOLUME::HOST_PATH_NOT_ALLOWED`.

### 2. Pass the directory via the `directories` parameter

All of `sandbox_script`, `sandbox_create`, and `sandbox_agent` accept a `directories` parameter (array of absolute host paths). Demesne mounts each directory read-only at `/in/<basename>` inside the sandbox:

```json
{
  "name": "sandbox_script",
  "arguments": {
    "command": "ls /in/my-data",
    "directories": ["/home/alice/projects/my-data"]
  }
}
```

### 3. Access the directory at `/in/<basename>` inside the sandbox

The directory is mounted at `/in/<basename>` where `<basename>` is the last path component of the host path — so `/home/alice/projects/my-data` appears at `/in/my-data`. The mount is read-only; any writes inside the sandbox to paths under `/in/<basename>` will fail.

```bash
# Inside the sandbox, read the directory:
ls /in/my-data
cat /in/my-data/config.json
```

To work with the files, copy them to `/workspace` or `/out` first:

```bash
cp -r /in/my-data /workspace/my-data
```

## Pitfall: symlink resolution

Both the candidate path (the value in `directories`) and each entry in `DEMESNE_ALLOWED_PATHS` are **symlink-resolved** before the containment check. This means:

- A path that points outside the allowed prefix via a symlink is **rejected**, even if the unresolved path looks like it's inside the prefix.
- An `DEMESNE_ALLOWED_PATHS` entry that is itself a symlink is resolved to the real path; the candidate is then checked against that real path.

If you see `mount path <path> is not within DEMESNE_ALLOWED_PATHS` but the path looks correct, check for symlinks in either the candidate path or the allowed-paths entries using `realpath`.
