# demesne examples

These are runnable examples of the kinds of work you can ask your agent to run through demesne. Each example shows the natural-language request a user would make to their Claude Code/agent, what they get back, and (for reference) the raw JSON-RPC payload the agent ends up issuing to the demesne stdio server.

- [`hello-script/`](hello-script/) — **one-off script**: run a single shell command in a fresh sandbox.
- [`persistent-session/`](persistent-session/) — **persistent session**: create → exec → upload → exec → download → destroy lifecycle.
- [`sandbox-agent-hello/`](sandbox-agent-hello/) — **delegated agent task**: hand a one-shot prompt to a sub-agent.
- [`sandbox-agent-verifier/`](sandbox-agent-verifier/) — **multi-agent orchestration**: worker + verifier pattern.

## Running an example

Make sure demesne is running and the following environment variables are set:

```bash
export OPEN_SANDBOX_DOMAIN=localhost:8080   # your OpenSandbox server
export OPEN_SANDBOX_API_KEY=your-key-here
export DEMESNE_ALLOWED_PATHS=/home/username/demesne-example
```

Then run any example with:

```bash
bash hello-script/run.sh
```

For the agent examples you also need:

```bash
export DEMESNE_CLAUDE_CODE_OAUTH_TOKEN=your-token-here
bash sandbox-agent-hello/run.sh
```

For the production path — running demesne through an MCP client rather than raw stdio — see [Wire demesne into your MCP client](../docs/how-to/wire-into-mcp-client.md).
