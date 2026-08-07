package tasks

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"
)

type testContextKey struct{}

func TestScheduler_logsStableSuccessWithoutCallbackData(t *testing.T) {
	// Given
	clock := newFakeClock(time.Date(2026, 8, 7, 9, 0, 0, 0, time.UTC))
	var output syncBuffer
	logger := slog.New(slog.NewTextHandler(&output, nil))
	scheduler := NewScheduler(SchedulerOptions{Clock: clock, Logger: logger})
	schedule, err := EverySeconds(1)
	if err != nil {
		t.Fatalf("EverySeconds(): %v", err)
	}
	completed := make(chan struct{}, 1)
	contextValue := make(chan string, 1)
	secretContext := context.WithValue(context.Background(), testContextKey{}, "SUPER_SECRET")
	ctx, cancel := context.WithCancel(secretContext)
	if err := scheduler.Add(Job{Name: "safe-success", Schedule: schedule, Run: func(runContext context.Context) error {
		contextValue <- runContext.Value(testContextKey{}).(string)
		completed <- struct{}{}
		return nil
	}}); err != nil {
		t.Fatalf("Add(): %v", err)
	}
	scheduler.Start(ctx)
	clock.WaitTimer()

	// When
	clock.Advance(time.Second)
	receive(t, completed)
	cancel()
	scheduler.Wait()

	// Then
	log := output.String()
	if got := <-contextValue; got != "SUPER_SECRET" {
		t.Fatalf("callback context value = %q", got)
	}
	if !strings.Contains(log, "task.run.succeeded") || !strings.Contains(log, "job=safe-success") || !strings.Contains(log, "duration_category=") || !strings.Contains(log, "duration_ms=") {
		t.Fatalf("log output = %q, want stable success fields", log)
	}
	if strings.Contains(log, "SUPER_SECRET") {
		t.Fatalf("log output leaked callback data: %q", log)
	}
}

func TestScheduler_doesNotLogSuccessWhenCallbackReturnsNilAfterDeadline(t *testing.T) {
	// Given
	clock := newFakeClock(time.Date(2026, 8, 7, 9, 0, 0, 0, time.UTC))
	var output syncBuffer
	logger := slog.New(slog.NewTextHandler(&output, nil))
	scheduler := NewScheduler(SchedulerOptions{Clock: clock, Logger: logger, Timeout: time.Second})
	schedule, err := EverySeconds(60)
	if err != nil {
		t.Fatalf("EverySeconds(): %v", err)
	}
	started := make(chan struct{}, 1)
	deadlineObserved := make(chan error, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := scheduler.Add(Job{Name: "deadline-nil", Schedule: schedule, ImmediateRun: func(runContext context.Context) error {
		started <- struct{}{}
		<-runContext.Done()
		deadlineObserved <- runContext.Err()
		return nil
	}, Run: func(context.Context) error { return nil }}); err != nil {
		t.Fatalf("Add(): %v", err)
	}
	scheduler.Start(ctx)
	clock.WaitTimer()
	receive(t, started)
	clock.WaitTimer()

	// When
	clock.Advance(time.Second)
	cancel()
	scheduler.Wait()

	// Then
	if contextErr := <-deadlineObserved; !errors.Is(contextErr, context.DeadlineExceeded) {
		t.Fatalf("callback context error = %v, want deadline exceeded", contextErr)
	}
	if strings.Contains(output.String(), "task.run.succeeded") {
		t.Fatalf("log output = %q, must not contain success after deadline", output.String())
	}
}

func TestScheduler_doesNotLogSuccessWhenCallbackReturnsNilAfterParentCancellation(t *testing.T) {
	// Given
	clock := newFakeClock(time.Date(2026, 8, 7, 9, 0, 0, 0, time.UTC))
	var output syncBuffer
	logger := slog.New(slog.NewTextHandler(&output, nil))
	scheduler := NewScheduler(SchedulerOptions{Clock: clock, Logger: logger})
	schedule, err := EverySeconds(60)
	if err != nil {
		t.Fatalf("EverySeconds(): %v", err)
	}
	started := make(chan struct{}, 1)
	parentCanceled := make(chan struct{}, 1)
	ctx, cancel := context.WithCancel(context.Background())
	if err := scheduler.Add(Job{Name: "parent-nil", Schedule: schedule, ImmediateRun: func(runContext context.Context) error {
		started <- struct{}{}
		<-runContext.Done()
		parentCanceled <- struct{}{}
		return nil
	}, Run: func(context.Context) error { return nil }}); err != nil {
		t.Fatalf("Add(): %v", err)
	}
	scheduler.Start(ctx)
	clock.WaitTimer()
	receive(t, started)

	// When
	cancel()
	receive(t, parentCanceled)
	scheduler.Wait()

	// Then
	if strings.Contains(output.String(), "task.run.succeeded") {
		t.Fatalf("log output = %q, must not contain success after parent cancellation", output.String())
	}
}

func TestScheduler_systemClockTimeoutPreservesDeadlineSemantics(t *testing.T) {
	// Given
	timeout := 20 * time.Millisecond
	scheduler := NewScheduler(SchedulerOptions{Clock: SystemClock{}, Timeout: timeout})
	startedAt := time.Now()

	// When
	runContext, cancel := scheduler.runContext(context.Background())
	defer cancel()
	deadline, ok := runContext.Deadline()
	if !ok {
		t.Fatal("run context has no deadline")
	}
	select {
	case <-runContext.Done():
	case <-time.After(time.Second):
		t.Fatal("system clock timeout did not cancel context")
	}

	// Then
	if deadline.Before(startedAt.Add(timeout)) || deadline.After(time.Now()) {
		t.Fatalf("run context deadline = %v, want system-clock timeout boundary", deadline)
	}
	if !errors.Is(runContext.Err(), context.DeadlineExceeded) {
		t.Fatalf("run context error = %v, want deadline exceeded", runContext.Err())
	}
}

func TestScheduler_runContextCancelIsIdempotent(t *testing.T) {
	// Given
	scheduler := NewScheduler(SchedulerOptions{Timeout: time.Hour})
	_, cancel := scheduler.runContext(context.Background())
	t.Cleanup(cancel)

	// When
	cancel()
	cancel()

	// Then: context.CancelFunc permits repeated calls without panicking.
}
