package tasks

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// Clock provides wall-clock time and cancellable timers for the scheduler.
type Clock interface {
	Now() time.Time
	NewTimer(time.Duration) Timer
}

// Timer is a single event which can be stopped before it fires.
type Timer interface {
	C() <-chan time.Time
	Stop() bool
}

// SystemClock is the production Clock implementation.
type SystemClock struct{}

func (SystemClock) Now() time.Time { return time.Now() }

func (SystemClock) NewTimer(delay time.Duration) Timer {
	return systemTimer{Timer: time.NewTimer(delay)}
}

type systemTimer struct{ *time.Timer }

func (timer systemTimer) C() <-chan time.Time { return timer.Timer.C }

// Job defines one recurring callback and an optional distinct startup callback.
type Job struct {
	Name         string
	Schedule     Schedule
	ImmediateRun func(context.Context) error
	Run          func(context.Context) error
}

// SchedulerOptions configures shared execution behavior.
type SchedulerOptions struct {
	Clock          Clock
	Logger         *slog.Logger
	MaxConcurrency int
	Timeout        time.Duration
}

// Scheduler starts one boundary-wait loop per registered job.
type Scheduler struct {
	clock     Clock
	logger    *slog.Logger
	timeout   time.Duration
	semaphore chan struct{}
	jobs      []Job
	loops     sync.WaitGroup
	runs      sync.WaitGroup
	startMu   sync.Mutex
	started   bool
}

// NewScheduler constructs a scheduler with a default shared concurrency limit of one.
func NewScheduler(options SchedulerOptions) *Scheduler {
	clock := options.Clock
	if clock == nil {
		clock = SystemClock{}
	}
	logger := options.Logger
	if logger == nil {
		logger = slog.Default()
	}
	limit := options.MaxConcurrency
	if limit < 1 {
		limit = 1
	}
	return &Scheduler{clock: clock, logger: logger, timeout: options.Timeout, semaphore: make(chan struct{}, limit)}
}

// Add validates and registers a job before Start.
func (scheduler *Scheduler) Add(job Job) error {
	scheduler.startMu.Lock()
	defer scheduler.startMu.Unlock()
	if scheduler.started {
		return &ValidationError{Field: "job", Rule: "added before scheduler start"}
	}
	if job.Name == "" {
		return &ValidationError{Field: "job.name", Rule: "non-empty"}
	}
	if job.Schedule == nil {
		return &ValidationError{Field: "job.schedule", Rule: "non-nil"}
	}
	if job.Run == nil {
		return &ValidationError{Field: "job.run", Rule: "non-nil"}
	}
	scheduler.jobs = append(scheduler.jobs, job)
	return nil
}

// Start begins all registered job loops. Calling Start more than once has no effect.
func (scheduler *Scheduler) Start(ctx context.Context) {
	scheduler.startMu.Lock()
	defer scheduler.startMu.Unlock()
	if scheduler.started {
		return
	}
	scheduler.started = true
	for _, job := range scheduler.jobs {
		scheduler.loops.Add(1)
		go scheduler.waitLoop(ctx, job)
	}
}

// Wait returns only after every wait loop and active callback has exited.
func (scheduler *Scheduler) Wait() {
	scheduler.loops.Wait()
	scheduler.runs.Wait()
}

func (scheduler *Scheduler) waitLoop(ctx context.Context, job Job) {
	defer scheduler.loops.Done()
	state := jobState{}
	if job.ImmediateRun != nil {
		scheduler.startRun(ctx, job, job.ImmediateRun, &state)
	}
	for {
		now := scheduler.clock.Now()
		next := job.Schedule.Next(now)
		timer := scheduler.clock.NewTimer(next.Sub(now))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C():
			scheduler.startRun(ctx, job, job.Run, &state)
		}
	}
}

type jobState struct {
	mu      sync.Mutex
	running bool
	waiting bool
}

func (scheduler *Scheduler) startRun(ctx context.Context, job Job, run func(context.Context) error, state *jobState) {
	state.mu.Lock()
	if state.running || state.waiting {
		state.mu.Unlock()
		scheduler.logger.Warn("task.run.skipped", "job", job.Name, "reason", "in_flight")
		return
	}
	state.waiting = true
	state.mu.Unlock()
	scheduler.runs.Add(1)
	go func() {
		defer scheduler.runs.Done()
		select {
		case scheduler.semaphore <- struct{}{}:
		case <-ctx.Done():
			state.clearWaiting()
			return
		}
		if ctx.Err() != nil {
			<-scheduler.semaphore
			state.clearWaiting()
			return
		}
		state.mu.Lock()
		state.waiting = false
		state.running = true
		state.mu.Unlock()
		runContext, cancel := scheduler.runContext(ctx)
		err := run(runContext)
		cancel()
		state.mu.Lock()
		state.running = false
		state.mu.Unlock()
		<-scheduler.semaphore
		if err != nil && ctx.Err() == nil {
			scheduler.logger.Warn("task.run.failed", "job", job.Name)
		}
	}()
}

func (state *jobState) clearWaiting() {
	state.mu.Lock()
	state.waiting = false
	state.mu.Unlock()
}

func (scheduler *Scheduler) runContext(parent context.Context) (context.Context, context.CancelFunc) {
	if scheduler.timeout <= 0 {
		return context.WithCancel(parent)
	}
	runContext, cancel := context.WithTimeout(parent, scheduler.timeout)
	timer := scheduler.clock.NewTimer(scheduler.timeout)
	finished := make(chan struct{})
	var watcher sync.WaitGroup
	watcher.Add(1)
	go func() {
		defer watcher.Done()
		defer timer.Stop()
		select {
		case <-timer.C():
			cancel()
		case <-parent.Done():
			cancel()
		case <-finished:
		}
	}()
	return runContext, func() {
		close(finished)
		watcher.Wait()
		cancel()
	}
}
