package app

import (
	"context"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"jproxy-go/internal/store/sqlite"
)

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

func (store *orderedStore) Close() error {
	store.recorder.add("store.close")
	return nil
}

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

func (server *environmentServer) Shutdown(context.Context) error {
	close(server.done)
	return nil
}

func (*environmentServer) Close() error { return nil }

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
