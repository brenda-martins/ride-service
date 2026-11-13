package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ride/internal/config"
	"ride/internal/database"
	"ride/internal/httpapi"
	"ride/internal/logging"
)

func main() {
	// The logger must exist before the config is loaded: if the config is
	// invalid we still want the error emitted as structured log.
	bootstrap := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(bootstrap)

	cfg, err := config.Load()
	if err != nil {
		bootstrap.Error("invalid configuration", "err", err)
		os.Exit(1)
	}

	logger := logging.New(cfg)
	slog.SetDefault(logger)

	logger.Info("configuration loaded",
		"env", cfg.Env,
		"http_addr", cfg.HTTPAddr,
		"log_level", cfg.LogLevel.String(),
		"log_format", cfg.LogFormat,
		"shutdown_timeout", cfg.ShutdownTimeout.String(),
	)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("could not connect to database", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	logger.Debug("database connection established")

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           httpapi.NewRouter(db, logger),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("ride-service listening", "addr", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		logger.Error("server stopped with error", "err", err)
		os.Exit(1)
	case <-ctx.Done():
	}

	logger.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown did not complete within the deadline", "err", err)
	}
}
