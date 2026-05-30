package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	// Side-effect imports: register the claude-code agent and the
	// proxies whose egress hosts must end up in the sandbox allowlist.
	_ "github.com/jbeshir/demesne/internal/agents/anthropic"
	_ "github.com/jbeshir/demesne/internal/agents/codex"
	"github.com/jbeshir/demesne/internal/mcpproxy"
	_ "github.com/jbeshir/demesne/internal/proxies/anthropic"
	_ "github.com/jbeshir/demesne/internal/proxies/goproxy"
	_ "github.com/jbeshir/demesne/internal/proxies/mcp"
	_ "github.com/jbeshir/demesne/internal/proxies/openai"
	"github.com/jbeshir/demesne/internal/sandbox"
	"github.com/jbeshir/demesne/internal/server"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	cfg, err := sandbox.LoadConfigFromEnv()
	if err != nil {
		return err
	}

	owner, err := sandbox.ComputeOwner()
	if err != nil {
		return fmt.Errorf("compute owner identity: %w", err)
	}
	cfg.Owner = owner

	// Reap orphaned sandboxes from previous instances before serving.
	// Best-effort: errors are logged but never block startup.
	reapCtx, reapCancel := context.WithTimeout(context.Background(), 30*time.Second)
	reaped, reapErrs := sandbox.ReapOrphans(reapCtx, cfg)
	reapCancel()
	for _, e := range reapErrs {
		log.Printf("demesne: orphan reaper: %v", e)
	}
	if reaped > 0 {
		log.Printf("demesne: reaped %d orphaned sandbox(es)", reaped)
	}

	// The runner must exist before the aggregator: the aggregator
	// mounts the runner's own demesne MCP server (the in-sandbox
	// child-spawning tools) as an extra server.
	runner := sandbox.NewRunner(cfg)
	demesneName, demesneTools, demesneHandler := runner.ChildMCPServer()

	// Each demesne-mcp instance (one per Claude Code session) gets its own
	// aggregator socket. The default path is per-PID; binding removes any
	// pre-existing socket at the path first, so a shared default would let a
	// newly-started instance unlink a running instance's live socket and
	// silently break its in-sandbox child spawning. Clean up our own socket
	// directory on exit (only when using the per-instance default).
	socketPath := envOr("DEMESNE_MCP_SOCKET", defaultSocketPath())
	if os.Getenv("DEMESNE_MCP_SOCKET") == "" {
		defer func() { _ = os.RemoveAll(filepath.Dir(socketPath)) }()
	}

	// The host MCP aggregator is optional: if it can't start (no host
	// config, bad allowlist, …) demesne still serves every other tool,
	// agents just get no host MCP tools. Its bindings/catalogue feed
	// back into the runner via SetMCPWiring; these are not env-derived,
	// so they're populated here in setup rather than in LoadConfigFromEnv.
	agg, err := startAggregator(socketPath, mcpproxy.ExtraServer{
		Name:    demesneName,
		Tools:   demesneTools,
		Handler: demesneHandler,
	})
	if err != nil {
		log.Printf("demesne: host MCP proxy disabled: %v", err)
	} else if agg != nil {
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := agg.Shutdown(ctx); err != nil {
				log.Printf("demesne: aggregator shutdown: %v", err)
			}
		}()
		runner.SetMCPWiring(agg.Servers(), agg.SocketPath(), agg.Catalogue())
	}

	// Register signal disposition before creating the server so that
	// SIGHUP (reload/graceful-drain), SIGTERM, and SIGINT all cancel the
	// context passed to RunContext. The defers above (aggregator shutdown,
	// socket-dir cleanup) were registered first, so they unwind AFTER
	// stop() restores default signal handlers.
	ctx, stop := signal.NotifyContext(context.Background(),
		syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	defer stop()

	srv := server.NewServer(runner)
	return srv.RunContext(ctx)
}

// startAggregator builds and starts the host MCP aggregator from
// env-configured paths, mounting the given extra in-process servers
// (demesne's own child-spawning tools) alongside the discovered stdio
// upstreams. The demesne server alone is enough to bring the
// aggregator up even when no host stdio servers are configured. All
// env reads happen here in setup, not inside the mcpproxy package.
func startAggregator(socketPath string, extra ...mcpproxy.ExtraServer) (*mcpproxy.Aggregator, error) {
	hostConfig := envOr("DEMESNE_HOST_MCP_CONFIG", defaultHostMCPConfig())
	allowlist := envOr("DEMESNE_MCP_ALLOWLIST", defaultAllowlistPath())

	agg, err := mcpproxy.NewAggregator(mcpproxy.Config{
		HostMCPConfigPath: hostConfig,
		AllowlistFilePath: allowlist,
		SeedAllowlistFile: true,
		// The aggregator listens on a unix socket; the runner
		// bind-mounts it into each sidecar. A socket (not a host TCP
		// port) is what works under rootless podman.
		SocketPath:   socketPath,
		ExtraServers: extra,
	})
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := agg.Start(ctx); err != nil {
		return nil, err
	}
	return agg, nil
}

func defaultHostMCPConfig() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".claude.json")
}

func defaultAllowlistPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "demesne", "mcp-allowlist.json")
}

// defaultSocketPath is where the aggregator's unix socket lives when
// DEMESNE_MCP_SOCKET is unset. The PID makes it unique per running
// instance so concurrent demesne-mcp processes (one per Claude Code
// session) never share — or unlink — each other's socket. Kept under
// the temp dir so it's a short path (unix sockets cap at ~108 chars)
// and trivially bind-mountable into sidecars.
func defaultSocketPath() string {
	return filepath.Join(os.TempDir(), "demesne-mcp", strconv.Itoa(os.Getpid()), "aggregator.sock")
}

func envOr(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}
