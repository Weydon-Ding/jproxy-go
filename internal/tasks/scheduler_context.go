package tasks

import (
	"context"
	"sync"
	"time"
)

func (scheduler *Scheduler) runContext(parent context.Context) (context.Context, context.CancelFunc) {
	if scheduler.timeout <= 0 {
		return context.WithCancel(parent)
	}
	deadline := scheduler.clock.Now().Add(scheduler.timeout)
	if parentDeadline, ok := parent.Deadline(); ok && parentDeadline.Before(deadline) {
		deadline = parentDeadline
	}
	runContext := newDeadlineContext(parent, deadline)
	timer := scheduler.clock.NewTimer(scheduler.timeout)
	finished := make(chan struct{})
	var watcher sync.WaitGroup
	var finishOnce sync.Once
	watcher.Add(1)
	go func() {
		defer watcher.Done()
		defer timer.Stop()
		select {
		case <-timer.C():
			runContext.cancel(context.DeadlineExceeded)
		case <-parent.Done():
			runContext.cancel(parent.Err())
		case <-finished:
		}
	}()
	return runContext, func() {
		finishOnce.Do(func() {
			close(finished)
			watcher.Wait()
			runContext.cancel(context.Canceled)
		})
	}
}

type deadlineContext struct {
	parent   context.Context
	deadline time.Time
	done     chan struct{}
	mu       sync.Mutex
	err      error
	once     sync.Once
}

func newDeadlineContext(parent context.Context, deadline time.Time) *deadlineContext {
	return &deadlineContext{parent: parent, deadline: deadline, done: make(chan struct{})}
}

func (ctx *deadlineContext) Deadline() (time.Time, bool) { return ctx.deadline, true }

func (ctx *deadlineContext) Done() <-chan struct{} { return ctx.done }

func (ctx *deadlineContext) Err() error {
	ctx.mu.Lock()
	defer ctx.mu.Unlock()
	return ctx.err
}

func (ctx *deadlineContext) Value(key any) any { return ctx.parent.Value(key) }

func (ctx *deadlineContext) cancel(err error) {
	ctx.once.Do(func() {
		ctx.mu.Lock()
		ctx.err = err
		ctx.mu.Unlock()
		close(ctx.done)
	})
}
