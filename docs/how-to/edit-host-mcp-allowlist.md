# Edit the host MCP allowlist

Demesne re-exposes a curated subset of your host's stdio MCP servers to containerised agents. The allowlist file controls which tools from each server are exposed. By default, demesne seeds a conservative read-only allowlist for every known server.

## The allowlist file

The allowlist file lives at:

```
~/.config/demesne/mcp-allowlist.json
```

This path is the default value of the `DEMESNE_MCP_ALLOWLIST` environment variable. You can override it:

```bash
export DEMESNE_MCP_ALLOWLIST=/etc/demesne/mcp-allowlist.json
```

If the file does not exist when demesne starts, demesne creates it automatically (via `SeedOverrideFile` in `internal/mcpproxy/allowlist.go`) with a `"default"` entry for every server that has built-in defaults. You can then edit it.

## File format

The file is a JSON object. Each key is an MCP server name (matching the name in your Claude Code or Codex config, e.g. `"workflowy"`, `"alignment"`). The value is one of:

| Value | Meaning |
|-------|---------|
| `"default"` | Use demesne's built-in read-only tool set for this server (the default for known servers). |
| `"*"` | Expose every tool the upstream server advertises at runtime (no filtering). |
| `["tool-name-1", "tool-name-2"]` | Expose only the listed tools. |
| `[]` | Disable this server entirely — it will not appear in the agent's tool list. |

Keys beginning with `_` (e.g. `"_doc"`) are metadata and are ignored by the parser.

### Example

```json
{
  "_doc": "Per-server tool allowlist. Value is one of: \"default\", \"*\", a list of tool names, or [] to disable.",
  "workflowy": "default",
  "alignment": ["search_articles", "get_article", "semantic_search"],
  "anki": [],
  "my-custom-server": "*"
}
```

In this example:
- `workflowy` uses demesne's built-in read-only defaults.
- `alignment` exposes only three specific tools.
- `anki` is disabled — the agent cannot see or call any Anki tools.
- `my-custom-server` exposes every tool it advertises.

## When changes take effect

Demesne reads the allowlist file **once at startup** (via `ResolveAllowlist` in `internal/mcpproxy/allowlist.go`). Changes to the file take effect only after you restart the demesne process. If demesne is managed by your MCP client (e.g. Claude Code spawns it), restart the MCP server connection (in Claude Code: `/mcp` → disconnect and reconnect, or restart Claude Code).

## Notes

- The allowlist applies to `tools/list` filtering at the aggregator level. A tool that is not on the allowlist never appears in the agent's tool list and cannot be called — the agent doesn't even know it exists.
- There is no per-tool argument filtering; the allowlist is coarse-grained (tool-level, not argument-level).
- The demesne self-server (which exposes `sandbox_script`, `sandbox_agent`, etc. to child agents) is not governed by this allowlist file — it is wired in directly by the runner.
- Resources, resource templates, prompts, and completion bypass the allowlist — setting a server to `[]` removes its tools but the server's resources/prompts are still exposed.
