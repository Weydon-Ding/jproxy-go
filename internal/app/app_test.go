package app

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
	"testing"
	"time"

	"jproxy-go/internal/config"
	"jproxy-go/internal/runtime"
	"jproxy-go/internal/store/sqlite"

	_ "modernc.org/sqlite"
)

func TestRun_servesHealthAndShutsDown_whenEnvironmentModeIsCancelled(t *testing.T) {
	// Given
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ready := make(chan struct{})
	deps := productionDependencies()
	listenerReady := make(chan net.Listener, 1)
	deps.listen = func(network, _ string) (net.Listener, error) {
		listener, err := net.Listen(network, "127.0.0.1:0")
		if err == nil {
			listenerReady <- listener
		}
		return listener, err
	}
	result := make(chan error, 1)
	logger := slog.New(startedHandler{ready: ready})
	go func() { result <- run(ctx, config.Config{Addr: "127.0.0.1:0"}, logger, deps) }()
	listener := <-listenerReady
	<-ready

	// When
	response, err := http.Get("http://" + listener.Addr().String() + "/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	body, readErr := io.ReadAll(response.Body)
	closeErr := response.Body.Close()
	cancel()

	// Then
	if readErr != nil || closeErr != nil || response.StatusCode != http.StatusOK || string(body) != "ok" {
		t.Fatalf("health response = status %d body %q read %v close %v", response.StatusCode, body, readErr, closeErr)
	}
	if err := awaitResult(t, result); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestLocalVersion_usesReleaseVersionWhenAvailable(t *testing.T) {
	if actual := localVersion(&debug.BuildInfo{Main: debug.Module{Version: "v1.2.3"}}, true); actual != "v1.2.3" {
		t.Fatalf("release version=%q", actual)
	}
	if actual := localVersion(&debug.BuildInfo{Main: debug.Module{Version: "(devel)"}}, true); actual != "dev" {
		t.Fatalf("development version=%q", actual)
	}
}

func TestRun_doesNotListen_whenStoreStartupFails(t *testing.T) {
	// Given
	called := false
	deps := productionDependencies()
	deps.openStore = func(context.Context, string) (runtimeStore, error) { return nil, errors.New("canary-secret") }
	deps.listen = func(string, string) (net.Listener, error) {
		called = true
		return nil, errors.New("unexpected listen")
	}

	// When
	err := run(context.Background(), config.Config{Database: config.DatabaseConfig{Enabled: true, Path: "canary-secret"}}, testLogger(t), deps)

	// Then
	if err == nil || called {
		t.Fatalf("Run() error = %v, listen called = %t", err, called)
	}
}

func TestRun_closesStoreAfterHTTPShutdown_whenDatabaseModeIsCancelled(t *testing.T) {
	// Given
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	store := &fakeStore{snapshot: sqlite.Snapshot{}, snapshotRead: make(chan struct{})}
	handlerReady := make(chan struct{})
	deps := productionDependencies()
	deps.openStore = func(context.Context, string) (runtimeStore, error) { return store, nil }
	deps.listen = func(network, _ string) (net.Listener, error) { return net.Listen(network, "127.0.0.1:0") }
	deps.newHandler = func(config.Config, runtime.Provider) http.Handler {
		close(handlerReady)
		return http.NewServeMux()
	}
	deps.newDatabaseRuntime = func(_ config.Config, _ runtime.Provider, _ managementStore, _ *slog.Logger) (databaseRuntime, error) {
		close(handlerReady)
		return databaseRuntime{handler: http.NewServeMux()}, nil
	}
	result := make(chan error, 1)
	go func() {
		result <- run(ctx, config.Config{Addr: "127.0.0.1:0", Database: config.DatabaseConfig{Enabled: true, Path: "temp.db"}}, testLogger(t), deps)
	}()
	<-handlerReady

	// When
	cancel()

	// Then
	if err := awaitResult(t, result); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !store.closed {
		t.Fatal("store was not closed")
	}
}

func TestRun_redactsStartupError(t *testing.T) {
	// Given
	var output strings.Builder
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	deps := productionDependencies()
	deps.openStore = func(context.Context, string) (runtimeStore, error) { return nil, errors.New("token=canary-secret") }

	// When
	err := run(context.Background(), config.Config{Database: config.DatabaseConfig{Enabled: true, Path: "canary-secret"}}, logger, deps)
	if err != nil {
		logger.Error("application.failed", "operation", "startup")
	}

	// Then
	if strings.Contains(output.String(), "canary-secret") {
		t.Fatalf("log leaked secret: %s", output.String())
	}
}

func TestRun_migratesDatabaseBeforeRejectingInvalidSnapshot(t *testing.T) {
	// Given
	path := t.TempDir() + "/runtime.db"
	called := false
	deps := productionDependencies()
	deps.listen = func(string, string) (net.Listener, error) {
		called = true
		return nil, errors.New("unexpected listener")

	}

	// When
	err := run(context.Background(), config.Config{Database: config.DatabaseConfig{Enabled: true, Path: path}}, testLogger(t), deps)

	// Then
	if err == nil || called {
		t.Fatalf("Run() error = %v, listener called = %t", err, called)
	}
	db, openErr := sql.Open("sqlite", path)
	if openErr != nil {
		t.Fatalf("open migrated database: %v", openErr)
	}
	defer db.Close()
	var migrations int
	if queryErr := db.QueryRow(`SELECT count(*) FROM jproxy_go_migration`).Scan(&migrations); queryErr != nil || migrations == 0 {
		t.Fatalf("migration ledger = %d, error = %v", migrations, queryErr)
	}
}

func TestRun_forcesClose_whenShutdownTimesOut(t *testing.T) {
	// Given
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	server := &fakeServer{serving: make(chan struct{}), serveDone: make(chan struct{}), shutdownErr: errors.New("shutdown timeout")}
	deps := productionDependencies()
	deps.listen = func(string, string) (net.Listener, error) { return net.Listen("tcp", "127.0.0.1:0") }
	deps.newServer = func(http.Handler) runtimeServer { return server }
	deps.shutdownFor = time.Millisecond
	result := make(chan error, 1)
	go func() { result <- run(ctx, config.Config{}, testLogger(t), deps) }()
	<-server.serving

	// When
	cancel()

	// Then
	err := awaitResult(t, result)
	if err == nil || !server.closed || !server.shutdownCalled {
		t.Fatalf("Run() error = %v, shutdown = %t, close = %t", err, server.shutdownCalled, server.closed)
	}
}

func TestRun_returnsStoreCloseError_afterCleanShutdown(t *testing.T) {
	// Given
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	store := &fakeStore{snapshot: sqlite.Snapshot{}, snapshotRead: make(chan struct{}), closeErr: errors.New("close store")}
	deps := productionDependencies()
	deps.openStore = func(context.Context, string) (runtimeStore, error) { return store, nil }
	deps.listen = func(network, _ string) (net.Listener, error) { return net.Listen(network, "127.0.0.1:0") }
	deps.newDatabaseRuntime = func(cfg config.Config, provider runtime.Provider, _ managementStore, _ *slog.Logger) (databaseRuntime, error) {
		return databaseRuntime{handler: deps.newHandler(cfg, provider)}, nil
	}
	result := make(chan error, 1)
	go func() {
		result <- run(ctx, config.Config{Addr: "127.0.0.1:0", Database: config.DatabaseConfig{Enabled: true, Path: "temp.db"}}, testLogger(t), deps)
	}()
	<-store.snapshotRead

	// When
	cancel()

	// Then
	if err := awaitResult(t, result); !errors.Is(err, store.closeErr) {
		t.Fatalf("Run() error = %v, want store close error", err)
	}
}

func TestConfiguredLogger_omitsSecretAttributes(t *testing.T) {
	// Given
	var output strings.Builder
	logger := ConfiguredLogger(&output)

	// When
	logger.Error("application.failed", "stage", "runtime", "token", "canary-secret", "path", "canary.db")

	// Then
	if strings.Contains(output.String(), "canary-secret") || strings.Contains(output.String(), "canary.db") {
		t.Fatalf("log leaked secret data: %s", output.String())
	}
	if !strings.Contains(output.String(), `"stage":"runtime"`) {
		t.Fatalf("log omitted allowlisted stage: %s", output.String())
	}
}

