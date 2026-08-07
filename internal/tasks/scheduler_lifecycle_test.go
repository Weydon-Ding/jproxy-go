package tasks

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestScheduler_CancelsRunAtTimeoutAndRecovers(t *testing.T) {
	// Given
	clock := newFakeClock(time.Date(2026, 8, 7, 9, 0, 0, 0, time.UTC))
	var output syncBuffer
	output.written = make(chan struct{}, 1)
	logger := slog.New(slog.NewTextHandler(&output, nil))
	scheduler := NewScheduler(SchedulerOptions{Clock: clock, Logger: logger, Timeout: time.Second})
	schedule, err := EverySeconds(1)
	if err != nil {
		t.Fatalf("EverySeconds(): %v", err)
	}
	started := make(chan struct{}, 2)
	timedOut := make(chan struct{}, 1)
	var runs int
	var lock sync.Mutex
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := scheduler.Add(Job{Name: "recover", Schedule: schedule, Run: func(runContext context.Context) error {
		lock.Lock()
		runs++
		run := runs
		lock.Unlock()
		started <- struct{}{}
		if run == 1 {
			<-runContext.Done()
			timedOut <- struct{}{}
			return runContext.Err()
		}
		return nil
	}}); err != nil {
		t.Fatalf("Add(): %v", err)
	}
	scheduler.Start(ctx)
	clock.WaitTimer()
	clock.Advance(time.Second)
	receive(t, started)
	clock.WaitTimer()
	clock.WaitTimer()

	// When
	clock.Advance(time.Second)
	receive(t, timedOut)
	output.WaitWrite()
	clock.WaitTimer()
	clock.Advance(time.Second)

	// Then
	receive(t, started)
	cancel()
	scheduler.Wait()
}

func TestScheduler_CancellationStopsTimersAndWaitsForRuns(t *testing.T) {
	// Given
	clock := newFakeClock(time.Date(2026, 8, 7, 9, 0, 0, 0, time.UTC))
	scheduler := NewScheduler(SchedulerOptions{Clock: clock, Timeout: time.Hour})
	schedule, err := EverySeconds(1)
	if err != nil {
		t.Fatalf("EverySeconds(): %v", err)
	}
	started := make(chan struct{}, 1)
	finished := make(chan struct{}, 1)
	ctx, cancel := context.WithCancel(context.Background())
	if err := scheduler.Add(Job{Name: "run", Schedule: schedule, Run: func(runContext context.Context) error {
		started <- struct{}{}
		<-runContext.Done()
		finished <- struct{}{}
		return runContext.Err()
	}}); err != nil {
		t.Fatalf("Add(): %v", err)
	}
	scheduler.Start(ctx)
	clock.WaitTimer()
	clock.Advance(time.Second)
	receive(t, started)
	clock.WaitTimer()

	// When
	cancel()

	// Then
	receive(t, finished)
	scheduler.Wait()
	if clock.ActiveTimers() != 0 {
		t.Fatalf("ActiveTimers() = %d, want 0", clock.ActiveTimers())
	}
}

func TestScheduler_LogsFailureAndContinues(t *testing.T) {
	// Given
	clock := newFakeClock(time.Date(2026, 8, 7, 9, 0, 0, 0, time.UTC))
	var output syncBuffer
	output.written = make(chan struct{}, 1)
	logger := slog.New(slog.NewTextHandler(&output, nil))
	scheduler := NewScheduler(SchedulerOptions{Clock: clock, Logger: logger})
	schedule, err := EverySeconds(1)
	if err != nil {
		t.Fatalf("EverySeconds(): %v", err)
	}
	completed := make(chan struct{}, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := scheduler.Add(Job{Name: "failure", Schedule: schedule, Run: func(context.Context) error { return errors.New("failed") }}); err != nil {
		t.Fatalf("Add failure: %v", err)
	}
	if err := scheduler.Add(Job{Name: "success", Schedule: schedule, Run: func(context.Context) error {
		completed <- struct{}{}
		return nil
	}}); err != nil {
		t.Fatalf("Add success: %v", err)
	}
	scheduler.Start(ctx)
	clock.WaitTimer()
	clock.WaitTimer()

	// When
	clock.Advance(time.Second)

	// Then
	receive(t, completed)
	output.WaitWrite()
	if !strings.Contains(output.String(), "task.run.failed") || !strings.Contains(output.String(), "job=failure") {
		t.Fatalf("log output = %q, want failure event and job name", output.String())
	}
	cancel()
	scheduler.Wait()
}

func TestScheduler_CancelledWaitingJobNeverRunsAfterSemaphoreAcquisition(t *testing.T) {
	// Given
	clock := newFakeClock(time.Date(2026, 8, 7, 9, 0, 0, 0, time.UTC))
	scheduler := NewScheduler(SchedulerOptions{Clock: clock, MaxConcurrency: 1})
	hourly, err := Hourly(0, 0)
	if err != nil {
		t.Fatalf("Hourly(): %v", err)
	}
	secondly, err := EverySeconds(1)
	if err != nil {
		t.Fatalf("EverySeconds(): %v", err)
	}
	aStarted := make(chan struct{}, 1)
	aRelease := make(chan struct{})
	bCalled := make(chan struct{}, 1)
	ctx, cancel := context.WithCancel(context.Background())
	if err := scheduler.Add(Job{Name: "holder", Schedule: hourly, ImmediateRun: func(runContext context.Context) error {
		aStarted <- struct{}{}
		<-aRelease
		return runContext.Err()
	}, Run: func(context.Context) error { return nil }}); err != nil {
		t.Fatalf("Add holder: %v", err)
	}
	if err := scheduler.Add(Job{Name: "waiting", Schedule: secondly, Run: func(context.Context) error {
		bCalled <- struct{}{}
		return nil
	}}); err != nil {
		t.Fatalf("Add waiting: %v", err)
	}
	scheduler.Start(ctx)
	receive(t, aStarted)
	clock.WaitTimer()
	clock.WaitTimer()
	clock.Advance(time.Second)
	clock.WaitTimerConsumed()

	// When
	cancel()
	close(aRelease)

	// Then
	scheduler.Wait()
	select {
	case <-bCalled:
		t.Fatal("waiting callback executed after parent cancellation")
	default:
	}
}
