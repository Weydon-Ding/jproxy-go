package runtime

import (
	"context"
	"errors"
	"testing"
)

func TestRegistryTitleSyncGate_rejectsSameDomainWhileAttemptIsInFlight(t *testing.T) {
	// Given
	registry := newGateRegistry()
	started := make(chan TitleSyncAttempt)
	release := make(chan struct{})
	done := make(chan struct{})
	go func() {
		attempt, err := registry.BeginTitleSync(SonarrTitleSyncInterval)
		if err != nil {
			panic(err)
		}
		started <- attempt
		<-release
		registry.FinishTitleSync(attempt, false)
		close(done)
	}()
	<-started

	// When
	_, err := registry.BeginTitleSync(SonarrTitleSyncInterval)

	// Then
	if !errors.Is(err, ErrTitleSyncTooFrequent) {
		t.Fatalf("BeginTitleSync() error = %v, want too-frequent", err)
	}
	close(release)
	<-done
}

func TestRegistryTitleSyncGate_allowsDifferentDomainsAndRetriesFailures(t *testing.T) {
	// Given
	registry := newGateRegistry()
	sonarr, err := registry.BeginTitleSync(SonarrTitleSyncInterval)
	if err != nil {
		t.Fatal(err)
	}

	// When
	radarr, err := registry.BeginTitleSync(RadarrTitleSyncInterval)
	registry.FinishTitleSync(sonarr, false)
	retry, retryErr := registry.BeginTitleSync(SonarrTitleSyncInterval)

	// Then
	if err != nil || retryErr != nil {
		t.Fatalf("radarr=%v retry=%v", err, retryErr)
	}
	registry.FinishTitleSync(radarr, false)
	registry.FinishTitleSync(retry, false)
}

func TestRegistryTitleSyncGate_retainsSuccessUntilMarkerIsCleared(t *testing.T) {
	// Given
	registry := newGateRegistry()
	attempt, err := registry.BeginTitleSync(TMDBTitleSyncInterval)
	if err != nil {
		t.Fatal(err)
	}

	// When
	registry.FinishTitleSync(attempt, true)
	_, retainedErr := registry.BeginTitleSync(TMDBTitleSyncInterval)
	deleteErr := registry.DeleteMarker(TMDBTitleSyncInterval)
	retry, retryErr := registry.BeginTitleSync(TMDBTitleSyncInterval)

	// Then
	if !errors.Is(retainedErr, ErrTitleSyncTooFrequent) || deleteErr != nil || retryErr != nil {
		t.Fatalf("retained=%v delete=%v retry=%v", retainedErr, deleteErr, retryErr)
	}
	registry.FinishTitleSync(retry, false)
}

func TestRegistryTitleSyncGate_namedInvalidationClearsRetainedSuccess(t *testing.T) {
	// Given
	registry := newGateRegistry()
	attempt, err := registry.BeginTitleSync(SonarrTitleSyncInterval)
	if err != nil {
		t.Fatal(err)
	}
	registry.FinishTitleSync(attempt, true)

	// When
	invalidateErr := registry.Invalidate(context.Background(), SonarrTitleSyncInterval)
	retry, retryErr := registry.BeginTitleSync(SonarrTitleSyncInterval)

	// Then
	if invalidateErr != nil || retryErr != nil {
		t.Fatalf("invalidate=%v retry=%v", invalidateErr, retryErr)
	}
	registry.FinishTitleSync(retry, false)
}

func TestRegistryTitleSyncGate_ignoresStaleAttemptAfterNewAttemptStarts(t *testing.T) {
	// Given
	registry := newGateRegistry()
	first, err := registry.BeginTitleSync(SonarrTitleSyncInterval)
	if err != nil {
		t.Fatal(err)
	}
	registry.FinishTitleSync(first, false)
	second, err := registry.BeginTitleSync(SonarrTitleSyncInterval)
	if err != nil {
		t.Fatal(err)
	}

	// When
	registry.FinishTitleSync(first, true)
	_, err = registry.BeginTitleSync(SonarrTitleSyncInterval)

	// Then
	if !errors.Is(err, ErrTitleSyncTooFrequent) {
		t.Fatalf("stale completion admitted a parallel attempt: %v", err)
	}
	registry.FinishTitleSync(second, false)
}

func TestRegistryTitleSyncGate_configUpdateAndClearAllReleaseRetainedSuccess(t *testing.T) {
	// Given
	registry := newGateRegistry()
	attempt, err := registry.BeginTitleSync(RadarrTitleSyncInterval)
	if err != nil {
		t.Fatal(err)
	}
	registry.FinishTitleSync(attempt, true)

	// When
	registry.DeleteSystemConfigSyncMarkers()
	afterConfig, configErr := registry.BeginTitleSync(RadarrTitleSyncInterval)
	registry.FinishTitleSync(afterConfig, true)
	clearErr := registry.InvalidateAll(context.Background())
	afterClear, retryErr := registry.BeginTitleSync(RadarrTitleSyncInterval)

	// Then
	if configErr != nil || clearErr != nil || retryErr != nil {
		t.Fatalf("config=%v clear=%v retry=%v", configErr, clearErr, retryErr)
	}
	registry.FinishTitleSync(afterClear, false)
}

func TestRegistryTitleSyncGate_configUpdateDoesNotRestoreStaleSuccess(t *testing.T) {
	// Given
	registry := newGateRegistry()
	attempt, err := registry.BeginTitleSync(SonarrTitleSyncInterval)
	if err != nil {
		t.Fatal(err)
	}

	// When
	registry.DeleteSystemConfigSyncMarkers()
	registry.FinishTitleSync(attempt, true)
	retry, retryErr := registry.BeginTitleSync(SonarrTitleSyncInterval)

	// Then
	if retryErr != nil {
		t.Fatalf("stale success retained a marker: %v", retryErr)
	}
	registry.FinishTitleSync(retry, false)
}

func TestRegistryTitleSyncGate_configUpdateKeepsRunningAttemptBlockedUntilItFinishes(t *testing.T) {
	// Given
	registry := newGateRegistry()
	attempt, err := registry.BeginTitleSync(TMDBTitleSyncInterval)
	if err != nil {
		t.Fatal(err)
	}

	// When
	registry.DeleteSystemConfigSyncMarkers()
	_, inFlightErr := registry.BeginTitleSync(TMDBTitleSyncInterval)
	registry.FinishTitleSync(attempt, false)
	retry, retryErr := registry.BeginTitleSync(TMDBTitleSyncInterval)

	// Then
	if !errors.Is(inFlightErr, ErrTitleSyncTooFrequent) || retryErr != nil {
		t.Fatalf("in-flight=%v retry=%v", inFlightErr, retryErr)
	}
	registry.FinishTitleSync(retry, false)
}

func TestRegistryTitleSyncGate_rejectsUnknownMarkerWithoutEffects(t *testing.T) {
	// Given
	registry := newGateRegistry()

	// When
	_, err := registry.BeginTitleSync(IndexerResult)

	// Then
	if !errors.Is(err, ErrUnknownCacheName) {
		t.Fatalf("BeginTitleSync() error = %v, want unknown cache name", err)
	}
}

func newGateRegistry() *Registry {
	return NewRegistry(NewStaticProvider(testSnapshot("gate")), &testCache{}, &testCache{}, &testCache{})
}
