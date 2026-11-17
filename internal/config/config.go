package config

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"
)

const (
	EnvDebug = "debug"
	EnvProd  = "prod"
)

// Accepted log formats.
const (
	FormatJSON = "json"
	FormatText = "text"
)

// Config gathers the parameters read from the environment at service start.
type Config struct {
	Env             string
	HTTPAddr        string
	DatabaseURL     string
	LogLevel        slog.Level
	LogFormat       string
	ShutdownTimeout time.Duration
}

// Load reads the configuration from the environment and validates the result.
// Any invalid value fails at startup: better not to start than to start with a
// silently wrong configuration.
func Load() (Config, error) {
	cfg := Config{
		Env:             strings.ToLower(env("APP_ENV", EnvProd)),
		HTTPAddr:        env("HTTP_ADDR", ":8081"),
		DatabaseURL:     os.Getenv("DATABASE_URL"),
		LogLevel:        slog.LevelInfo,
		LogFormat:       FormatJSON,
		ShutdownTimeout: 10 * time.Second,
	}

	switch cfg.Env {
	case EnvDebug:
		// In debug the log is human-readable and verbose by default.
		cfg.LogLevel = slog.LevelDebug
		cfg.LogFormat = FormatText
	case EnvProd:
	default:
		return Config{}, fmt.Errorf("invalid APP_ENV: %q (use %q or %q)", cfg.Env, EnvDebug, EnvProd)
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}

	if v, ok := os.LookupEnv("LOG_LEVEL"); ok && v != "" {
		if err := cfg.LogLevel.UnmarshalText([]byte(v)); err != nil {
			return Config{}, fmt.Errorf("invalid LOG_LEVEL: %q (use debug, info, warn or error)", v)
		}
	}

	if v, ok := os.LookupEnv("LOG_FORMAT"); ok && v != "" {
		switch strings.ToLower(v) {
		case FormatJSON, FormatText:
			cfg.LogFormat = strings.ToLower(v)
		default:
			return Config{}, fmt.Errorf("invalid LOG_FORMAT: %q (use %q or %q)", v, FormatJSON, FormatText)
		}
	}

	if v, ok := os.LookupEnv("SHUTDOWN_TIMEOUT"); ok && v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid SHUTDOWN_TIMEOUT: %q (use a Go duration, such as %q)", v, "10s")
		}
		if d <= 0 {
			return Config{}, fmt.Errorf("SHUTDOWN_TIMEOUT must be positive: %q", v)
		}
		cfg.ShutdownTimeout = d
	}

	return cfg, nil
}

// IsDebug reports whether the service is running in development mode.
func (c Config) IsDebug() bool {
	return c.Env == EnvDebug
}

func env(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
