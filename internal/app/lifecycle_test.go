package app

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"jproxy-go/internal/config"
	"jproxy-go/internal/runtime"
	"jproxy-go/internal/store/sqlite"
)

func TestRun_ordersDatabaseTaskLifecycle_whenContextCancelled(t *testing.T) {
	// Given
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	recorder := &lifecycleRecorder{}
	store := &orderedStore{recorder: recorder, snapshotRead: make(chan struct{})}
	server := &orderedServer{recorder: recorder, done: make(chan struct{})}
	tasks := &orderedTasks{recorder: recorder, started: make(chan struct{})}
	deps := productionDependencies()
	deps.openStore = func(context.Context, string) (runtimeStore, error) {
		recorder.add("store")
		return store, nil
	}
	deps.newHandler = func(config.Config, runtime.Provider) http.Handler { return http.NewServeMux() }
	deps.newDatabaseRuntime = func(config.Config, runtime.Provider, managementStore, *slog.Logger) (databaseRuntime, error) {
		recorder.add("compose")
		return databaseRuntime{handler: http.NewServeMux(), tasks: tasks}, nil
	}
	deps.listen = func(string, string) (net.Listener, error) {
		recorder.add("listen")
		return net.Listen("tcp", "127.0.0.1:0")
	}
	deps.newServer = func(http.Handler) runtimeServer { return server }
	result := make(chan error, 1)
	go func() {
		result <- run(ctx, config.Config{Database: config.DatabaseConfig{Enabled: true, Path: "test.db"}}, testLogger(t), deps)
	}()
	tasks.awaitStarted(t)

	// When
	cancel()

	// Then
	if err := awaitResult(t, result); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	want := []string{"store", "snapshot", "compose", "listen", "serve", "tasks.start", "tasks.wait", "server.shutdown", "serve.done", "store.close"}
	if got := recorder.values(); !sameStrings(got, want) {
		t.Fatalf("lifecycle = %#v, want %#v", got, want)
	}
}

func TestRun_doesNotStartTasks_whenServeReturnsBeforeAccept(t *testing.T) {
	// Given
	store := &orderedStore{recorder: &lifecycleRecorder{}, snapshotRead: make(chan struct{})}
	tasks := &orderedTasks{recorder: store.recorder, started: make(chan struct{})}
	deps := productionDependencies()
	deps.openStore = func(context.Context, string) (runtimeStore, error) { return store, nil }
	deps.newDatabaseRuntime = func(config.Config, runtime.Provider, managementStore, *slog.Logger) (databaseRuntime, error) {
		return databaseRuntime{handler: http.NewServeMux(), tasks: tasks}, nil
	}
	deps.listen = func(string, string) (net.Listener, error) { return fakeListener{}, nil }
	serveErr := errors.New("serve failed")
	deps.newServer = func(http.Handler) runtimeServer { return immediateServer{err: serveErr} }

	// When
	err := run(context.Background(), config.Config{Database: config.DatabaseConfig{Enabled: true, Path: "test.db"}}, testLogger(t), deps)

	// Then
	if !errors.Is(err, serveErr) {
		t.Fatalf("run() error = %v", err)
	}
	if got := store.recorder.values(); !sameStrings(got, []string{"snapshot", "store.close"}) {
		t.Fatalf("events = %#v, tasks must not start or wait", got)
	}
}

func TestRun_startsTasksBeforeServeCanReturnAfterSuccessfulAccept(t *testing.T) {
	// Given
	store := &orderedStore{recorder: &lifecycleRecorder{}, snapshotRead: make(chan struct{})}
	tasks := &orderedTasks{recorder: store.recorder, started: make(chan struct{})}
	deps := productionDependencies()
	deps.openStore = func(context.Context, string) (runtimeStore, error) { return store, nil }
	deps.newDatabaseRuntime = func(config.Config, runtime.Provider, managementStore, *slog.Logger) (databaseRuntime, error) {
		return databaseRuntime{handler: http.NewServeMux(), tasks: tasks}, nil
	}
	deps.listen = func(string, string) (net.Listener, error) { return net.Listen("tcp", "127.0.0.1:0") }
	serveErr := errors.New("serve failed after accept")
	deps.newServer = func(http.Handler) runtimeServer { return acceptThenFailServer{err: serveErr} }

	// When
	err := run(context.Background(), config.Config{Database: config.DatabaseConfig{Enabled: true, Path: "test.db"}}, testLogger(t), deps)

	// Then
	if !errors.Is(err, serveErr) {
		t.Fatalf("run() error = %v", err)
	}
	if got := store.recorder.values(); !sameStrings(got, []string{"snapshot", "tasks.start", "tasks.wait", "store.close"}) {
		t.Fatalf("events = %#v, tasks must start before Serve returns", got)
	}
}

