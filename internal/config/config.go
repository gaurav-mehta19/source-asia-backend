// Package config loads and validates application configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config holds all application configuration loaded from the environment.
type Config struct {
	AppEnv string

	HTTPPort                string
	HTTPReadTimeoutSeconds  int
	HTTPWriteTimeoutSeconds int
	HTTPShutdownTimeout     int

	RateLimitMaxRequests   int
	RateLimitWindowSeconds int

	ProductMaxURLsPerRequest  int
	ProductMaxURLLength       int
	ProductDefaultPageLimit   int
	ProductMaxPageLimit       int
	ProductMaxMediaPerProduct int

	LogLevel string
}

// Load reads the .env file (if present) and parses all required fields.
// Returns an error if any required field is missing or invalid.
func Load(envFile string) (*Config, error) {
	// godotenv.Load is a no-op if the file doesn't exist — we handle missing vars below.
	_ = godotenv.Load(envFile)

	cfg := &Config{}
	var err error

	cfg.AppEnv = getEnv("APP_ENV", "development")
	cfg.LogLevel = getEnv("LOG_LEVEL", "info")
	cfg.HTTPPort = getEnv("HTTP_PORT", "8080")

	if cfg.HTTPReadTimeoutSeconds, err = getEnvInt("HTTP_READ_TIMEOUT_SECONDS", 10); err != nil {
		return nil, fmt.Errorf("HTTP_READ_TIMEOUT_SECONDS: %w", err)
	}
	if cfg.HTTPWriteTimeoutSeconds, err = getEnvInt("HTTP_WRITE_TIMEOUT_SECONDS", 10); err != nil {
		return nil, fmt.Errorf("HTTP_WRITE_TIMEOUT_SECONDS: %w", err)
	}
	if cfg.HTTPShutdownTimeout, err = getEnvInt("HTTP_SHUTDOWN_TIMEOUT_SECONDS", 15); err != nil {
		return nil, fmt.Errorf("HTTP_SHUTDOWN_TIMEOUT_SECONDS: %w", err)
	}
	if cfg.RateLimitMaxRequests, err = getEnvInt("RATE_LIMIT_MAX_REQUESTS", 5); err != nil {
		return nil, fmt.Errorf("RATE_LIMIT_MAX_REQUESTS: %w", err)
	}
	if cfg.RateLimitWindowSeconds, err = getEnvInt("RATE_LIMIT_WINDOW_SECONDS", 60); err != nil {
		return nil, fmt.Errorf("RATE_LIMIT_WINDOW_SECONDS: %w", err)
	}
	if cfg.ProductMaxURLsPerRequest, err = getEnvInt("PRODUCT_MAX_URLS_PER_REQUEST", 20); err != nil {
		return nil, fmt.Errorf("PRODUCT_MAX_URLS_PER_REQUEST: %w", err)
	}
	if cfg.ProductMaxURLLength, err = getEnvInt("PRODUCT_MAX_URL_LENGTH", 2048); err != nil {
		return nil, fmt.Errorf("PRODUCT_MAX_URL_LENGTH: %w", err)
	}
	if cfg.ProductDefaultPageLimit, err = getEnvInt("PRODUCT_DEFAULT_PAGE_LIMIT", 20); err != nil {
		return nil, fmt.Errorf("PRODUCT_DEFAULT_PAGE_LIMIT: %w", err)
	}
	if cfg.ProductMaxPageLimit, err = getEnvInt("PRODUCT_MAX_PAGE_LIMIT", 100); err != nil {
		return nil, fmt.Errorf("PRODUCT_MAX_PAGE_LIMIT: %w", err)
	}
	if cfg.ProductMaxMediaPerProduct, err = getEnvInt("PRODUCT_MAX_MEDIA_PER_PRODUCT", 200); err != nil {
		return nil, fmt.Errorf("PRODUCT_MAX_MEDIA_PER_PRODUCT: %w", err)
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) validate() error {
	if c.RateLimitMaxRequests <= 0 {
		return fmt.Errorf("RATE_LIMIT_MAX_REQUESTS must be > 0")
	}
	if c.RateLimitWindowSeconds <= 0 {
		return fmt.Errorf("RATE_LIMIT_WINDOW_SECONDS must be > 0")
	}
	if c.ProductDefaultPageLimit > c.ProductMaxPageLimit {
		return fmt.Errorf("PRODUCT_DEFAULT_PAGE_LIMIT must be <= PRODUCT_MAX_PAGE_LIMIT")
	}
	if c.ProductMaxMediaPerProduct <= 0 {
		return fmt.Errorf("PRODUCT_MAX_MEDIA_PER_PRODUCT must be > 0")
	}
	return nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("must be an integer, got %q", v)
	}
	return n, nil
}
