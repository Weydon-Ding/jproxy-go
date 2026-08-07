package tasks

import (
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

type syncBuffer struct {
	mu      sync.Mutex
	value   []byte
	written chan struct{}
}

func (buffer *syncBuffer) Write(value []byte) (int, error) {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	buffer.value = append(buffer.value, value...)
	if buffer.written != nil {
		select {
		case buffer.written <- struct{}{}:
		default:
		}
	}
	return len(value), nil
}

func (buffer *syncBuffer) String() string {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	return string(buffer.value)
}

func (buffer *syncBuffer) WaitContains(t *testing.T, text string) {
	t.Helper()
	deadline := time.NewTimer(time.Second)
	defer deadline.Stop()
	for !strings.Contains(buffer.String(), text) {
		select {
		case <-buffer.written:
		case <-deadline.C:
			t.Fatalf("log output = %q, want %q", buffer.String(), text)
		}
	}
}

type fakeClock struct {
	mu       sync.Mutex
	now      time.Time
	timers   map[*fakeTimer]struct{}
	created  chan struct{}
	consumed chan struct{}
}

func newFakeClock(now time.Time) *fakeClock {
	return &fakeClock{now: now, timers: make(map[*fakeTimer]struct{}), created: make(chan struct{}, 16), consumed: make(chan struct{}, 16)}
}

func (clock *fakeClock) Now() time.Time {
	clock.mu.Lock()
	defer clock.mu.Unlock()
	return clock.now
}

func (clock *fakeClock) NewTimer(delay time.Duration) Timer {
	clock.mu.Lock()
	defer clock.mu.Unlock()
	timer := &fakeTimer{clock: clock, at: clock.now.Add(delay), channel: make(chan time.Time), stopped: make(chan struct{})}
	clock.timers[timer] = struct{}{}
	clock.created <- struct{}{}
	return timer
}

func (clock *fakeClock) WaitTimer() { <-clock.created }

func (clock *fakeClock) WaitTimerConsumed() { <-clock.consumed }

func (clock *fakeClock) Advance(delay time.Duration) {
	clock.mu.Lock()
	clock.now = clock.now.Add(delay)
	var due []*fakeTimer
	for timer := range clock.timers {
		if !timer.at.After(clock.now) {
			delete(clock.timers, timer)
			due = append(due, timer)
		}
	}
	now := clock.now
	clock.mu.Unlock()
	for _, timer := range due {
		select {
		case timer.channel <- now:
			clock.consumed <- struct{}{}
		case <-timer.stopped:
		}
	}
}

func (clock *fakeClock) ActiveTimers() int {
	clock.mu.Lock()
	defer clock.mu.Unlock()
	return len(clock.timers)
}

type fakeTimer struct {
	clock    *fakeClock
	at       time.Time
	channel  chan time.Time
	stopped  chan struct{}
	stopOnce sync.Once
}

func (timer *fakeTimer) C() <-chan time.Time { return timer.channel }

func (timer *fakeTimer) Stop() bool {
	timer.clock.mu.Lock()
	_, active := timer.clock.timers[timer]
	if active {
		delete(timer.clock.timers, timer)
	}
	timer.clock.mu.Unlock()
	timer.stopOnce.Do(func() { close(timer.stopped) })
	return active
}

var _ io.Writer = (*syncBuffer)(nil)
