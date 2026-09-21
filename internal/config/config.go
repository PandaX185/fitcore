package config

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
)

type Config struct {
	Env         string
	HTTPAddr    string
	DatabaseURL string
	LogLevel    slog.Level
}

// Load reads configuration from the environment. DATABASE_URL is required.
func Load() (Config, error) {
	cfg := Config{
		Env:         getenv("FITCORE_ENV", "development"),
		HTTPAddr:    getenv("HTTP_ADDR", ":8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		LogLevel:    parseLogLevel(os.Getenv("LOG_LEVEL")),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}

	return cfg, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func parseLogLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
