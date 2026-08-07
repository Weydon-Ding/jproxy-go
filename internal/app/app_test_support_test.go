package app

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"testing"
	"time"

	"jproxy-go/internal/store/sqlite"
)

type fakeStore struct {
	snapshot     sqlite.Snapshot
	snapshotRead chan struct{}
	closed       bool
	closeErr     error
}

func (store *fakeStore) Close() error {
	store.closed = true
	return store.closeErr
}

func (store *fakeStore) FormatterSnapshot(context.Context) (sqlite.Snapshot, error) {
	close(store.snapshotRead)
	return store.snapshot, nil
}

func (store *fakeStore) LoadFormatterSnapshot(ctx context.Context) (sqlite.Snapshot, error) {
	return store.FormatterSnapshot(ctx)
}

func (*fakeStore) Repositories() sqlite.Repositories { return sqlite.Repositories{} }

func (*fakeStore) UpdateSystemConfigs(context.Context, []sqlite.SystemConfig) (sqlite.Snapshot, error) {
	return sqlite.Snapshot{}, errors.New("not implemented")
}

type fakeServer struct {
	serving        chan struct{}
	serveDone      chan struct{}
	shutdownCalled bool
	shutdownErr    error
	closed         bool
}

func (server *fakeServer) Serve(listener net.Listener) error {
	_, _ = listener.Accept()
	close(server.serving)
	<-server.serveDone
	return http.ErrServerClosed
}

func (server *fakeServer) Shutdown(context.Context) error {
	server.shutdownCalled = true
	return server.shutdownErr
}

func (server *fakeServer) Close() error {
	server.closed = true
	close(server.serveDone)
	return nil
}

type fakeListener struct{}

func (fakeListener) Accept() (net.Conn, error) { return nil, net.ErrClosed }
func (fakeListener) Close() error              { return nil }
func (fakeListener) Addr() net.Addr            { return &net.TCPAddr{} }

type startedHandler struct{ ready chan<- struct{} }

func (handler startedHandler) Enabled(context.Context, slog.Level) bool { return true }

func (handler startedHandler) Handle(_ context.Context, record slog.Record) error {
	if record.Message == "application.started" {
		handler.ready <- struct{}{}
	}
	return nil
}

func (handler startedHandler) WithAttrs([]slog.Attr) slog.Handler { return handler }
func (handler startedHandler) WithGroup(string) slog.Handler      { return handler }

func awaitResult(t *testing.T, result <-chan error) error {
	t.Helper()
	select {
	case err := <-result:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("Run() did not return")
		return nil
	}
}

func testLogger(t *testing.T) *slog.Logger {
	t.Helper()
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}
