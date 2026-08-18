// Package config loads backend configuration from environment variables.
package config

import (
	"fmt"
	"os"
)

type Config struct {
	APIToken    string
	DatabaseURL string
	ARIBaseURL  string
	ARIUser     string
	ARIPassword string
	ARIAppName  string
	HTTPAddr    string
}

// Load reads configuration from the environment. It fails loudly if
// required secrets are unset, rather than silently allowing all requests.
func Load() (Config, error) {
	cfg := Config{
		APIToken:    os.Getenv("API_TOKEN"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		ARIBaseURL:  getenvDefault("ARI_BASE_URL", "http://asterisk:8088"),
		ARIUser:     getenvDefault("ARI_USER", "homephone"),
		ARIPassword: os.Getenv("ARI_PASSWORD"),
		ARIAppName:  getenvDefault("ARI_APP_NAME", "homephone"),
		HTTPAddr:    getenvDefault("HTTP_ADDR", ":8080"),
	}
	if cfg.APIToken == "" {
		return Config{}, fmt.Errorf("API_TOKEN must be set (no insecure default)")
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL must be set")
	}
	if cfg.ARIPassword == "" {
		return Config{}, fmt.Errorf("ARI_PASSWORD must be set (no insecure default)")
	}
	return cfg, nil
}

func getenvDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
