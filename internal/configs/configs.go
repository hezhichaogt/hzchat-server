/*
Package configs is responsible for loading and parsing the application's configuration settings.

It primarily configures server parameters by reading operating system environment variables,
including the running environment, port, CORS allowed origins, and Proof-of-Work (PoW) difficulty.
*/
package configs

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// AppConfig contains all configuration parameters required for the application to run.
// All configuration values are loaded from environment variables.
type AppConfig struct {
	// Environment defines the application's operating environment (e.g., "development", "production").
	Environment string

	// Port is the port number on which the HTTP server will listen.
	Port int

	// AllowedOrigins is the list of origins permitted for CORS and WebSocket connections.
	AllowedOrigins []string

	// PowDifficulty is the required difficulty level for the Proof-of-Work (PoW) algorithm.
	// Note: This feature is currently reserved, and the server does not yet implement PoW validation logic.
	PowDifficulty int
}

// LoadConfig reads and parses the application configuration from environment variables.
// It provides default values for each configuration item and performs necessary type conversions and validation.
// It returns a pointer to the AppConfig struct and any error encountered.
func LoadConfig() (*AppConfig, error) {
	cfg := &AppConfig{}

	// Environment
	cfg.Environment = os.Getenv("ENVIRONMENT")
	if cfg.Environment == "" {
		cfg.Environment = "development"
	}

	// Port
	portStr := os.Getenv("PORT")
	if portStr == "" {
		portStr = "8080"
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, fmt.Errorf("invalid PORT environment variable: %w", err)
	}
	cfg.Port = port

	if cfg.Port < 1024 || cfg.Port > 65535 {
		return nil, fmt.Errorf("port number %d is outside the recommended range (%d-%d) to avoid privileged ports", cfg.Port, 1024, 65535)
	}

	// AllowedOrigins
	originsStr := os.Getenv("ALLOWED_ORIGINS")
	if originsStr != "" {
		origins := strings.Split(originsStr, ",")
		for _, origin := range origins {
			trimmed := strings.TrimSpace(origin)
			if trimmed != "" {
				cfg.AllowedOrigins = append(cfg.AllowedOrigins, trimmed)
			}
		}
	} else {
		cfg.AllowedOrigins = []string{}
	}

	// PowDifficulty
	difficultyStr := os.Getenv("POW_DIFFICULTY")
	if difficultyStr == "" {
		difficultyStr = "4"
	}
	difficulty, err := strconv.Atoi(difficultyStr)
	if err != nil {
		return nil, fmt.Errorf("invalid POW_DIFFICULTY environment variable: %w", err)
	}
	cfg.PowDifficulty = difficulty

	return cfg, nil
}
