// Package proxies defines the registry of host-side proxies that the
// per-sandbox sidecar runs. Each upstream-specific subpackage (Anthropic
// for the Claude API, future ones for MCP, Go modules, etc.) registers
// itself from init.
//
// The demesne-mcp binary blank-imports each proxy package so its
// EgressHosts contribute to the per-sandbox egress allowlist. The
// demesne-sidecar binary blank-imports the same packages and runs all
// registered proxies inside the sandbox network namespace.
package proxies

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"syscall"
)

// BypassDialerControl is a net.Dialer.Control hook every proxy uses on
// its outbound sockets. It applies SO_MARK so the OpenSandbox egress
// sidecar accepts the packet unconditionally (see sockmark_linux.go).
// Wire it into both the http.Transport's Dialer and any custom DNS
// Resolver's Dialer so both connection and resolution traffic are
// marked.
func BypassDialerControl(_, _ string, c syscall.RawConn) error {
	var setErr error
	if cerr := c.Control(func(fd uintptr) { setErr = setBypassMark(fd) }); cerr != nil {
		return cerr
	}
	return setErr
}

// Proxy is the contract every demesne proxy implements. Implementations
// must be safe for concurrent invocations of Run/Shutdown across
// goroutines — the sidecar starts all proxies in parallel.
type Proxy interface {
	// Name is a short identifier used in logs and metrics ("anthropic",
	// "mcp", "go-mod"). Must be unique within the registry.
	Name() string

	// EgressHosts are the hostnames the proxy needs to reach upstream.
	// The runner adds these to the sandbox's egress allowlist; the agent
	// itself only talks to 127.0.0.1, so it never appears in egress
	// traffic directly.
	EgressHosts() []string

	// ListenAddr is the 127.0.0.1:<port> the proxy binds inside the
	// sidecar. The agent reaches the proxy at this address.
	ListenAddr() string

	// Run serves until ctx is cancelled. Returns nil on clean shutdown.
	Run(ctx context.Context) error
}

var (
	registryMu sync.RWMutex
	registry   = map[string]Proxy{}
)

// Register adds a proxy to the global registry. Intended for use from a
// proxy subpackage's init function.
func Register(p Proxy) {
	registryMu.Lock()
	defer registryMu.Unlock()
	name := p.Name()
	if _, exists := registry[name]; exists {
		panic(fmt.Sprintf("proxies: %q already registered", name))
	}
	registry[name] = p
}

// All returns every registered proxy, sorted by Name for stable
// iteration order.
func All() []Proxy {
	registryMu.RLock()
	defer registryMu.RUnlock()
	out := make([]Proxy, 0, len(registry))
	for _, p := range registry {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name() < out[j].Name() })
	return out
}

// EgressHosts returns the union of every registered proxy's
// EgressHosts. The sandbox runner adds these to the egress allowlist
// alongside the caller's chosen mode.
func EgressHosts() []string {
	seen := map[string]struct{}{}
	var hosts []string
	for _, p := range All() {
		for _, h := range p.EgressHosts() {
			if _, ok := seen[h]; ok {
				continue
			}
			seen[h] = struct{}{}
			hosts = append(hosts, h)
		}
	}
	return hosts
}