func TestServeListener_holdsAcceptedConnectionUntilCommitted(t *testing.T) {
	// Given
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()
	wrapped := newServeListener(listener)
	accepted := make(chan struct{})
	serveReturned := make(chan struct{})
	go func() {
		connection, acceptErr := wrapped.Accept()
		if acceptErr == nil {
			connection.Close()
			close(accepted)
		}
		close(serveReturned)
	}()
	connection, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("dial probe: %v", err)
	}
	defer connection.Close()
	<-wrapped.ready

	// When
	select {
	case <-serveReturned:
		t.Fatal("Serve listener returned before readiness was committed")
	default:
	}
	close(wrapped.release)

	// Then
	select {
	case <-accepted:
	case <-time.After(5 * time.Second):
		t.Fatal("accepted connection was not released after commit")
	}
}

func TestRun_doesNotStartTasks_whenReadyAndCancellationAreBothReady(t *testing.T) {
	// Given
	ctx, cancel := context.WithCancel(context.Background())
	store := &orderedStore{recorder: &lifecycleRecorder{}, snapshotRead: make(chan struct{})}
	tasks := &orderedTasks{recorder: store.recorder, started: make(chan struct{})}
	deps := productionDependencies()
	deps.openStore = func(context.Context, string) (runtimeStore, error) { return store, nil }
	deps.newDatabaseRuntime = func(config.Config, runtime.Provider, managementStore, *slog.Logger) (databaseRuntime, error) {
		return databaseRuntime{handler: http.NewServeMux(), tasks: tasks}, nil
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	deps.listen = func(string, string) (net.Listener, error) {
		return &cancelOnAcceptListener{Listener: listener, cancel: cancel}, nil
	}
	deps.newServer = func(http.Handler) runtimeServer {
		return &orderedServer{recorder: store.recorder, done: make(chan struct{})}
	}
	result := make(chan error, 1)
	go func() {
		result <- run(ctx, config.Config{Database: config.DatabaseConfig{Enabled: true, Path: "test.db"}}, testLogger(t), deps)
	}()

	// When
	if runErr := awaitResult(t, result); runErr != nil {
		t.Fatalf("run() error = %v", runErr)
	}

	// Then
	if got := store.recorder.values(); !sameStrings(got, []string{"snapshot", "serve", "server.shutdown", "serve.done", "store.close"}) {
		t.Fatalf("events = %#v, tasks must not start or wait", got)
	}
}

func TestRun_returnsProbeFailureWithoutStartingTasks(t *testing.T) {
	// Given
	probeErr := errors.New("probe unavailable")
	store := &orderedStore{recorder: &lifecycleRecorder{}, snapshotRead: make(chan struct{})}
	tasks := &orderedTasks{recorder: store.recorder, started: make(chan struct{})}
	deps := productionDependencies()
	deps.openStore = func(context.Context, string) (runtimeStore, error) { return store, nil }
	deps.newDatabaseRuntime = func(config.Config, runtime.Provider, managementStore, *slog.Logger) (databaseRuntime, error) {
		return databaseRuntime{handler: http.NewServeMux(), tasks: tasks}, nil
	}
	deps.listen = func(string, string) (net.Listener, error) { return fakeListener{}, nil }
	deps.dial = func(context.Context, string, string) (net.Conn, error) { return nil, probeErr }
	server := &probeFailureServer{done: make(chan struct{})}
	deps.newServer = func(http.Handler) runtimeServer { return server }

	// When
	err := run(context.Background(), config.Config{Database: config.DatabaseConfig{Enabled: true, Path: "test.db"}}, testLogger(t), deps)

	// Then
	if !errors.Is(err, probeErr) || !server.shutdown {
		t.Fatalf("run() error = %v, shutdown = %t", err, server.shutdown)
	}
	if got := store.recorder.values(); !sameStrings(got, []string{"snapshot", "store.close"}) {
		t.Fatalf("events = %#v, tasks must not start or wait", got)
	}
}

func TestRun_returnsReadinessProbeTimeoutWithoutStartingTasks(t *testing.T) {
	// Given
	store := &orderedStore{recorder: &lifecycleRecorder{}, snapshotRead: make(chan struct{})}
	tasks := &orderedTasks{recorder: store.recorder, started: make(chan struct{})}
	probeConnection, peerConnection := net.Pipe()
	defer peerConnection.Close()
	deps := productionDependencies()
	deps.openStore = func(context.Context, string) (runtimeStore, error) { return store, nil }
	deps.newDatabaseRuntime = func(config.Config, runtime.Provider, managementStore, *slog.Logger) (databaseRuntime, error) {
		return databaseRuntime{handler: http.NewServeMux(), tasks: tasks}, nil
	}
	deps.listen = func(string, string) (net.Listener, error) { return fakeListener{}, nil }
	deps.dial = func(context.Context, string, string) (net.Conn, error) { return probeConnection, nil }
	deps.shutdownFor = time.Millisecond
	server := &probeFailureServer{done: make(chan struct{})}
	deps.newServer = func(http.Handler) runtimeServer { return server }

	// When
	err := run(context.Background(), config.Config{Database: config.DatabaseConfig{Enabled: true, Path: "test.db"}}, testLogger(t), deps)

	// Then
	if !errors.Is(err, errProbeReadiness) || !errors.Is(err, context.DeadlineExceeded) || !server.shutdown {
		t.Fatalf("run() error = %v, shutdown = %t", err, server.shutdown)
	}
	if got := store.recorder.values(); !sameStrings(got, []string{"snapshot", "store.close"}) {
		t.Fatalf("events = %#v, tasks must not start or wait", got)
	}
}

func TestRun_doesNotConstructTasks_whenEnvironmentModeEnabled(t *testing.T) {
	// Given
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	called := false
	deps := productionDependencies()
	deps.newDatabaseRuntime = func(config.Config, runtime.Provider, managementStore, *slog.Logger) (databaseRuntime, error) {
		called = true
		return databaseRuntime{}, nil
	}
	deps.listen = func(string, string) (net.Listener, error) { return net.Listen("tcp", "127.0.0.1:0") }
	server := &environmentServer{started: make(chan struct{}), done: make(chan struct{})}
	deps.newServer = func(http.Handler) runtimeServer { return server }
	result := make(chan error, 1)
	go func() { result <- run(ctx, config.Config{}, testLogger(t), deps) }()
	<-server.started

	// When
	cancel()

	// Then
	if err := awaitResult(t, result); err != nil || called {
		t.Fatalf("run() error = %v, database runtime called = %t", err, called)
	}
}

type lifecycleRecorder struct {
	mu     sync.Mutex
	events []string
}

func (recorder *lifecycleRecorder) add(event string) {
	recorder.mu.Lock()
	recorder.events = append(recorder.events, event)
	recorder.mu.Unlock()
}

func (recorder *lifecycleRecorder) values() []string {
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	return append([]string(nil), recorder.events...)
}

type orderedStore struct {
	recorder     *lifecycleRecorder
	snapshotRead chan struct{}
}

func (store *orderedStore) Close() error { store.recorder.add("store.close"); return nil }
func (store *orderedStore) FormatterSnapshot(context.Context) (sqlite.Snapshot, error) {
	store.recorder.add("snapshot")
	close(store.snapshotRead)
	return sqlite.Snapshot{}, nil
}
func (store *orderedStore) LoadFormatterSnapshot(ctx context.Context) (sqlite.Snapshot, error) {
	return store.FormatterSnapshot(ctx)
}
func (*orderedStore) Repositories() sqlite.Repositories { return sqlite.Repositories{} }
func (*orderedStore) UpdateSystemConfigs(context.Context, []sqlite.SystemConfig) (sqlite.Snapshot, error) {
	return sqlite.Snapshot{}, nil
}

type orderedServer struct {
	recorder *lifecycleRecorder
	done     chan struct{}
}

func (server *orderedServer) Serve(listener net.Listener) error {
	server.recorder.add("serve")
	_, _ = listener.Accept()
	<-server.done
	server.recorder.add("serve.done")
	return http.ErrServerClosed
}
func (server *orderedServer) Shutdown(context.Context) error {
	server.recorder.add("server.shutdown")
	close(server.done)
	return nil
}
func (*orderedServer) Close() error { return nil }

type orderedTasks struct {
	recorder *lifecycleRecorder
	started  chan struct{}
}

func (tasks *orderedTasks) Start(context.Context) {
	tasks.recorder.add("tasks.start")
	close(tasks.started)
}
func (tasks *orderedTasks) Wait() { tasks.recorder.add("tasks.wait") }
func (tasks *orderedTasks) awaitStarted(t *testing.T) {
	t.Helper()
	select {
	case <-tasks.started:
	case <-time.After(5 * time.Second):
		t.Fatal("tasks did not start")
	}
}

func sameStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for index := range got {
		if got[index] != want[index] {
			return false
		}
	}
	return true
}

