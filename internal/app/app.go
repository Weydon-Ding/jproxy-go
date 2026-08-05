// Package app composes the process-owned runtime services.
package app

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"runtime/debug"
	"time"

	"jproxy-go/internal/api/system"
	"jproxy-go/internal/config"
	"jproxy-go/internal/proxy"
	"jproxy-go/internal/runtime"
	"jproxy-go/internal/store/sqlite"
)

const shutdownTimeout = 5 * time.Second

type runtimeStore interface {
	Close() error
	FormatterSnapshot(context.Context) (sqlite.Snapshot, error)
	LoadFormatterSnapshot(context.Context) (sqlite.Snapshot, error)
	Repositories() sqlite.Repositories
}

type runtimeDependencies struct {
	openStore   func(context.Context, string) (runtimeStore, error)
	listen      func(string, string) (net.Listener, error)
	newHandler  func(config.Config, runtime.Provider) http.Handler
	newServer   func(http.Handler) runtimeServer
	shutdownFor time.Duration
}

type runtimeServer interface {
	Serve(net.Listener) error
	Shutdown(context.Context) error
	Close() error
}

type FailureKind string

const (
	FailureInvalidConfig   FailureKind = "invalid_configuration"
	FailurePathRequired    FailureKind = "database_path_required"
	FailureWriterOwned     FailureKind = "database_locked"
	FailureIncompatibleDB  FailureKind = "incompatible_schema"
	FailureStore           FailureKind = "database_open_or_migration_failed"
	FailureSnapshot        FailureKind = "invalid_formatter_snapshot"
	FailureListen          FailureKind = "listen_failed"
	FailureServe           FailureKind = "serve_failed"
	FailureShutdownTimeout FailureKind = "shutdown_timeout"
	FailureCleanup         FailureKind = "cleanup_failed"
)

type failure struct {
	kind FailureKind
	err  error
}

func (e failure) Error() string { return string(e.kind) }
func (e failure) Unwrap() error { return e.err }

func FailureKindOf(err error) FailureKind {
	var typed failure
	switch {
	case errors.As(err, &typed):
		return typed.kind
	case errors.Is(err, config.ErrDatabasePathRequired):
		return FailurePathRequired
	case errors.Is(err, sqlite.ErrWriterOwned):
		return FailureWriterOwned
	case errors.Is(err, sqlite.ErrIncompatibleSchema):
		return FailureIncompatibleDB
	default:
		return FailureInvalidConfig
	}
}

func storeFailureKind(err error) FailureKind {
	switch {
	case errors.Is(err, sqlite.ErrWriterOwned):
		return FailureWriterOwned
	case errors.Is(err, sqlite.ErrIncompatibleSchema):
		return FailureIncompatibleDB
	default:
		return FailureStore
	}
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
			case "time", "level", "msg", "database_enabled", "stage", "error_kind", "hint", "listen_addr":
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
		newHandler: func(cfg config.Config, provider runtime.Provider) http.Handler {
			server := proxy.NewServerWithRuntime(cfg, proxy.RuntimeOptions{Provider: provider})
			if !cfg.Database.Enabled {
				return server.Routes()
			}
			return server.Routes()
		},
		newServer:   func(handler http.Handler) runtimeServer { return &http.Server{Handler: handler} },
		shutdownFor: shutdownTimeout,
	}
}

func localVersion(info *debug.BuildInfo, ok bool) string {
	if !ok || info.Main.Version == "" || info.Main.Version == "(devel)" {
		return "dev"
	}
	return info.Main.Version
}

func localBuildVersion() string {
	info, ok := debug.ReadBuildInfo()
	return localVersion(info, ok)
}

func sqliteStore(ctx context.Context, path string) (runtimeStore, error) {
	return sqlite.Open(ctx, path)
}

func run(ctx context.Context, cfg config.Config, logger *slog.Logger, deps runtimeDependencies) error {
	appCtx, cancel := context.WithCancel(ctx)
	var store runtimeStore
	var listener net.Listener
	var server runtimeServer
	var served <-chan error
	var runErr error
	var serveErr error
	serveDone := false
	var provider runtime.Provider

	if cfg.Database.Enabled {
		store, runErr = deps.openStore(appCtx, cfg.Database.Path)
		if runErr != nil {
			cancel()
			return failure{storeFailureKind(runErr), runErr}
		}
		_ = store.Repositories()
		snapshot, snapshotErr := store.FormatterSnapshot(appCtx)
		if snapshotErr != nil {
			return cleanup(cancel, store, listener, server, served, false, failure{FailureSnapshot, snapshotErr}, deps.shutdownFor)
		}
		provider = runtime.NewProvider(snapshot, store)
	}
	if err := ctx.Err(); err != nil {
		return cleanup(cancel, store, listener, server, served, false, failure{FailureCleanup, err}, deps.shutdownFor)
	}

	handler := deps.newHandler(cfg, provider)
	if cfg.Database.Enabled {
		managementStore, ok := store.(system.Store)
		if !ok {
			return cleanup(cancel, store, listener, server, served, false, failure{FailureStore, errors.New("management store unavailable")}, deps.shutdownFor)
		}
		handler = rootHandler(cfg, provider, managementStore)
	}
	listener, runErr = deps.listen("tcp", cfg.Addr)
	if runErr != nil {
		return cleanup(cancel, store, listener, server, served, false, failure{FailureListen, runErr}, deps.shutdownFor)
	}
	server = deps.newServer(handler)
	result := make(chan error, 1)
	served = result
	go func() { result <- server.Serve(listener) }()
	logger.Info("application.started", "database_enabled", cfg.Database.Enabled, "listen_addr", listener.Addr().String())

	select {
	case serveErr = <-served:
		serveDone = true
		if !errors.Is(serveErr, http.ErrServerClosed) {
			runErr = failure{FailureServe, serveErr}
		}
	case <-ctx.Done():
	}
	return cleanup(cancel, store, listener, server, served, serveDone, runErr, deps.shutdownFor)
}

func rootHandler(cfg config.Config, provider runtime.Provider, store managementStore) http.Handler {
	proxyServer := proxy.NewServerWithRuntime(cfg, proxy.RuntimeOptions{Provider: provider})
	if !cfg.Database.Enabled {
		return proxyServer.Routes()
	}
	root := http.NewServeMux()
	root.Handle("/api/", managementRoutes(store, provider, proxyServer.CacheRegistry()))
	root.Handle("/", proxyServer.Routes())
	return root
}

func cleanup(cancel context.CancelFunc, store runtimeStore, listener net.Listener, server runtimeServer, served <-chan error, serveDone bool, runErr error, timeout time.Duration) error {
	cancel()
	var cleanupErr error
	if server != nil && !serveDone {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), timeout)
		shutdownErr := server.Shutdown(shutdownCtx)
		shutdownCancel()
		if shutdownErr != nil {
			kind := FailureCleanup
			if errors.Is(shutdownErr, context.DeadlineExceeded) {
				kind = FailureShutdownTimeout
			}
			cleanupErr = failure{kind, errors.Join(shutdownErr, server.Close())}
		}
		if serveErr := <-served; !errors.Is(serveErr, http.ErrServerClosed) {
			cleanupErr = errors.Join(cleanupErr, failure{FailureServe, serveErr})
		}
	} else if listener != nil {
		cleanupErr = listener.Close()
	}
	if store != nil {
		if closeErr := store.Close(); closeErr != nil {
			cleanupErr = errors.Join(cleanupErr, failure{FailureCleanup, closeErr})
		}
	}
	return errors.Join(runErr, cleanupErr)
}
