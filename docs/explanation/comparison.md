# Comparison: demesne vs alternatives

Demesne is a **single-user, local-first** MCP server that wraps a local OpenSandbox instance. It
does not offer multi-tenant hosting, a managed cloud plane, or a provisioning API. Each invocation
of `demesne-mcp` serves exactly one MCP client (typically one Claude Code session). Sandboxes are
disposable containers on the operator's own machine; there is no remote fleet.

This framing matters for the comparisons below. Most peers are cloud-hosted, multi-tenant products
with REST APIs and billed compute. Demesne competes on trust-boundary quality and ease of wiring
into a local agent session, not on scale or managed infrastructure.

---

## E2B

E2B is a hosted sandbox runtime purpose-built for AI agents, with SDKs in Python and TypeScript.

**Where demesne wins**

- No cloud account or billing relationship required — the sandbox runs on your own machine.
- The MCP interface is native: no SDK layer, no language runtime, no HTTP round-trip to a remote
  data centre. The tool call stays in-process.
- Read-only host directory mounts via `DEMESNE_ALLOWED_PATHS` — a pattern E2B's SDK does not
  expose as a first-class primitive.
- Host MCP proxy: existing MCP servers from your Claude Code config are re-exposed read-only to
  sandboxed agents — no equivalent documented in E2B.

**Where E2B wins**

- Managed cloud hosting: no local Docker/Podman daemon needed; sandboxes run on E2B's
  infrastructure.
- Multi-language SDKs (Python, TypeScript) and a REST API usable from any language or CI pipeline.
- Enterprise adoption: large production usage and a documented error-class reference
  (`TimeoutError`, `SandboxFilesystemError`, etc.).
- Animated demo and product page — lower onboarding friction for first-time evaluators.

**Where they don't compete**

E2B is a cloud SaaS product; demesne is a local tool. If you need sandboxes from CI, from a
server, or from multiple concurrent sessions, E2B is the appropriate choice. If you need the
sandbox to run on your own hardware with no external dependency, demesne is.

---

## Modal

Modal is a cloud AI-infrastructure platform with a Python SDK for running inference, training,
batch jobs, and sandboxes.

**Where demesne wins**

- No Python dependency or Modal account. A single static Go binary (`demesne-mcp`) is the entire
  server.
- Direct MCP integration: demesne speaks JSON-RPC over stdio to any MCP client out of the box;
  Modal Sandboxes require a Python SDK layer.
- Credential isolation: demesne's per-sandbox Anthropic proxy means the agent never sees the real
  API key — it only talks to 127.0.0.1.

**Where Modal wins**

- Sub-second cold starts, autoscaling, and GPU access — demesne relies on the local container
  runtime and has no autoscaling.
- Well-structured reference docs with typed Python signatures, `Raises` blocks, and inline example
  responses per method.
- Broader platform: inference, training, and scheduled jobs beyond sandboxing.

**Where they don't compete**

Modal is designed for cloud-hosted, billed compute at scale. Demesne is designed for a single
developer who wants a local trust boundary for one agent session. They serve different operating
models.

---

## Daytona

Daytona is the closest structural peer: it positions itself as "secure and elastic infrastructure
for running AI-generated code" with a local-first option and a dedicated docs site.

**Where demesne wins**

- MCP-native interface: the demesne server is a single binary that plugs into any MCP client
  without an SDK or daemon process.
- Nested agent spawning: `sandbox_agent` and `sandbox_research` let an in-session agent spawn child
  sandboxes, propagating inputs and workspace automatically — a pattern not documented in Daytona.
- No account or network dependency for the sandbox runtime itself (Daytona's cloud offering
  requires account setup).

**Where Daytona wins**

- Multi-language SDKs (Python, TypeScript, Go) and a REST API with OpenAPI specs.
- Audience-split docs: "Agent Tools" and "Human Tools" as separate entry points with ⌘K search.
- Broader runtime features: Git integration, file system API, process API, and observability
  sections.

