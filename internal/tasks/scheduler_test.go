package tasks

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestScheduler_RunsRecurringJobOnFirstBoundary(t *testing.T) {
	// Given
	clock := newFakeClock(time.Date(2026, 8, 7, 9, 0, 0, 0, time.UTC))
	scheduler := NewScheduler(SchedulerOptions{Clock: clock})
	schedule, err := EveryMinutes(1, 0)
	if err != nil {
		t.Fatalf("EveryMinutes(): %v", err)
	}
	started := make(chan struct{}, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := scheduler.Add(Job{Name: "minute", Schedule: schedule, Run: func(context.Context) error {
		started <- struct{}{}
		return nil
	}}); err != nil {
		t.Fatalf("Add(): %v", err)
	}
	scheduler.Start(ctx)
	clock.WaitTimer()

	// When
	clock.Advance(time.Minute)

	// Then
	receive(t, started)
	cancel()
	scheduler.Wait()
}

func TestScheduler_ImmediateRunUsesDistinctCallbackBeforeRecurringBoundary(t *testing.T) {
	// Given
	clock := newFakeClock(time.Date(2026, 8, 7, 9, 0, 30, 0, time.UTC))
	scheduler := NewScheduler(SchedulerOptions{Clock: clock})
	schedule, err := EveryMinutes(1, 0)
	if err != nil {
		t.Fatalf("EveryMinutes(): %v", err)
	}
	immediateStarted := make(chan struct{}, 1)
	recurringStarted := make(chan struct{}, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := scheduler.Add(Job{Name: "downloader-login", Schedule: schedule, ImmediateRun: func(context.Context) error {
		immediateStarted <- struct{}{}
		return nil
	}, Run: func(context.Context) error {
		recurringStarted <- struct{}{}
		return nil
	}}); err != nil {
		t.Fatalf("Add(): %v", err)
	}
	scheduler.Start(ctx)
	clock.WaitTimer()

	// When
	receive(t, immediateStarted)
	assertNoReceive(t, recurringStarted)
	clock.Advance(30 * time.Second)

	// Then
	receive(t, recurringStarted)
	cancel()
	scheduler.Wait()
}

func TestScheduler_AddRejectsInvalidJob(t *testing.T) {
	// Given
	scheduler := NewScheduler(SchedulerOptions{})
	schedule, err := EverySeconds(30)
	if err != nil {
		t.Fatalf("EverySeconds(): %v", err)
	}

	// When / Then
	if err := scheduler.Add(Job{Name: "missing schedule", Run: func(context.Context) error { return nil }}); !errors.Is(err, ErrValidation) {
		t.Fatalf("nil Schedule error = %v, want ErrValidation", err)
	}
	if err := scheduler.Add(Job{Name: "missing run", Schedule: schedule}); !errors.Is(err, ErrValidation) {
		t.Fatalf("nil Run error = %v, want ErrValidation", err)
	}
}

func TestScheduler_SkipsOverlappingOccurrenceForSameJob(t *testing.T) {
	// Given
	clock := newFakeClock(time.Date(2026, 8, 7, 9, 0, 0, 0, time.UTC))
	scheduler := NewScheduler(SchedulerOptions{Clock: clock})
	schedule, err := EverySeconds(1)
	if err != nil {
		t.Fatalf("EverySeconds(): %v", err)
	}
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	finished := make(chan struct{}, 1)
	var calls int
	var lock sync.Mutex
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := scheduler.Add(Job{Name: "slow", Schedule: schedule, Run: func(context.Context) error {
		lock.Lock()
		calls++
		lock.Unlock()
		started <- struct{}{}
		<-release
		finished <- struct{}{}
		return nil
	}}); err != nil {
		t.Fatalf("Add(): %v", err)
	}
	scheduler.Start(ctx)
	clock.WaitTimer()
	clock.Advance(time.Second)
	receive(t, started)
	clock.WaitTimer()

	// When
	clock.Advance(time.Second)
	clock.WaitTimerConsumed()

	// Then
	cancel()
	close(release)
	receive(t, finished)
	scheduler.Wait()
	lock.Lock()
	defer lock.Unlock()
	if calls != 1 {
		t.Fatalf("calls = %d, want exactly 1", calls)
	}
}

func TestScheduler_UsesSharedConcurrencyLimit(t *testing.T) {
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
	firstStarted := make(chan struct{}, 1)
	secondStarted := make(chan struct{}, 1)
	release := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := scheduler.Add(Job{Name: "first", Schedule: hourly, ImmediateRun: func(context.Context) error {
		firstStarted <- struct{}{}
		<-release
		return nil
	}, Run: func(context.Context) error { return nil }}); err != nil {
		t.Fatalf("Add first: %v", err)
	}
	if err := scheduler.Add(Job{Name: "second", Schedule: secondly, Run: func(context.Context) error {
		secondStarted <- struct{}{}
		return nil
	}}); err != nil {
		t.Fatalf("Add second: %v", err)
	}
	scheduler.Start(ctx)
	receive(t, firstStarted)
	clock.WaitTimer()
	clock.WaitTimer()

	// When
	clock.Advance(time.Second)

	// Then
	assertNoReceive(t, secondStarted)
	close(release)
	receive(t, secondStarted)
	cancel()
	scheduler.Wait()
}

func receive(t *testing.T, received <-chan struct{}) {
	t.Helper()
	<-received
}

func assertNoReceive(t *testing.T, received <-chan struct{}) {
	t.Helper()
	select {
	case <-received:
		t.Fatal("received unexpected event")
	default:
	}
}
