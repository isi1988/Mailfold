// Package config loads Mailfold runtime configuration from the environment.
package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds runtime configuration, populated from environment variables.
type Config struct {
	Addr               string // HTTP listen address, e.g. ":8080".
	MailcowBaseURL     string // Base URL of the mailcow instance.
	MailcowAPIKey      string // mailcow API key (X-API-Key).
	MailcowInsecureTLS bool   // Skip TLS verification (development only).
	FrontendDir        string // Path to the built frontend SPA (optional).
}

// Load reads configuration from the environment and applies defaults.
// It returns an error if a required value is missing.
func Load() (*Config, error) {
	cfg := &Config{
		Addr:               getenv("MAILFOLD_ADDR", ":8080"),
		MailcowBaseURL:     getenv("MAILFOLD_MAILCOW_URL", ""),
		MailcowAPIKey:      os.Getenv("MAILFOLD_MAILCOW_API_KEY"),
		MailcowInsecureTLS: getbool("MAILFOLD_MAILCOW_INSECURE_TLS", false),
		FrontendDir:        getenv("MAILFOLD_FRONTEND_DIR", "./frontend/dist"),
	}

	if cfg.MailcowBaseURL == "" {
		return nil, fmt.Errorf("MAILFOLD_MAILCOW_URL is required")
	}
	return cfg, nil
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getbool(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}
