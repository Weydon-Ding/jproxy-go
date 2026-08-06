package runtime

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"jproxy-go/internal/search"
	"jproxy-go/internal/store/sqlite"
)

func TestProviderRefresh_replacesOnlySonarrCandidates_whenSonarrTitlesChange(t *testing.T) {
	// Given
	initial := testSearchSnapshot("old-sonarr", "old-radarr")
	loader := &testLoader{snapshot: testSearchSnapshot("new-sonarr", "new-radarr")}
	provider := NewProvider(initial, loader)
	before := provider.Snapshot()

	// When
	err := provider.Refresh(context.Background(), ScopeSonarrTitles)

	// Then
	after := provider.Snapshot()
	if err != nil || after.SonarrCandidates[0].Query != "new-sonarr" || !reflect.DeepEqual(after.RadarrCandidates, before.RadarrCandidates) || after.SonarrSearchRevision != before.SonarrSearchRevision+1 || after.RadarrSearchRevision != before.RadarrSearchRevision {
		t.Fatalf("err=%v before=%+v after=%+v", err, before, after)
	}
}

func TestRegistryInvalidate_refreshesSonarrCandidates_whenTMDBTitlesChange(t *testing.T) {
	// Given
	initial := testSearchSnapshot("old-sonarr", "old-radarr")
	loader := &testLoader{snapshot: testSearchSnapshot("tmdb-alias", "new-radarr")}
	provider := NewProvider(initial, loader)
	registry := NewRegistry(provider, &testCache{}, &testCache{}, &testCache{})
	before := provider.Snapshot()

	// When
	err := registry.Invalidate(context.Background(), TMDBTitleSyncInterval, SonarrSearchTitle)

	// Then
	after := provider.Snapshot()
	if err != nil || after.SonarrCandidates[0].Query != "tmdb-alias" || !reflect.DeepEqual(after.RadarrCandidates, before.RadarrCandidates) || after.SonarrSearchRevision != before.SonarrSearchRevision+1 || after.RadarrSearchRevision != before.RadarrSearchRevision {
		t.Fatalf("err=%v before=%+v after=%+v", err, before, after)
	}
}

func TestProviderRefresh_replacesOnlyRadarrCandidates_whenRadarrTitlesChange(t *testing.T) {
	// Given
	initial := testSearchSnapshot("old-sonarr", "old-radarr")
	loader := &testLoader{snapshot: testSearchSnapshot("new-sonarr", "new-radarr")}
	provider := NewProvider(initial, loader)
	before := provider.Snapshot()

	// When
	err := provider.Refresh(context.Background(), ScopeRadarrTitles)

	// Then
	after := provider.Snapshot()
	if err != nil || after.RadarrCandidates[0].Query != "new-radarr" || !reflect.DeepEqual(after.SonarrCandidates, before.SonarrCandidates) || after.RadarrSearchRevision != before.RadarrSearchRevision+1 || after.SonarrSearchRevision != before.SonarrSearchRevision {
		t.Fatalf("err=%v before=%+v after=%+v", err, before, after)
	}
}

func TestProviderSnapshot_returnsIndependentSearchCandidateCopies(t *testing.T) {
	// Given
	provider := NewStaticProvider(testSearchSnapshot("sonarr", "radarr"))
	first := provider.Snapshot()

	// When
	first.SonarrCandidates[0].Query = "mutated"
	first.RadarrCandidates[0].Query = "mutated"

	// Then
	second := provider.Snapshot()
	if second.SonarrCandidates[0].Query != "sonarr" || second.RadarrCandidates[0].Query != "radarr" {
		t.Fatalf("published snapshot was mutated: %+v", second)
	}
}

func TestNewProvider_doesNotRetainMutableCandidateInput(t *testing.T) {
	// Given
	initial := testSearchSnapshot("sonarr", "radarr")
	provider := NewStaticProvider(initial)

	// When
	initial.SonarrCandidates[0].Query = "mutated"
	initial.RadarrCandidates[0].Query = "mutated"

	// Then
	snapshot := provider.Snapshot()
	if snapshot.SonarrCandidates[0].Query != "sonarr" || snapshot.RadarrCandidates[0].Query != "radarr" {
		t.Fatalf("provider retained mutable input: %+v", snapshot)
	}
}

func TestProviderRefresh_retainsSearchCandidates_whenLoadFailsOrIsCancelled(t *testing.T) {
	// Given
	initial := testSearchSnapshot("old-sonarr", "old-radarr")
	loader := &testLoader{err: errors.New("malformed")}
	provider := NewProvider(initial, loader)
	before := provider.Snapshot()

	// When
	failure := provider.Refresh(context.Background(), ScopeSonarrTitles)
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	cancellation := provider.Refresh(cancelled, ScopeRadarrTitles)

	// Then
	after := provider.Snapshot()
	if !errors.Is(failure, ErrSnapshotRefresh) || !errors.Is(cancellation, context.Canceled) || !reflect.DeepEqual(after.SonarrCandidates, before.SonarrCandidates) || !reflect.DeepEqual(after.RadarrCandidates, before.RadarrCandidates) {
		t.Fatalf("failure=%v cancellation=%v before=%+v after=%+v", failure, cancellation, before, after)
	}
}

func testSearchSnapshot(sonarr, radarr string) sqlite.Snapshot {
	return sqlite.Snapshot{
		SonarrCandidates: []search.Candidate{{Query: sonarr}},
		RadarrCandidates: []search.Candidate{{Query: radarr}},
	}
}
