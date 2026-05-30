# demesne examples

These are runnable example MCP calls demonstrating the three main usage patterns for `demesne-mcp`. Each example contains a `README.md` explaining the call, one or more `request.json` files with complete JSON-RPC payloads, and a `run.sh` that pipes those payloads to the demesne stdio server.

1. [`hello-script/`](hello-script/) — single-shot `sandbox_script` with a mounted host file.
2. [`persistent-session/`](persistent-session/) — full create → exec → upload → exec → download → destroy lifecycle.
3. [`sandbox-agent-hello/`](sandbox-agent-hello/) — `sandbox_agent` with token-usage artefacts.

## Running an example

Make sure demesne is running and the following environment variables are set:

```bash
export OPEN_SANDBOX_DOMAIN=localhost:6060   # your OpenSandbox server
export OPEN_SANDBOX_API_KEY=your-key-here
export DEMESNE_ALLOWED_PATHS=/tmp/demesne-example
```

Then run any example with:

```bash
bash hello-script/run.sh
```

For the agent example you also need:

```bash
export DEMESNE_CLAUDE_CODE_OAUTH_TOKEN=your-token-here
bash sandbox-agent-hello/run.sh
```

For the production path — running demesne through an MCP client rather than raw stdio — see the "Wire into Claude Code" section of the main README.
