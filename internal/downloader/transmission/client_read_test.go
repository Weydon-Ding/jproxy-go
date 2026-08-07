package transmission

import (
	"context"
	"errors"
	"io"
	"testing"
)

func TestClient_classifiesReadErrors_fromResponseBodies(t *testing.T) {
	t.Run("caller cancellation", func(t *testing.T) {
		// Given
		ctx, cancel := context.WithCancel(context.Background())
		body := readCloser{read: func([]byte) (int, error) {
			cancel()
			return 0, io.ErrUnexpectedEOF
		}}
		client := readErrorClient(t, body)

		// When
		_, err := client.Files(ctx, "hash")

		// Then
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("deadline", func(t *testing.T) {
		// Given
		ctx := newManualDeadlineContext()
		client := readErrorClient(t, readCloser{read: func([]byte) (int, error) {
			ctx.expire()
			return 0, io.ErrUnexpectedEOF
		}})

		// When
		_, err := client.Files(ctx, "hash")

		// Then
		if !errors.Is(err, ErrTimeout) {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("transport", func(t *testing.T) {
		// Given
		client := readErrorClient(t, readCloser{read: func([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }})

		// When
		_, err := client.Files(context.Background(), "hash")

		// Then
		if !errors.Is(err, ErrTransport) {
			t.Fatalf("err=%v", err)
		}
	})
}
