package main

import (
	"context"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"
)

func TestRun_returnsFailureForInvalidConfiguration(t *testing.T) {
	// Given
	t.Setenv("JPROXY_DB_ENABLED", "not-a-boolean")
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	// When
	exitCode := run(context.Background(), logger)

	// Then
	if exitCode != 1 {
		t.Fatalf("run() exit code = %d, want 1", exitCode)
	}
}

func TestServiceContext_cancelsWhenTestControlReceivesStop(t *testing.T) {
	// Given
	t.Setenv("JPROXY_TEST_CONTROL", "stdin")
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdin pipe: %v", err)
	}
	t.Cleanup(func() {
		_ = reader.Close()
		_ = writer.Close()
	})
	ctx, stop := serviceContext(context.Background(), reader)
	t.Cleanup(stop)

	// When
	if _, err := io.WriteString(writer, "STOP\nSTOP\n"); err != nil {
		t.Fatalf("write test control: %v", err)
	}

	// Then
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("test control did not cancel service context")
	}
}