type environmentServer struct {
	started chan struct{}
	done    chan struct{}
}

func (server *environmentServer) Serve(listener net.Listener) error {
	_, _ = listener.Accept()
	close(server.started)
	<-server.done
	return http.ErrServerClosed
}
func (server *environmentServer) Shutdown(context.Context) error { close(server.done); return nil }
func (*environmentServer) Close() error                          { return nil }

type immediateServer struct{ err error }

func (server immediateServer) Serve(net.Listener) error { return server.err }
func (immediateServer) Shutdown(context.Context) error  { return nil }
func (immediateServer) Close() error                    { return nil }

type acceptThenFailServer struct{ err error }

func (server acceptThenFailServer) Serve(listener net.Listener) error {
	_, _ = listener.Accept()
	return server.err
}
func (acceptThenFailServer) Shutdown(context.Context) error { return nil }
func (acceptThenFailServer) Close() error                   { return nil }

type cancelOnAcceptListener struct {
	net.Listener
	cancel context.CancelFunc
	once   sync.Once
}

func (listener *cancelOnAcceptListener) Accept() (net.Conn, error) {
	connection, err := listener.Listener.Accept()
	if err == nil {
		listener.once.Do(listener.cancel)
	}
	return connection, err
}

type probeFailureServer struct {
	done     chan struct{}
	shutdown bool
}

func (server *probeFailureServer) Serve(net.Listener) error {
	<-server.done
	return http.ErrServerClosed
}
func (server *probeFailureServer) Shutdown(context.Context) error {
	server.shutdown = true
	close(server.done)
	return nil
}
func (*probeFailureServer) Close() error { return nil }
