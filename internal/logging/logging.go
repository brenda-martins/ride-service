package logging

import (
	"log/slog"
	"os"

	"ride/internal/config"
)

// New builds the service logger. In debug the output is human-readable (text)
// and includes the file:line of every call; in production it is JSON, for
// consumption by log collectors. The service=ride attribute is attached to
// every line.
func New(cfg config.Config) *slog.Logger {
	opts := &slog.HandlerOptions{
		Level:     cfg.LogLevel,
		AddSource: cfg.IsDebug(),
	}

	var handler slog.Handler
	switch cfg.LogFormat {
	case config.FormatText:
		handler = slog.NewTextHandler(os.Stdout, opts)
	default:
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}

	return slog.New(handler).With("service", "ride")
}
