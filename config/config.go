package config

import (
	"encoding/json"
	"os"
)

// Config holds the application's configuration.
type Config struct {
	Port        string            `json:"port"`
	Credentials map[string]string `json:"credentials"`
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
