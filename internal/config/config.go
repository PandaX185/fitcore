package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Env         string
	HTTPAddr    string
	DatabaseURL string
	LogLevel    slog.Level

	RedisURL       string
	TokenSecret    string
	AccessTokenTTL time.Duration
	RefreshTTL     time.Duration

	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration

	MaxOpenConns int
	MaxIdleConns int
}

// Load reads configuration from the environment. DATABASE_URL, TOKEN_SECRET
// and REDIS_URL are required.
func Load() (Config, error) {
	cfg := Config{
		Env:         getenv("FITCORE_ENV", "development"),
		HTTPAddr:    getenv("HTTP_ADDR", ":8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		LogLevel:    parseLogLevel(os.Getenv("LOG_LEVEL")),

		RedisURL:       getenv("REDIS_URL", "redis://localhost:6379/0"),
		TokenSecret:    os.Getenv("TOKEN_SECRET"),
		AccessTokenTTL: parseDuration(getenv("TOKEN_TTL", "15m"), 15*time.Minute),
		RefreshTTL:     parseDuration(getenv("REFRESH_TOKEN_TTL", "168h"), 7*24*time.Hour),

		ReadTimeout:  parseDuration(getenv("HTTP_READ_TIMEOUT", "10s"), 10*time.Second),
		WriteTimeout: parseDuration(getenv("HTTP_WRITE_TIMEOUT", "15s"), 15*time.Second),
		IdleTimeout:  parseDuration(getenv("HTTP_IDLE_TIMEOUT", "60s"), 60*time.Second),

		// MaxOpenConns must stay <= the pgbouncer DEFAULT_POOL_SIZE (20):
		// under POOL_MODE=session every app connection can pin a server
		// connection, so the app pool must never exceed the pooler.
		MaxOpenConns: parseInt(getenv("DB_MAX_OPEN_CONNS", "15"), 15),
		MaxIdleConns: parseInt(getenv("DB_MAX_IDLE_CONNS", "5"), 5),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.TokenSecret == "" {
		return Config{}, fmt.Errorf("TOKEN_SECRET is required")
	}
	if len(cfg.TokenSecret) < 32 {
		return Config{}, fmt.Errorf("TOKEN_SECRET must be at least 32 bytes")
	}

	return cfg, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func parseDuration(s string, fallback time.Duration) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil || d <= 0 {
		return fallback
	}
	return d
}

func parseInt(s string, fallback int) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n <= 0 {
		return fallback
	}
	return n
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
