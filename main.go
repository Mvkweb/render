package main

import (
	"context"
	"gopin/config"
	"gopin/logger"
	"gopin/server"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	log := logger.New()

	cfg, err := config.Load("config.json")
	if err != nil {
		log.Error("FATAL: Failed to load config.json", "error", err)
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	s := server.New(cfg, log)

	// Start the background scraper
	refreshInterval, err := time.ParseDuration(cfg.Scraping.RefreshInterval)
	if err != nil {
		log.Error("FATAL: Invalid refresh interval in config.json", "error", err)
		os.Exit(1)
	}
	s.StartBackgroundScraper(refreshInterval)

	go func() {
		if err := s.Start(); err != nil {
			log.Error("Server failed to start", "error", err)
		}
	}()

	<-ctx.Done()
	s.Shutdown(context.Background())
}
