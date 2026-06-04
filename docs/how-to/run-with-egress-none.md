# Run with `egress=none`

## When and why

Use `egress=none` when you want to guarantee that code running inside a sandbox cannot make arbitrary outbound network requests. This is the right choice when:

- You are running untrusted code that should have no access to the internet or internal network services.
- You want to prevent accidental data exfiltration during a build or test step.
- You are running `sandbox_agent` and want to restrict the agent to only its vendor API proxy (this is already the default for `sandbox_agent`).

Note: effective `egress=none` requires your OpenSandbox server to be configured with `[egress] mode = "dns+nft"` in `~/.sandbox.toml`. The default `"dns"` mode only filters at DNS resolution; raw-IP connections still succeed, so `none` is not fully enforced without the `dns+nft` setting. See [docs/reference/requirements.md §OpenSandbox configuration](../reference/requirements.md#opensandbox-configuration) for the required config.

## What still works under `egress=none`

### Go module downloads

Go module downloads work unchanged under `egress=none` because of the sidecar Go-module proxy — see [The four proxies](../explanation/architecture.md#the-four-proxies).

For more detail on how the sidecar bypass is wired, see [Trust boundary, agents, and the per-sandbox sidecar](../explanation/trust-boundary.md).

### Agent vendor API (sandbox_agent)

For `sandbox_agent`, the vendor proxy is always reachable from the sandbox regardless of the `egress` setting: the Anthropic API proxy (`127.0.0.1:8088`) for `agent=claude-code`, or the OpenAI/Codex proxy (`127.0.0.1:8086`) for `agent=codex`. Both run in the sidecar's network namespace and use the same SO_MARK bypass. The `egress` parameter controls what is reachable *in addition to* the vendor proxy, not whether the vendor proxy itself is accessible.

## What breaks under `egress=none`

The following registries and endpoints become unreachable:

- **npm registry** (`registry.npmjs.org`) — `npm install` fails.
- **PyPI** (`pypi.org`, `files.pythonhosted.org`) — `pip install` fails.
- **Conda registries** (`repo.anaconda.com`, `conda.anaconda.org`) — `conda install` fails.
- **Arbitrary `curl` / `wget` / HTTP client calls** — any outbound request to a host not covered by the sidecar bypass is rejected.

If your script needs package manager access, use `egress=package-managers` instead. If it needs unrestricted access, use `sandbox_research` (which combines open egress with no input mounts by design — see [key concepts](../explanation/key-concepts.md)).
