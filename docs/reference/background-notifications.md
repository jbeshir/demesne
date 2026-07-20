# Background completion notifications

With `background: true`, Demesne automatically attempts one advisory MCP logging notification when the job first reaches `succeeded`, `failed`, or `cancelled`. The JSON-RPC method is `notifications/message`; `params.level` is `info`, `params.logger` is `demesne.background-job`, and `params.data` contains `job_id`, `status`, and a polling reminder. Synchronous calls do not register this notification. There is no separate subscription or opt-in parameter.

The notification is deliberately advisory. A disconnected client, closed writer, unsupported logging session, or filtered log level can prevent delivery or display without changing the terminal job state. Continue to use `sandbox_status`, or call `sandbox_wait` repeatedly, to obtain a reliable terminal result.

## Evidence and compatibility

| Component | Verified local evidence | What is not established |
|---|---|---|
| MCP protocol types shipped by mcp-go v0.50.0 | `mcp/types.go` defines `LoggingMessageNotification` as a server notification and says a server may choose messages automatically when no `logging/setLevel` request was sent. `mcp/utils.go` constructs it with method `notifications/message`. | The protocol notification itself does not promise UI display, waking an idle agent, or injecting content into a model turn. |
| mcp-go v0.50.0 server | `server/session.go` provides `SendLogMessageToClient`; its stdio session notification channel uses JSON-RPC notifications, applies the negotiated logging level, and reports unavailable/uninitialized sessions as errors. Upstream server tests cover active, unavailable, and blocked sessions. | Successful queueing is not proof that a particular MCP client presents the message to a user or model. |
| Generic mcp-go client | The pinned module's client documentation/tests register handlers for `notifications/message`. This verifies that mcp-go clients can consume the standard notification when configured. | It does not establish behavior of other clients. |
| Codex | Repository and installed local Codex documentation/config evidence verifies stdio MCP connection/configuration only. No local primary documentation or test was found proving terminal logging notifications wake Codex, appear in its UI, or are injected into a later model turn. | Wake, display, and injection are **unverified**; no support claim is made. |
| Claude Code | Repository and installed local Claude Code documentation/config evidence verifies stdio MCP connection/configuration only. No local primary documentation or test was found proving terminal logging notifications wake Claude Code, appear in its UI, or are injected into a later model turn. | Wake, display, and injection are **unverified**; no support claim is made. |

The inference is therefore narrow: standards-compliant clients may consume or display the logging notification, but polling is required whenever that behavior is absent or unknown.
