package main

import (
	"context"
	"gopin/config"
	"gopin/pkg/console"
	"gopin/pkg/logger"
	"gopin/pkg/network"
	"gopin/server"
	"os"
	"os/signal"
	"syscall"
)

const version = "1.0.0"

func main() {
	log := logger.New()

	cfg, err := config.Load("config.json")
	if err != nil {
		log.Error("FATAL: Failed to load config.json", "error", err)
		os.Exit(1)
	}

	console.PrintBanner(version, network.GetLocalIP(), cfg.Port)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	s := server.New(cfg, log)

	go func() {
		if err := s.Start(); err != nil {
			log.Error("Server failed to start", "error", err)
		}
	}()

	<-ctx.Done()
	s.Shutdown(context.Background())
}
