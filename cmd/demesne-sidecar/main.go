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
	proxymcp "github.com/jbeshir/demesne/internal/proxies/mcp"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	anth, err := buildAnthropicProxy()
	if err != nil {
		return err
	}
	mcpSrv, err := buildMCPProxy()
	if err != nil {
		return err
	}

	// Both proxies serve under the same shutdown context; the first
	// non-context error wins and cancels the other.
	errCh := make(chan error, 2)
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		log.Printf("starting proxy %q on %s", proxyanthropic.Name, proxyanthropic.BindAddr())
		errCh <- anth.Start(runCtx)
	}()
	go func() {
		errCh <- mcpSrv.Start(runCtx)
	}()

	for range 2 {
		if err := <-errCh; err != nil && !errors.Is(err, context.Canceled) {
			cancel()
			return err
		}
	}
	return nil
}

// buildMCPProxy wires the MCP tunnel from its env vars. An
// unset/empty DEMESNE_MCP_BINDINGS yields a tunnel with no listeners
// that simply blocks until shutdown — the sandbox just has no host
// MCP tools.
func buildMCPProxy() (*proxymcp.Server, error) {
	bindings, err := proxymcp.ParseBindings(os.Getenv(proxymcp.BindingsEnv))
	if err != nil {
		return nil, err
	}
	return proxymcp.NewServer(os.Getenv(proxymcp.SocketPathEnv), bindings), nil
}

// buildAnthropicProxy wires the Anthropic proxy from its env vars.
// Tokens and the usage path are required at the sidecar level.
func buildAnthropicProxy() (*proxyanthropic.ProxyServer, error) {
	auth := os.Getenv(proxyanthropic.AuthTokenEnv)
	if auth == "" {
		return nil, errors.New(proxyanthropic.AuthTokenEnv + " must be set on the anthropic proxy sidecar")
	}
	upstream := os.Getenv(proxyanthropic.UpstreamTokenEnv)
	if upstream == "" {
		return nil, errors.New(proxyanthropic.UpstreamTokenEnv + " must be set on the anthropic proxy sidecar")
	}
	usagePath := os.Getenv(proxyanthropic.UsagePathEnv)
	if err := proxyanthropic.EnsureUsageDir(usagePath); err != nil {
		return nil, err
	}
	tracker := proxyanthropic.NewTracker(usagePath)
	return proxyanthropic.NewProxyServer(proxyanthropic.BindAddr(), auth, upstream, tracker), nil
}
