package app

import (
	"context"
	"errors"
	"testing"
)

func TestStartupLoginTask_joinsBothLoginErrors_whenBothDownloadersFail(t *testing.T) {
	// Given
	qbErr := errors.New("qBittorrent login failed")
	transmissionErr := errors.New("Transmission login failed")
	task := startupLoginTask(loginFailure{err: qbErr}, loginFailure{err: transmissionErr})

	// When
	err := task(context.Background())

	// Then
	if !errors.Is(err, qbErr) || !errors.Is(err, transmissionErr) {
		t.Fatalf("startup login error = %v", err)
	}
}

type loginFailure struct{ err error }

func (failure loginFailure) Login(context.Context) error { return failure.err }
