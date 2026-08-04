// Package app composes the process-owned runtime services.
package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"time"

	"jproxy-go/internal/config"
	"jproxy-go/internal/proxy"
	"jproxy-go/internal/store/sqlite"
)

const shutdownTimeout = 5 * time.Second

type runtimeStore interface {
	Close() error
	FormatterSnapshot(context.Context) (sqlite.Snapshot, error)
	Repositories() sqlite.Repositories
}

type runtimeDependencies struct {
	openStore   func(context.Context, string) (runtimeStore, error)
	listen      func(string, string) (net.Listener, error)
	newHandler  func(config.Config) http.Handler
	newServer   func(http.Handler) runtimeServer
	shutdownFor time.Duration
}

type runtimeServer interface {
	Serve(net.Listener) error
	Shutdown(context.Context) error
	Close() error
}

// Run opens all required resources before listening and closes them in reverse
// dependency order after ctx is cancelled.
func Run(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	return run(ctx, cfg, logger, productionDependencies())
}

// ConfiguredLogger writes JSON records containing only stable application fields.
func ConfiguredLogger(output io.Writer) *slog.Logger {
	return slog.New(slog.NewJSONHandler(output, &slog.HandlerOptions{
		ReplaceAttr: func(_ []string, attribute slog.Attr) slog.Attr {
			switch attribute.Key {
			case "time", "level", "msg", "database_enabled", "stage", "error_kind":
				return attribute
			default:
				return slog.Attr{}
			}
		},
	}))
}

func productionDependencies() runtimeDependencies {
	return runtimeDependencies{
		openStore: sqliteStore,
		listen:    net.Listen,
		newHandler: func(cfg config.Config) http.Handler {
			return proxy.NewServer(cfg).Routes()
		},
		newServer:   func(handler http.Handler) runtimeServer { return &http.Server{Handler: handler} },
		shutdownFor: shutdownTimeout,
	}
}

func sqliteStore(ctx context.Context, path string) (runtimeStore, error) {
	return sqlite.Open(ctx, path)
}

func run(ctx context.Context, cfg config.Config, logger *slog.Logger, deps runtimeDependencies) (runErr error) {
	appCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var store runtimeStore
	if cfg.Database.Enabled {
		store, runErr = deps.openStore(appCtx, cfg.Database.Path)
		if runErr != nil {
			return fmt.Errorf("open application store: %w", runErr)
		}
		defer func() { runErr = errors.Join(runErr, store.Close()) }()

		_ = store.Repositories()
		snapshot, snapshotErr := store.FormatterSnapshot(appCtx)
		if snapshotErr != nil {
			return fmt.Errorf("load formatter snapshot: %w", snapshotErr)
		}
		cfg.RadarrFormatting.Config = snapshot.Radarr
		cfg.SonarrFormatting.Config = snapshot.Sonarr
	}

	handler := deps.newHandler(cfg)
	listener, listenErr := deps.listen("tcp", cfg.Addr)
	if listenErr != nil {
		return fmt.Errorf("listen: %w", listenErr)
	}
	server := deps.newServer(handler)
	serveResult := make(chan error, 1)
	go func() { serveResult <- server.Serve(listener) }()
	logger.Info("application.started", "database_enabled", cfg.Database.Enabled)

	select {
	case serveErr := <-serveResult:
		if !errors.Is(serveErr, http.ErrServerClosed) {
			return fmt.Errorf("serve HTTP: %w", serveErr)
		}
		return nil
	case <-ctx.Done():
	}

	cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), deps.shutdownFor)
	shutdownErr := server.Shutdown(shutdownCtx)
	shutdownCancel()
	if shutdownErr != nil {
		shutdownErr = errors.Join(shutdownErr, server.Close())
	}
	serveErr := <-serveResult
	if errors.Is(serveErr, http.ErrServerClosed) {
		serveErr = nil
	}
	return errors.Join(shutdownErr, serveErr)
}
