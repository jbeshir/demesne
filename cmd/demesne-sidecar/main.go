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
	proxyopenai "github.com/jbeshir/demesne/internal/proxies/openai"
	proxycommon "github.com/jbeshir/demesne/internal/proxies/proxycommon"
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

	anth, err := buildCredentialProxy(credProxyEnv{
		authEnv:     proxyanthropic.AuthTokenEnv,
		upstreamEnv: proxyanthropic.UpstreamTokenEnv,
		usageEnv:    proxyanthropic.UsagePathEnv,
		build: func(auth, upstream, usagePath string) starter {
			return proxyanthropic.NewProxyServer(
				proxyanthropic.BindAddr(), auth, upstream, proxyanthropic.NewTracker(usagePath))
		},
	})
	if err != nil {
		return err
	}
	if anth != nil {
		starters = append(starters, anth)
	}
	oai, err := buildCredentialProxy(credProxyEnv{
		authEnv:     proxyopenai.AuthTokenEnv,
		upstreamEnv: proxyopenai.UpstreamKeyEnv,
		usageEnv:    proxyopenai.UsagePathEnv,
		build: func(auth, upstream, usagePath string) starter {
			return proxyopenai.NewProxyServer(
				proxyopenai.BindAddr(), auth, upstream, proxyopenai.NewTracker(usagePath))
		},
	})
	if err != nil {
		return err
	}
	if oai != nil {
		starters = append(starters, oai)
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

// credProxyEnv names the env vars one credential proxy reads and how to
// construct it. build receives the validated values and returns a ready
// starter; it is called only when the auth token is present and the
// upstream credential is set. Keeping the env-var names per-vendor (the
// Anthropic upstream is an OAuth token, the OpenAI one an API key)
// preserves each proxy's distinct contract while sharing the read →
// validate → ensure-dir → construct boilerplate.
type credProxyEnv struct {
	authEnv     string
	upstreamEnv string
	usageEnv    string
	build       func(auth, upstream, usagePath string) starter
}

// buildCredentialProxy wires one agent-vendor credential proxy from its
// env vars. Returns (nil, nil) when the auth token is absent (not that
// vendor's agent run — a plain script/create sandbox, or the other
// vendor); when the auth token is present, the upstream credential is
// required too.
func buildCredentialProxy(c credProxyEnv) (starter, error) {
	auth := os.Getenv(c.authEnv)
	if auth == "" {
		return nil, nil
	}
	upstream := os.Getenv(c.upstreamEnv)
	if upstream == "" {
		return nil, errors.New(c.upstreamEnv + " must be set when " + c.authEnv + " is")
	}
	usagePath := os.Getenv(c.usageEnv)
	if err := proxycommon.EnsureUsageDir(usagePath); err != nil {
		return nil, err
	}
	return c.build(auth, upstream, usagePath), nil
}
