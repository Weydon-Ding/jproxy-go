package runtime

import (
	"context"
	"errors"
	"testing"
)

func TestRegistryInvalidate_prevalidatesUnknownBatch_andDeduplicatesEffects(t *testing.T) {
	// Given
	loader := &testLoader{snapshot: testSnapshot("new")}
	provider := NewProvider(testSnapshot("old"), loader)
	results, offsets, markers := &testCache{}, &testCache{}, &testCache{}
	registry := NewRegistry(provider, results, offsets, markers)
	before := provider.Snapshot()

	// When
	unknownErr := registry.Invalidate(context.Background(), IndexerResult, "missing")
	duplicateErr := registry.Invalidate(context.Background(), IndexerResult, IndexerResult)

	// Then
	after := provider.Snapshot()
	if !errors.Is(unknownErr, ErrUnknownCacheName) || results.clears != 1 || duplicateErr != nil || after.RadarrRevision != before.RadarrRevision || loader.loads != 0 {
		t.Fatalf("unknown=%v duplicate=%v clears=%d loads=%d", unknownErr, duplicateErr, results.clears, loader.loads)
	}
}

func TestRegistryInvalidateAll_keepsAllCaches_whenRefreshFails(t *testing.T) {
	// Given
	loader := &testLoader{err: errors.New("invalid runtime snapshot")}
	provider := NewProvider(testSnapshot("old"), loader)
	results, offsets, markers := &testCache{}, &testCache{}, &testCache{}
	registry := NewRegistry(provider, results, offsets, markers)
	before := provider.Snapshot()

	// When
	err := registry.InvalidateAll(context.Background())

	// Then
	if err == nil || results.clears != 0 || offsets.clears != 0 || markers.clears != 0 || provider.Snapshot().RadarrRevision != before.RadarrRevision {
		t.Fatalf("err=%v results=%d offsets=%d markers=%d", err, results.clears, offsets.clears, markers.clears)
	}
}
