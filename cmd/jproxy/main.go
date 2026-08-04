package main

import (
	"bufio"
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"jproxy-go/internal/app"
	"jproxy-go/internal/config"
)

func main() {
	logger := app.ConfiguredLogger(os.Stderr)
	ctx, stop := serviceContext(context.Background(), os.Stdin)
	exitCode := run(ctx, logger)
	stop()
	os.Exit(exitCode)
}

func serviceContext(parent context.Context, input *os.File) (context.Context, context.CancelFunc) {
	signalCtx, stopSignals := signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
	ctx, cancel := context.WithCancel(parent)
	go func() {
		select {
		case <-signalCtx.Done():
			stopSignals()
			cancel()
		case <-ctx.Done():
			stopSignals()
		}
	}()
	if os.Getenv("JPROXY_TEST_CONTROL") == "stdin" {
		go func() {
			scanner := bufio.NewScanner(input)
			for scanner.Scan() {
				if scanner.Text() == "STOP" {
					cancel()
					return
				}
			}
		}()
	}
	return ctx, cancel
}

func run(ctx context.Context, logger *slog.Logger) int {
	cfg, err := config.LoadConfig()
	if err != nil {
		logger.Error("application.failed", "stage", "configuration", "error_kind", app.FailureKindOf(err), "hint", "check_environment")
		return 1
	}
	if err := app.Run(ctx, cfg, logger); err != nil {
		logger.Error("application.failed", "stage", "runtime", "error_kind", app.FailureKindOf(err), "hint", "inspect_safe_category")
		return 1
	}
	logger.Info("application.stopped")
	return 0
}
