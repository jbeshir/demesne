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

	log.Printf("starting proxy %q on %s", proxyanthropic.Name, proxyanthropic.BindAddr())
	if err := anth.Start(ctx); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	return nil
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
