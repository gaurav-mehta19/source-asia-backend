// Package main is the entrypoint for the Source Asia backend service.
// It loads configuration, wires all dependencies, and starts the HTTP server.
package main

import (
	"fmt"
	"os"

	"github.com/source-asia/backend/internal/config"
	"github.com/source-asia/backend/internal/logger"
	"github.com/source-asia/backend/internal/server"
)

func main() {
	cfg, err := config.Load(".env")
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}

	log := logger.New(cfg.AppEnv, cfg.LogLevel)

	router := server.NewRouter(cfg, log)
	srv := server.New(cfg, router, log)

	if err := srv.Run(); err != nil {
		log.Error("server exited with error", "error", err.Error())
		os.Exit(1)
	}
}
