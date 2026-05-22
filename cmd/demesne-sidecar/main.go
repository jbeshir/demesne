// demesne-sidecar runs the demesne-managed vendor proxy inside a
// per-sandbox sidecar container. The container shares the OpenSandbox
// egress sidecar's network namespace, so the agent reaches the proxy
// on 127.0.0.1:<port> and outbound traffic from the proxy passes
// through OpenSandbox's egress filter as normal. A sandbox runs
// exactly one vendor proxy — pick which one per agent vendor.
//
// All env-var reads happen in this file: proxy packages receive their
// config as explicit constructor arguments and never call os.Getenv
// themselves.
package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"

	proxyanthropic "github.com/jbeshir/demesne/internal/proxies/anthropic"
	proxygo "github.com/jbeshir/demesne/internal/proxies/goproxy"
	proxymcp "github.com/jbeshir/demesne/internal/proxies/mcp"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

// starter is the common Start contract of every sidecar proxy.
type starter interface {
	Start(context.Context) error
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// The Go module proxy runs in every sandbox. The Anthropic proxy
	// and MCP tunnel are agent-mode only; their builders return nil when
	// their config env vars are absent (a plain script/create sandbox).
	starters := []starter{proxygo.NewProxyServer(proxygo.BindAddr())}

	anth, err := buildAnthropicProxy()
	if err != nil {
		return err
	}
	if anth != nil {
		starters = append(starters, anth)
	}
	mcpSrv, err := buildMCPProxy()
	if err != nil {
		return err
	}
	if mcpSrv != nil {
		starters = append(starters, mcpSrv)
	}

	// All proxies serve under one shutdown context; the first
	// non-context error wins and cancels the rest.
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	errCh := make(chan error, len(starters))
	for _, s := range starters {
		go func(s starter) { errCh <- s.Start(runCtx) }(s)
	}
	for range starters {
		if err := <-errCh; err != nil && !errors.Is(err, context.Canceled) {
			cancel()
			return err
		}
	}
	return nil
}

// buildMCPProxy wires the MCP tunnel from its env vars. Returns nil
// when DEMESNE_MCP_BINDINGS is unset/empty (no host MCP tools wired
// in, e.g. a plain script sandbox).
func buildMCPProxy() (*proxymcp.Server, error) {
	raw := os.Getenv(proxymcp.BindingsEnv)
	if raw == "" {
		return nil, nil
	}
	bindings, err := proxymcp.ParseBindings(raw)
	if err != nil {
		return nil, err
	}
	return proxymcp.NewServer(os.Getenv(proxymcp.SocketPathEnv), bindings), nil
}

// buildAnthropicProxy wires the Anthropic proxy from its env vars.
// Returns nil when the auth token is absent (not an agent run); when
// it is present, the upstream token is required too.
func buildAnthropicProxy() (*proxyanthropic.ProxyServer, error) {
	auth := os.Getenv(proxyanthropic.AuthTokenEnv)
	if auth == "" {
		return nil, nil
	}
	upstream := os.Getenv(proxyanthropic.UpstreamTokenEnv)
	if upstream == "" {
		return nil, errors.New(proxyanthropic.UpstreamTokenEnv + " must be set when " + proxyanthropic.AuthTokenEnv + " is")
	}
	usagePath := os.Getenv(proxyanthropic.UsagePathEnv)
	if err := proxyanthropic.EnsureUsageDir(usagePath); err != nil {
		return nil, err
	}
	tracker := proxyanthropic.NewTracker(usagePath)
	return proxyanthropic.NewProxyServer(proxyanthropic.BindAddr(), auth, upstream, tracker), nil
}
