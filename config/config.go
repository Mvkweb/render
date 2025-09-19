package config

import (
	"encoding/json"
	"os"
)

// ScrapingConfig holds the configuration for the scraping process.
type ScrapingConfig struct {
	MinDelay        string   `json:"minDelay"`
	MaxDelay        string   `json:"maxDelay"`
	PoolSize        int      `json:"poolSize"`
	RefreshInterval string   `json:"refreshInterval"`
	Queries         []string `json:"queries"`
	UserAgents      []string `json:"userAgents"`
}

// DatabaseConfig holds the configuration for the database.
type DatabaseConfig struct {
	CleanupInterval string `json:"cleanupInterval"`
	MaxAge          string `json:"maxAge"`
}

// Config holds the application's configuration.
type Config struct {
	Port        string            `json:"port"`
	Credentials map[string]string `json:"credentials"`
	NumWorkers  int               `json:"numWorkers"`
	Scraping    ScrapingConfig    `json:"scraping"`
	Database    DatabaseConfig    `json:"database"`
}

// Load loads the configuration from a file.
func Load(path string) (*Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	cfg := &Config{}
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}