**Where they don't compete**

Daytona's primary product is a developer-environment runtime with full lifecycle management.
Demesne's scope is narrower: a hardened MCP sandbox boundary for one local agent session.

---

## Runloop

Runloop reached GA in May 2025. It provides disposable "devboxes" (VM-based, not container-based),
credential proxying via "Agent Gateways," and MCP tool routing via "MCP Hub".

**Where demesne wins**

- Single static binary, no cloud account, no VM overhead — demesne's container-based sandboxes
  start faster for typical shell/script tasks.
- The MCP interface is the primary API surface, not a secondary integration.
- Open-source and self-hostable; Runloop is a commercial product.

**Where Runloop wins**

- VM-level isolation (stronger than container isolation) via Runloop's devbox technology.
- Agent Gateways: credential proxy so the sandbox never sees real API keys at the VM level.
- MCP Hub for routing tool-server traffic without exposing real credentials.
- Enterprise-grade sandboxes with documented security guarantees for production agent deployments.

**Where they don't compete**

Runloop is a hosted commercial product aimed at enterprise agent deployments. Demesne is a
local-first open-source tool for individual developers. They have similar architectural ideas
(credential proxy, MCP integration, disposable containers) but operate at very different scales
and trust models.

---

## Cloudflare Sandbox SDK

Cloudflare Sandbox SDK went GA on 2026-04-13. It provides isolated container sandboxes with
per-sandbox egress policies, zero-trust credential injection, and a dedicated "Set up Claude
Managed Agents" tutorial.

**Where demesne wins**

- No Cloudflare account or Workers plan needed — demesne runs entirely on the local machine.
- The MCP interface is the primary integration surface; Cloudflare Sandbox SDK exposes a Workers
  API, not an MCP stdio server.
- Nested agent spawning with shared `/workspace` and `results.json` cost roll-up — features not
  documented in the Cloudflare SDK.

**Where Cloudflare Sandbox SDK wins**

- Per-sandbox egress policies with zero-trust credential injection and TLS interception at network
  level.
- Global edge infrastructure: sandboxes run close to Cloudflare's edge, with live preview URLs.
- Official "Set up Claude Managed Agents" tutorial with a documented integration pattern for
  Claude Managed Agents.
- Cloudflare's production security posture and compliance certifications.

**Where they don't compete**

Cloudflare Sandbox SDK requires a Cloudflare Workers deployment and is built for cloud-hosted
agent workflows. Demesne is for a local developer session with no cloud dependency. Their egress
control designs are conceptually similar but implemented at different layers (Workers egress policy
vs. container network namespace + nftables).

---

## Summary table

| | demesne | E2B | Modal | Daytona | Runloop | Cloudflare Sandbox SDK |
|---|---|---|---|---|---|---|
| Deployment | Local binary | Cloud SaaS | Cloud SaaS | Local + cloud | Cloud (commercial) | Cloudflare Workers |
| Isolation | Containers (OpenSandbox) | Containers (hosted) | Containers (hosted) | Containers / VMs | VMs | Containers (edge) |
| MCP interface | Native stdio | Via SDK | Via Python SDK | Via SDK | Via MCP Hub | Via Workers API |
| Credential proxy | Yes (per-vendor API proxy) | Not documented | Not documented | Not documented | Yes (Agent Gateways) | Yes (zero-trust injection) |
| Nested agent spawning | Yes | Not documented | Not documented | Not documented | Not documented | Not documented |
| Host directory mounts | Yes (DEMESNE_ALLOWED_PATHS) | Not documented | Not documented | Yes | Not documented | Not documented |
| Open source | Yes (MIT) | Yes (Apache-2.0) | Partial (client SDK) | Yes (Apache-2.0) | No | No |
| Scale | Single user, local | Multi-tenant, cloud | Multi-tenant, cloud | Local + cloud | Enterprise, cloud | Edge, cloud |
