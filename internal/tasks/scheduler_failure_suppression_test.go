package tasks

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestScheduler_logsFirstFailureAgainAfterValidSuccess_whenImmediateAndRecurringCallbacksShareJobState(t *testing.T) {
	// Given
	clock := newFakeClock(time.Date(2026, 8, 7, 9, 0, 0, 0, time.UTC))
	var output syncBuffer
	scheduler := NewScheduler(SchedulerOptions{Clock: clock, Logger: slog.New(slog.NewTextHandler(&output, nil))})
	state := jobState{}
	results := make(chan error, 4)
	completed := make(chan struct{}, 4)
	callback := func(context.Context) error {
		result := <-results
		completed <- struct{}{}
		return result
	}
	job := Job{Name: "shared-state", ImmediateRun: callback, Run: callback}

	// When
	results <- errors.New("first failure")
	runJobAndWait(context.Background(), scheduler, job, job.ImmediateRun, &state, completed)
	results <- context.DeadlineExceeded
	runJobAndWait(context.Background(), scheduler, job, job.Run, &state, completed)
	results <- nil
	runJobAndWait(context.Background(), scheduler, job, job.Run, &state, completed)
	results <- errors.New("failure after success")
	runJobAndWait(context.Background(), scheduler, job, job.Run, &state, completed)

	// Then
	log := output.String()
	if got := strings.Count(log, "task.run.failed"); got != 2 {
		t.Fatalf("failed log count = %d, want 2; output = %q", got, log)
	}
	if !strings.Contains(log, "error_kind=task_failed") {
		t.Fatalf("log output = %q, want first failure kind", log)
	}
	if strings.Contains(log, "error_kind=deadline_exceeded") {
		t.Fatalf("log output = %q, must suppress changed failure kind before success", log)
	}
	if got := strings.Count(log, "task.run.succeeded"); got != 1 {
		t.Fatalf("success log count = %d, want 1; output = %q", got, log)
	}
}

func TestScheduler_keepsFailureEpisodeAcrossSkippedCanceledAndDeadlineNilRuns(t *testing.T) {
	// Given
	clock := newFakeClock(time.Date(2026, 8, 7, 9, 0, 0, 0, time.UTC))
	var output syncBuffer
	scheduler := NewScheduler(SchedulerOptions{Clock: clock, Logger: slog.New(slog.NewTextHandler(&output, nil)), Timeout: time.Second})
	state := jobState{}
	job := Job{Name: "non-success"}
	failingRun := func(context.Context) error { return errors.New("failure") }

	// When
	scheduler.startRun(context.Background(), job, failingRun, &state)
	scheduler.runs.Wait()

	started := make(chan struct{}, 1)
	canceled := make(chan struct{}, 1)
	runningContext, cancelRunning := context.WithCancel(context.Background())
	scheduler.startRun(runningContext, job, func(runContext context.Context) error {
		started <- struct{}{}
		<-runContext.Done()
		canceled <- struct{}{}
		return nil
	}, &state)
	receive(t, started)
	scheduler.startRun(context.Background(), job, failingRun, &state)
	cancelRunning()
	receive(t, canceled)
	scheduler.runs.Wait()

	canceledContext, cancel := context.WithCancel(context.Background())
	cancel()
	scheduler.startRun(canceledContext, job, failingRun, &state)
	scheduler.runs.Wait()

	deadlineStarted := make(chan struct{}, 1)
	scheduler.startRun(context.Background(), job, func(runContext context.Context) error {
		deadlineStarted <- struct{}{}
		<-runContext.Done()
		return nil
	}, &state)
	receive(t, deadlineStarted)
	clock.WaitTimer()
	clock.Advance(time.Second)
	scheduler.runs.Wait()

	scheduler.startRun(context.Background(), job, failingRun, &state)
	scheduler.runs.Wait()

	// Then
	log := output.String()
	if got := strings.Count(log, "task.run.failed"); got != 1 {
		t.Fatalf("failed log count = %d, want 1; output = %q", got, log)
	}
	if got := strings.Count(log, "task.run.skipped"); got != 1 {
		t.Fatalf("skipped log count = %d, want 1; output = %q", got, log)
	}
	if strings.Contains(log, "task.run.succeeded") {
		t.Fatalf("log output = %q, must not contain success", log)
	}
}

func TestScheduler_logsInitialFailureForEachJobIndependently(t *testing.T) {
	// Given
	var output syncBuffer
	scheduler := NewScheduler(SchedulerOptions{Logger: slog.New(slog.NewTextHandler(&output, nil)), MaxConcurrency: 2})
	firstState := jobState{}
	secondState := jobState{}
	failingRun := func(context.Context) error { return errors.New("failure") }
	firstJob := Job{Name: "first-job"}
	secondJob := Job{Name: "second-job"}

	// When
	scheduler.startRun(context.Background(), firstJob, failingRun, &firstState)
	scheduler.startRun(context.Background(), secondJob, failingRun, &secondState)
	scheduler.runs.Wait()

	// Then
	log := output.String()
	if got := strings.Count(log, "task.run.failed"); got != 2 {
		t.Fatalf("failed log count = %d, want 2; output = %q", got, log)
	}
	if !strings.Contains(log, "job=first-job") || !strings.Contains(log, "job=second-job") {
		t.Fatalf("log output = %q, want both job names", log)
	}
}

func runJobAndWait(ctx context.Context, scheduler *Scheduler, job Job, run func(context.Context) error, state *jobState, completed <-chan struct{}) {
	scheduler.startRun(ctx, job, run, state)
	<-completed
	scheduler.runs.Wait()
}
