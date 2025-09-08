package main

import (
	"gopin/config"
	"gopin/server"
	"log"
)

func main() {
	cfg, err := config.Load("config.json")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	s := server.New(cfg)
	s.Start()
}
