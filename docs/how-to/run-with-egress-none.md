# Run with `egress=none`

## When and why

Use `egress=none` when you want to guarantee that code running inside a sandbox cannot make arbitrary outbound network requests. This is the right choice when:

- You are running untrusted code that should have no access to the internet or internal network services.
- You want to prevent accidental data exfiltration during a build or test step.
- You are running `sandbox_agent` and want to restrict the agent to only its vendor API proxy (this is already the default for `sandbox_agent`).

Note: effective `egress=none` requires your OpenSandbox server to be configured with `[egress] mode = "dns+nft"` in `~/.sandbox.toml`. The default `"dns"` mode only filters at DNS resolution; raw-IP connections still succeed, so `none` is not fully enforced without the `dns+nft` setting. See [Run a local OpenSandbox](../../README.md#run-a-local-opensandbox) for the required config.

## What still works under `egress=none`

### Go module downloads

Every sandbox has `GOPROXY=http://127.0.0.1:8087` injected into its environment (set by `sandboxEnv` in `internal/sandbox/agent.go`). The per-sandbox sidecar runs a Go module proxy on that port (`internal/proxies/goproxy/proxy.go`), which forwards requests to `proxy.golang.org` via an `SO_MARK`-based egress bypass that is not subject to the sandbox's egress policy. This means `go get`, `go mod download`, and `go build` (which fetches missing modules) all work in `image=go` sandboxes even with `egress=none`.

The Go checksum database (`sum.golang.org`) is the one external host that demesne adds to every sandbox's egress allowlist, because `go` contacts the sumdb directly (not through the GOPROXY URL). Module hash verification therefore works unchanged.

For more detail on how the sidecar bypass is wired, see [Trust boundary, agents, and the per-sandbox sidecar](../explanation/trust-boundary.md).

### Agent vendor API (sandbox_agent)

For `sandbox_agent`, the Anthropic API proxy (`127.0.0.1:8088`) is always reachable from the sandbox regardless of the `egress` setting, because it runs in the sidecar's network namespace and uses the same SO_MARK bypass. The `egress` parameter for `sandbox_agent` controls what is reachable *in addition to* the vendor proxy, not whether the vendor proxy itself is accessible.

## What breaks under `egress=none`

The following registries and endpoints become unreachable:

- **npm registry** (`registry.npmjs.org`) — `npm install` fails.
- **PyPI** (`pypi.org`, `files.pythonhosted.org`) — `pip install` fails.
- **Conda registries** (`repo.anaconda.com`, `conda.anaconda.org`) — `conda install` fails.
- **Arbitrary `curl` / `wget` / HTTP client calls** — any outbound request to a host not covered by the sidecar bypass is rejected.

If your script needs package manager access, use `egress=package-managers` instead. If it needs unrestricted access, use `sandbox_research` (which combines open egress with no input mounts by design — see [key concepts](../explanation/key-concepts.md)).
