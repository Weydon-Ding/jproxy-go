package app

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"
)

var errProbeReadiness = errors.New("readiness probe did not reach accepted connection")

type dialContextFunc func(context.Context, string, string) (net.Conn, error)

func dialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return (&net.Dialer{}).DialContext(ctx, network, address)
}

type serveListener struct {
	net.Listener
	ready   chan struct{}
	release chan struct{}
	abort   chan struct{}
	once    sync.Once
}

func newServeListener(listener net.Listener) *serveListener {
	return &serveListener{Listener: listener, ready: make(chan struct{}, 1), release: make(chan struct{}), abort: make(chan struct{})}
}

func (listener *serveListener) Accept() (net.Conn, error) {
	connection, err := listener.Listener.Accept()
	if err != nil {
		return nil, err
	}
	accepted := false
	listener.once.Do(func() {
		accepted = true
		listener.ready <- struct{}{}
	})
	if !accepted {
		return connection, nil
	}
	select {
	case <-listener.release:
		return connection, nil
	case <-listener.abort:
		connection.Close()
		return nil, net.ErrClosed
	}
}

type serveRuntime struct {
	listener     *serveListener
	served       chan error
	probeResults chan error
	probeRelease chan struct{}
	stopProbe    context.CancelFunc
	releaseOnce  sync.Once
	abortOnce    sync.Once
}

func startServe(ctx context.Context, listener net.Listener, server runtimeServer, timeout time.Duration, dial dialContextFunc) *serveRuntime {
	wrapped := newServeListener(listener)
	runtime := &serveRuntime{listener: wrapped, served: make(chan error, 1), probeResults: make(chan error, 1), probeRelease: make(chan struct{})}
	go func() { runtime.served <- server.Serve(wrapped) }()
	probeCtx, cancel := context.WithTimeout(ctx, timeout)
	runtime.stopProbe = cancel
	go runtime.probe(probeCtx, dial)
	return runtime
}

func (runtime *serveRuntime) probe(ctx context.Context, dial dialContextFunc) {
	connection, err := dial(ctx, "tcp", runtime.listener.Addr().String())
	if err != nil {
		runtime.publishProbeResult(err)
		return
	}
	defer connection.Close()
	select {
	case <-runtime.probeRelease:
	case <-ctx.Done():
		if ctx.Err() == context.DeadlineExceeded {
			runtime.publishProbeResult(errors.Join(errProbeReadiness, ctx.Err()))
		}
	}
}

func (runtime *serveRuntime) publishProbeResult(err error) {
	select {
	case runtime.probeResults <- err:
	default:
	}
}

func (runtime *serveRuntime) commit() {
	runtime.releaseOnce.Do(func() {
		close(runtime.listener.release)
		close(runtime.probeRelease)
		runtime.stopProbe()
	})
}

func (runtime *serveRuntime) abort() {
	runtime.abortOnce.Do(func() {
		close(runtime.listener.abort)
		runtime.stopProbe()
	})
}
