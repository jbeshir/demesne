package main

import (
	"log"

	"github.com/jbeshir/demesne/internal/sandbox"
	"github.com/jbeshir/demesne/internal/server"
)

func main() {
	cfg, err := sandbox.LoadConfigFromEnv()
	if err != nil {
		log.Fatal(err)
	}

	runner := sandbox.NewRunner(cfg)
	srv := server.NewServer(runner)

	if err := srv.Run(); err != nil {
		log.Fatal(err)
	}
}
