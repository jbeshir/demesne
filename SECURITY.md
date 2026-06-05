# Security Policy

## Supported Versions

| Version | Supported |
|---------|-----------|
| Latest tagged release (pre-1.0) | :white_check_mark: |

## Reporting a Vulnerability

Please report security vulnerabilities using GitHub's private vulnerability reporting:

**[https://github.com/jbeshir/demesne/security/advisories/new](https://github.com/jbeshir/demesne/security/advisories/new)**

Do not open a public issue. demesne is a single-developer, best-effort project, so there's no guaranteed response time — I'll look into reports and fix what I can as I'm able. If a report is still unaddressed 90 days after you send it, you're free to disclose it publicly.

## Scope

### In scope

- Code in this repository
- The container-edge isolation boundary: an agent escaping its container, accessing host paths outside `DEMESNE_ALLOWED_PATHS`, or otherwise reaching the host beyond what the configured egress permits
- Credentials handling in the per-vendor proxy sidecar: the real upstream OAuth token leaking into the agent container or becoming accessible from within the sandbox
- MCP allowlist enforcement: a containerised agent invoking host MCP tools that are not on the read-only allowlist
- Path containment bypasses: symlink escapes or other mechanisms that allow mounting or accessing host paths outside the allowed set
- Egress policy enforcement: `egress: "none"` or `egress: "package-managers"` failing to restrict outbound traffic as documented

### Out of scope

- Trust between the stdio MCP client and demesne. The MCP client (parent process — distinct from the nested-agent "parent agent" usage in [trust-boundary.md](docs/explanation/trust-boundary.md)) is trusted by design — this is the documented trust boundary.
- Vulnerabilities in podman, OpenSandbox, the kernel, or other upstream dependencies — please report those upstream.
- Container-image CVEs in the four allowlisted images (`node`, `python`, `go`, `anaconda`) — please report those to the upstream image maintainers.
- Single-user-by-design assumptions. Multi-tenant deployment of demesne is not supported and is not in scope for this policy.
- Denial-of-service attacks against a local demesne instance.
- Issues requiring physical access to the host machine.
- Social engineering of the maintainer.

## Good-faith research

Good-faith security research is welcome. demesne is a personal, single-developer project that runs locally — there's no hosted service or third-party data involved. Please don't use a vulnerability as a pretext to access or disrupt anyone else's machine, and give me a reasonable window to fix an issue before disclosing it publicly.

See [docs/explanation/trust-boundary.md](docs/explanation/trust-boundary.md) for the trust-boundary explanation.
