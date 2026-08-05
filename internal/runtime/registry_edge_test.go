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

func TestRegistryDeleteMarker_deletesOnlyKnownTitleMarker_withoutRefreshingProvider(t *testing.T) {
	// Given
	loader := &testLoader{snapshot: testSnapshot("new")}
	provider := NewProvider(testSnapshot("old"), loader)
	results, offsets, markers := &testCache{}, &testCache{}, &testCache{}
	registry := NewRegistry(provider, results, offsets, markers)
	before := provider.Snapshot()

	// When
	err := registry.DeleteMarker(SonarrTitleSyncInterval)
	unknownErr := registry.DeleteMarker(IndexerResult)

	// Then
	after := provider.Snapshot()
	if err != nil || !errors.Is(unknownErr, ErrUnknownCacheName) || len(markers.deleted) != 1 || markers.deleted[0] != SonarrTitleSyncInterval || loader.loads != 0 || after.RadarrRevision != before.RadarrRevision || after.SonarrRevision != before.SonarrRevision || after.RadarrSearchRevision != before.RadarrSearchRevision || after.SonarrSearchRevision != before.SonarrSearchRevision || results.clears != 0 || offsets.clears != 0 {
		t.Fatalf("err=%v unknown=%v markers=%v loads=%d before=%+v after=%+v", err, unknownErr, markers.deleted, loader.loads, before, after)
	}
}
