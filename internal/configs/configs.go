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
	// General Server Settings
	Environment   string
	Port          int
	PowDifficulty int

	// Security Settings
	AllowedOrigins []string
	JWTSecret      string

	// S3 Storage Settings
	S3BucketName      string
	S3Endpoint        string
	S3AccessKeyID     string
	S3SecretAccessKey string

	// Database Settings
	DatabaseDSN string
}

// LoadConfig reads and parses the application configuration from environment variables.
// It provides default values for each configuration item and performs necessary type conversions and validation.
// It returns a pointer to the AppConfig struct and any error encountered.
func LoadConfig() (*AppConfig, error) {
	cfg := &AppConfig{}

	// --- General Server Settings ---
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

	// --- Security Settings ---
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

	// JWTSecret
	jwtSecret := os.Getenv("JWT_SECRET")
	if cfg.Environment == "development" {
		if jwtSecret == "" {
			jwtSecret = "your_default_insecure_secret_key_change_me"
		}
	} else {
		if jwtSecret == "" {
			return nil, fmt.Errorf("JWT_SECRET environment variable is required in %s environment for security", cfg.Environment)
		}
	}
	cfg.JWTSecret = jwtSecret

	// --- S3 Storage Settings ---
	// S3 Bucket Name
	cfg.S3BucketName = os.Getenv("S3_BUCKET_NAME")
	if cfg.S3BucketName == "" {
		return nil, fmt.Errorf("S3_BUCKET_NAME environment variable is required for S3 storage connection")
	}

	// S3 Endpoint
	cfg.S3Endpoint = os.Getenv("S3_ENDPOINT")
	if cfg.S3Endpoint == "" {
		return nil, fmt.Errorf("S3_ENDPOINT environment variable is required for S3 storage connection")
	}

	// S3 Access Key ID
	cfg.S3AccessKeyID = os.Getenv("S3_ACCESS_KEY_ID")
	if cfg.S3AccessKeyID == "" {
		return nil, fmt.Errorf("S3_ACCESS_KEY_ID environment variable is required for S3 authentication")
	}

	// S3 Secret Access Key
	cfg.S3SecretAccessKey = os.Getenv("S3_SECRET_ACCESS_KEY")
	if cfg.S3SecretAccessKey == "" {
		return nil, fmt.Errorf("S3_SECRET_ACCESS_KEY environment variable is required for S3 authentication")
	}

	// --- Database Settings ---
	cfg.DatabaseDSN = os.Getenv("DATABASE_URL")
	if cfg.DatabaseDSN == "" {
		if cfg.Environment == "development" {
			cfg.DatabaseDSN = "postgres://postgres:123456@localhost:5432/hzchat?sslmode=disable"
		} else {
			return nil, fmt.Errorf("DATABASE_URL environment variable is required in %s environment", cfg.Environment)
		}
	}

	return cfg, nil
}
