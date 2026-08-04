package main

import (
	"context"
	"io"
	"log/slog"
	"testing"
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
