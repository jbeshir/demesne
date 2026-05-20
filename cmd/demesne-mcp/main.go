package main

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"time"

	// Side-effect imports: register the claude-code agent and the
	// proxies whose egress hosts must end up in the sandbox allowlist.
	_ "github.com/jbeshir/demesne/internal/agents/anthropic"
	"github.com/jbeshir/demesne/internal/mcpproxy"
	_ "github.com/jbeshir/demesne/internal/proxies/anthropic"
	_ "github.com/jbeshir/demesne/internal/proxies/mcp"
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

	// The host MCP aggregator is optional: if it can't start (no host
	// config, bad allowlist, …) demesne still serves every other tool,
	// agents just get no host MCP tools. Its bindings/catalogue feed
	// into the runner config; these are not env-derived, so they're
	// populated here in setup rather than in LoadConfigFromEnv.
	agg, err := startAggregator()
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
		cfg.MCPServers = agg.Servers()
		cfg.MCPSocketPath = agg.SocketPath()
		cfg.MCPToolCatalogue = agg.Catalogue()
	}

	runner := sandbox.NewRunner(cfg)
	srv := server.NewServer(runner)
	return srv.Run()
}

// startAggregator builds and starts the host MCP aggregator from
// env-configured paths. Returns (nil, nil) when no host MCP config
// is present so demesne starts cleanly without host tools. All env
// reads happen here in setup, not inside the mcpproxy package.
func startAggregator() (*mcpproxy.Aggregator, error) {
	hostConfig := envOr("DEMESNE_HOST_MCP_CONFIG", defaultHostMCPConfig())
	allowlist := envOr("DEMESNE_MCP_ALLOWLIST", defaultAllowlistPath())

	agg, err := mcpproxy.NewAggregator(mcpproxy.Config{
		HostMCPConfigPath: hostConfig,
		AllowlistFilePath: allowlist,
		SeedAllowlistFile: true,
		// The aggregator listens on a unix socket; the runner
		// bind-mounts it into each sidecar. A socket (not a host TCP
		// port) is what works under rootless podman.
		SocketPath: envOr("DEMESNE_MCP_SOCKET", defaultSocketPath()),
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
// DEMESNE_MCP_SOCKET is unset. Kept under the temp dir so it's a
// short path (unix sockets cap at ~108 chars) and trivially
// bind-mountable into sidecars.
func defaultSocketPath() string {
	return filepath.Join(os.TempDir(), "demesne-mcp", "aggregator.sock")
}

func envOr(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}
