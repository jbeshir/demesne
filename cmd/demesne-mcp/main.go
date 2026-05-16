package main

import (
	"log"

	// Side-effect imports: register the claude-code agent and the
	// proxies whose egress hosts must end up in the sandbox allowlist.
	_ "github.com/jbeshir/demesne/internal/agents/anthropic"
	_ "github.com/jbeshir/demesne/internal/proxies/anthropic"
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
	runner := sandbox.NewRunner(cfg)
	srv := server.NewServer(runner)
	return srv.Run()
}
