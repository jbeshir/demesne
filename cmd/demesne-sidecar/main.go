// demesne-sidecar runs all demesne-managed proxies inside a
// per-sandbox sidecar container. The container shares the OpenSandbox
// egress sidecar's network namespace, so the agent reaches every proxy
// on 127.0.0.1:<port> and outbound traffic from each proxy passes
// through OpenSandbox's egress filter as normal.
//
// Proxies register themselves via internal/proxies; this binary
// blank-imports each provider's proxy package and runs every registered
// proxy concurrently.
package main

import (
	"context"
	"errors"
	"log"
	"os/signal"
	"syscall"

	"github.com/jbeshir/demesne/internal/proxies"

	// Side-effect imports: register one proxy per package.
	_ "github.com/jbeshir/demesne/internal/proxies/anthropic"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	registered := proxies.All()
	if len(registered) == 0 {
		return errors.New("no proxies registered")
	}

	errCh := make(chan error, len(registered))
	for _, p := range registered {
		p := p
		log.Printf("starting proxy %q on %s", p.Name(), p.ListenAddr())
		go func() { errCh <- p.Run(ctx) }()
	}

	var firstErr error
	for range registered {
		if err := <-errCh; err != nil && !errors.Is(err, context.Canceled) && firstErr == nil {
			firstErr = err
			stop()
		}
	}
	return firstErr
}
