package transmission

import (
	"context"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"

	"jproxy-go/internal/runtime"
)

type mutableProvider struct {
	mu             sync.RWMutex
	snapshot       runtime.Snapshot
	admissionCalls int
}

func (p *mutableProvider) Snapshot() runtime.Snapshot {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.snapshot
}

func (*mutableProvider) Refresh(context.Context, runtime.Scope) error { return nil }

func (p *mutableProvider) set(snapshot runtime.Snapshot) {
	p.mu.Lock()
	p.snapshot = snapshot
	p.mu.Unlock()
}

func (p *mutableProvider) AdmitTransmissionMutation(revision uint64, begin func()) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.snapshot.TransmissionRevision != revision {
		return runtime.ErrTransmissionRevisionStale
	}
	p.admissionCalls++
	begin()
	return nil
}

func (p *mutableProvider) admissions() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.admissionCalls
}

var _ runtime.TransmissionMutationAdmission = (*mutableProvider)(nil)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (transport roundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}

type readCloser struct {
	read func([]byte) (int, error)
}

func (body readCloser) Read(data []byte) (int, error) { return body.read(data) }
func (readCloser) Close() error                       { return nil }

type manualDeadlineContext struct {
	context.Context
	done chan struct{}
	mu   sync.Mutex
	err  error
}

func newManualDeadlineContext() *manualDeadlineContext {
	return &manualDeadlineContext{Context: context.Background(), done: make(chan struct{})}
}

func (ctx *manualDeadlineContext) Done() <-chan struct{} { return ctx.done }
func (ctx *manualDeadlineContext) Err() error {
	ctx.mu.Lock()
	defer ctx.mu.Unlock()
	return ctx.err
}
func (ctx *manualDeadlineContext) expire() {
	ctx.mu.Lock()
	ctx.err = context.DeadlineExceeded
	ctx.mu.Unlock()
	close(ctx.done)
}

func readErrorClient(t *testing.T, body io.ReadCloser) *Client {
	t.Helper()
	client, err := New(Options{Provider: &mutableProvider{snapshot: runtime.Snapshot{TransmissionURL: "http://example.test"}}, HTTPClient: &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: body}, nil
	})}, Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func testClient(t *testing.T, provider runtime.Provider) *Client {
	t.Helper()
	client, err := New(Options{Provider: provider, Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func waitFor[T any](t *testing.T, signal <-chan T, name string) T {
	t.Helper()
	select {
	case value := <-signal:
		return value
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for %s", name)
		var zero T
		return zero
	}
}
