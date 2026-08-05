package runtime

import (
	"context"
	"errors"
	"testing"

	"jproxy-go/internal/format"
	"jproxy-go/internal/store/sqlite"
)

type testLoader struct {
	snapshot sqlite.Snapshot
	err      error
}

func (l *testLoader) LoadFormatterSnapshot(context.Context) (sqlite.Snapshot, error) {
	return l.snapshot, l.err
}

type testCache struct {
	clears  int
	deleted []string
}

func (c *testCache) Clear()            { c.clears++ }
func (c *testCache) Delete(key string) { c.deleted = append(c.deleted, key) }

func TestRegistryInvalidate_refreshesOnlyNamedRevision_andKeepsUnrelatedCaches(t *testing.T) {
	// Given
	loader := &testLoader{snapshot: testSnapshot("new")}
	provider := NewProvider(testSnapshot("old"), loader)
	results, offsets, markers := &testCache{}, &testCache{}, &testCache{}
	registry := NewRegistry(provider, results, offsets, markers)
	before := provider.Snapshot()

	// When
	err := registry.Invalidate(context.Background(), RadarrRule, RadarrRule)

	// Then
	if err != nil || provider.Snapshot().RadarrRevision == before.RadarrRevision || provider.Snapshot().SonarrRevision != before.SonarrRevision || results.clears != 0 || offsets.clears != 0 {
		t.Fatalf("err=%v before=%+v after=%+v results=%d offsets=%d", err, before, provider.Snapshot(), results.clears, offsets.clears)
	}
}

func TestRegistryInvalidate_keepsLastKnownGood_andHasNoPartialEffects_whenRefreshFails(t *testing.T) {
	// Given
	loader := &testLoader{err: errors.New("database path secret")}
	provider := NewProvider(testSnapshot("old"), loader)
	results, offsets, markers := &testCache{}, &testCache{}, &testCache{}
	registry := NewRegistry(provider, results, offsets, markers)
	before := provider.Snapshot()

	// When
	err := registry.Invalidate(context.Background(), RadarrRule, IndexerResult)

	// Then
	after := provider.Snapshot()
	if err == nil || after.RadarrRevision != before.RadarrRevision || after.SonarrRevision != before.SonarrRevision || after.SearchRevision != before.SearchRevision || results.clears != 0 || offsets.clears != 0 || len(markers.deleted) != 0 {
		t.Fatalf("err=%v snapshot=%+v results=%d offsets=%d", err, provider.Snapshot(), results.clears, offsets.clears)
	}
}

func TestRegistryInvalidate_rejectsUnknownNames_beforeEffects(t *testing.T) {
	// Given
	provider := NewStaticProvider(testSnapshot("old"))
	results, offsets, markers := &testCache{}, &testCache{}, &testCache{}
	registry := NewRegistry(provider, results, offsets, markers)

	// When
	err := registry.Invalidate(context.Background(), IndexerResult, " ")

	// Then
	if !errors.Is(err, ErrUnknownCacheName) || results.clears != 0 {
		t.Fatalf("err=%v clears=%d", err, results.clears)
	}
}

func testSnapshot(title string) sqlite.Snapshot {
	return sqlite.Snapshot{Radarr: format.Config{Format: "{title}", Rules: []format.Rule{{Token: "title", Regex: ".*", Replacement: title}}}}
}
