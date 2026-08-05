package runtime

import (
	"context"
	"errors"
	"sync"
	"testing"

	"jproxy-go/internal/store/sqlite"
)

type blockingLoader struct {
	started  chan struct{}
	release  chan struct{}
	snapshot sqlite.Snapshot
	mu       sync.Mutex
	loads    int
}

func (l *blockingLoader) LoadFormatterSnapshot(ctx context.Context) (sqlite.Snapshot, error) {
	l.mu.Lock()
	l.loads++
	l.mu.Unlock()
	select {
	case l.started <- struct{}{}:
	default:
	}
	select {
	case <-l.release:
		return l.snapshot, nil
	case <-ctx.Done():
		return sqlite.Snapshot{}, ctx.Err()
	}
}

func TestProviderRefresh_doesNotPublish_whenContextCancelledBeforeOrDuringLoad(t *testing.T) {
	// Given
	loader := &blockingLoader{started: make(chan struct{}, 1), release: make(chan struct{}), snapshot: testSnapshot("new")}
	provider := NewProvider(testSnapshot("old"), loader)
	before := provider.Snapshot()
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()

	// When
	beforeErr := provider.Refresh(cancelled, ScopeRadarrRules)
	ctx, stop := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() { result <- provider.Refresh(ctx, ScopeRadarrRules) }()
	<-loader.started
	stop()
	duringErr := <-result

	// Then
	after := provider.Snapshot()
	if !errors.Is(beforeErr, context.Canceled) || !errors.Is(duringErr, context.Canceled) || after.RadarrRevision != before.RadarrRevision || loader.loads != 1 {
		t.Fatalf("before=%v during=%v revisions=%d/%d loads=%d", beforeErr, duringErr, before.RadarrRevision, after.RadarrRevision, loader.loads)
	}
}

func TestProviderRefresh_publishesOnlyCompleteSnapshots_duringConcurrentReads(t *testing.T) {
	// Given
	loader := &blockingLoader{started: make(chan struct{}, 1), release: make(chan struct{}), snapshot: testSnapshot("new")}
	provider := NewProvider(testSnapshot("old"), loader)
	result := make(chan error, 1)
	go func() { result <- provider.Refresh(context.Background(), ScopeRadarrRules) }()
	<-loader.started

	// When
	for range 100 {
		snapshot := provider.Snapshot()
		if snapshot.Radarr.Rules[0].Replacement != "old" || snapshot.RadarrRevision != 2 {
			t.Fatalf("read partial snapshot: %+v", snapshot)
		}
	}
	close(loader.release)
	if err := <-result; err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}

	// Then
	after := provider.Snapshot()
	if after.Radarr.Rules[0].Replacement != "new" || after.RadarrRevision != 3 {
		t.Fatalf("published snapshot = %+v", after)
	}
}

func TestProviderRefresh_serializesConcurrentInvalidations(t *testing.T) {
	// Given
	loader := &testLoader{snapshot: testSnapshot("new")}
	provider := NewProvider(testSnapshot("old"), loader)
	registry := NewRegistry(provider, &testCache{}, &testCache{}, &testCache{})
	results := make(chan error, 4)

	// When
	for range 4 {
		go func() { results <- registry.Invalidate(context.Background(), RadarrRule) }()
	}
	for range 4 {
		if err := <-results; err != nil {
			t.Fatalf("Invalidate() error = %v", err)
		}
	}

	// Then
	snapshot := provider.Snapshot()
	if loader.loads != 4 || snapshot.RadarrRevision != 6 || snapshot.Radarr.Rules[0].Replacement != "new" {
		t.Fatalf("loads=%d revision=%d snapshot=%+v", loader.loads, snapshot.RadarrRevision, snapshot)
	}
}
