package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"jproxy-go/internal/app"
	"jproxy-go/internal/config"
)

func main() {
	logger := app.ConfiguredLogger(os.Stdout)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	os.Exit(run(ctx, logger))
}

func run(ctx context.Context, logger *slog.Logger) int {
	cfg, err := config.LoadConfig()
	if err != nil {
		logger.Error("application.failed", "stage", "configuration", "error_kind", "invalid_configuration")
		return 1
	}
	if err := app.Run(ctx, cfg, logger); err != nil {
		logger.Error("application.failed", "stage", "runtime", "error_kind", "runtime_failure")
		return 1
	}
	logger.Info("application.stopped")
	return 0
}
