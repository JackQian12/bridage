package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all runtime configuration loaded from environment variables.
type Config struct {
	// Server
	ListenAddr string
	// Database
	DatabaseURL string
	// Security
	MasterKey      string // ≥32-char secret; HKDF-derived to AES-256 key
	AdminBootstrap string // one-time token for first admin creation via HTTP endpoint
	// JWT
	JWTSecret string
	JWTExpiry time.Duration
	// Logging
	LogLevel string
	// CORS
	CORSOrigins string
	// Provider defaults
	ProviderTimeout time.Duration
	ProviderRetries int
	// Request body limit (bytes). Default 4 MiB.
	MaxRequestBodyBytes int64
	// Environment
	Env string
}

// Load reads environment variables (and an optional .env file) into Config.
func Load() (*Config, error) {
	// .env is optional; ignore error if absent
	_ = godotenv.Load()

	cfg := &Config{
		ListenAddr:     getEnvOr("LISTEN_ADDR", ":8080"),
		DatabaseURL:    mustEnv("DATABASE_URL"),
		MasterKey:      mustEnv("MASTER_KEY"),
		AdminBootstrap: os.Getenv("ADMIN_BOOTSTRAP_TOKEN"),
		JWTSecret:      mustEnv("JWT_SECRET"),
		LogLevel:       getEnvOr("LOG_LEVEL", "info"),
		CORSOrigins:    getEnvOr("CORS_ORIGINS", "*"),
		Env:            getEnvOr("ENV", "production"),
	}

	jwtExpiry, err := time.ParseDuration(getEnvOr("JWT_EXPIRY", "24h"))
	if err != nil {
		return nil, fmt.Errorf("invalid JWT_EXPIRY: %w", err)
	}
	cfg.JWTExpiry = jwtExpiry

	providerTimeout, err := time.ParseDuration(getEnvOr("PROVIDER_TIMEOUT", "120s"))
	if err != nil {
		return nil, fmt.Errorf("invalid PROVIDER_TIMEOUT: %w", err)
	}
	cfg.ProviderTimeout = providerTimeout

	retries, err := strconv.Atoi(getEnvOr("PROVIDER_RETRIES", "1"))
	if err != nil {
		return nil, fmt.Errorf("invalid PROVIDER_RETRIES: %w", err)
	}
	cfg.ProviderRetries = retries

	maxBodyMB, err := strconv.ParseInt(getEnvOr("MAX_REQUEST_BODY_MB", "4"), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid MAX_REQUEST_BODY_MB: %w", err)
	}
	cfg.MaxRequestBodyBytes = maxBodyMB * 1024 * 1024

	if len(cfg.MasterKey) < 32 {
		return nil, fmt.Errorf("MASTER_KEY must be at least 32 characters")
	}
	if len(cfg.JWTSecret) < 32 {
		return nil, fmt.Errorf("JWT_SECRET must be at least 32 characters")
	}

	return cfg, nil
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		panic(fmt.Sprintf("required environment variable %s is not set", key))
	}
	return v
}

func getEnvOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
