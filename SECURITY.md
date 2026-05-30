# Security Policy

## Supported Versions

| Version | Supported |
|---------|-----------|
| Latest tagged release (pre-1.0) | :white_check_mark: |

## Reporting a Vulnerability

Please report security vulnerabilities using GitHub's private vulnerability reporting:

**[https://github.com/jbeshir/demesne/security/advisories/new](https://github.com/jbeshir/demesne/security/advisories/new)**

Do not open a public issue. Your issue will be fixed or made public within 90 days.

## Scope

### In scope

- Code in this repository
- The sandbox-edge isolation boundary: a sandboxed agent escaping its container, accessing host paths outside `DEMESNE_ALLOWED_PATHS`, or otherwise reaching the host beyond what the configured egress permits
- Credentials handling in the per-vendor proxy sidecar: the real upstream OAuth token leaking into the agent container or becoming accessible from within the sandbox
- MCP allowlist enforcement: a sandboxed agent invoking host MCP tools that are not on the read-only allowlist
- Path containment bypasses: symlink escapes or other mechanisms that allow mounting or accessing host paths outside the allowed set
- Egress policy enforcement: `egress: "none"` or `egress: "package-managers"` failing to restrict outbound traffic as documented

### Out of scope

- Trust between the stdio MCP client and demesne. The parent process is trusted by design — this is the documented trust boundary (see [docs/explanation/trust-boundary.md](docs/explanation/trust-boundary.md)).
- Vulnerabilities in podman, OpenSandbox, the kernel, or other upstream dependencies — please report those upstream.
- Container-image CVEs in the four whitelisted images (`node`, `python`, `go`, `anaconda`) — please report those to the upstream image maintainers.
- Single-user-by-design assumptions. Multi-tenant deployment of demesne is not supported and is not in scope for this policy.
- Denial-of-service attacks against a local demesne instance.
- Issues requiring physical access to the host machine.
- Social engineering of maintainers.

## Safe Harbor

Security research conducted in good faith is welcomed. We will not pursue legal action against researchers who discover and report vulnerabilities in accordance with this policy, provided they do not access or modify data belonging to others, disrupt production systems, or publicly disclose before the coordinated disclosure window has passed.

See [docs/explanation/trust-boundary.md](docs/explanation/trust-boundary.md) for the trust-boundary explanation.
