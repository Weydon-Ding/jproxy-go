package app

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"

	"jproxy-go/internal/store/sqlite"
)

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

func cleanup(cancel context.CancelFunc, tasks taskRuntime, store runtimeStore, listener net.Listener, server runtimeServer, served <-chan error, serveDone bool, runErr error, timeout time.Duration) error {
	cancel()
	if tasks != nil {
		tasks.Wait()
	}
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
