package app

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"testing"

	"jproxy-go/internal/cache"
	"jproxy-go/internal/runtime"
	"jproxy-go/internal/store/sqlite"
)

const (
	task8ResultKey = "task8-result"
	task8OffsetKey = "task8-offset"
)

type task8InvalidationFixture struct {
	provider runtime.Provider
	results  *cache.TTLCache[string]
	offsets  *cache.TTLCache[[]int]
	markers  *cache.TTLCache[struct{}]
	baseURL  string
	tmdbID   sqlite.TMDBTitleID
}

type task8InvalidationObservation struct {
	resultBefore          bool
	resultAfter           bool
	offsetBefore          bool
	offsetAfter           bool
	markerBefore          bool
	markerAfter           bool
	unrelatedMarkerBefore bool
	unrelatedMarkerAfter  bool
	relatedDelta          uint64
	unrelatedDelta        uint64
	relatedSearchDelta    uint64
	unrelatedSearchDelta  uint64
	isIsolated            bool
}

func newTask8InvalidationFixture(t *testing.T, store *sqlite.Store, provider runtime.Provider, results *cache.TTLCache[string], offsets *cache.TTLCache[[]int], markers *cache.TTLCache[struct{}], baseURL string, tmdbID sqlite.TMDBTitleID) *task8InvalidationFixture {
	t.Helper()
	sonarrClean := "sonarr"
	if err := store.Repositories().SonarrTitles.Upsert(context.Background(), sqlite.SonarrTitle{ID: 1, TVDBID: 1, MainTitle: "Sonarr", Title: "Sonarr", CleanTitle: &sonarrClean, SeasonNumber: 1, Monitored: sqlite.Monitored, ValidStatus: sqlite.Valid}); err != nil {
		t.Fatal(err)
	}
	if err := store.Repositories().RadarrTitles.Upsert(context.Background(), sqlite.RadarrTitle{ID: 1, TMDBID: 1, MainTitle: "Radarr", Title: "Radarr", Year: 2026, Monitored: sqlite.Monitored, ValidStatus: sqlite.Valid}); err != nil {
		t.Fatal(err)
	}
	return &task8InvalidationFixture{provider: provider, results: results, offsets: offsets, markers: markers, baseURL: baseURL, tmdbID: tmdbID}
}

func (f *task8InvalidationFixture) measureSonarr(t *testing.T) task8InvalidationObservation {
	t.Helper()
	f.warm(runtime.SonarrTitleSyncInterval)
	before := f.provider.Snapshot()
	observation := f.before(runtime.SonarrTitleSyncInterval, runtime.RadarrTitleSyncInterval)
	postRoot(t, f.baseURL, "/api/sonarr/title/remove", `[1]`, http.StatusOK)
	after := f.provider.Snapshot()
	return f.after(t, observation, runtime.SonarrTitleSyncInterval, runtime.RadarrTitleSyncInterval, after.SonarrRevision-before.SonarrRevision, after.RadarrRevision-before.RadarrRevision, after.SonarrSearchRevision-before.SonarrSearchRevision, after.RadarrSearchRevision-before.RadarrSearchRevision)
}

func (f *task8InvalidationFixture) measureRadarr(t *testing.T) task8InvalidationObservation {
	t.Helper()
	f.warm(runtime.RadarrTitleSyncInterval)
	before := f.provider.Snapshot()
	observation := f.before(runtime.RadarrTitleSyncInterval, runtime.SonarrTitleSyncInterval)
	postRoot(t, f.baseURL, "/api/radarr/title/remove", `[1]`, http.StatusOK)
	after := f.provider.Snapshot()
	return f.after(t, observation, runtime.RadarrTitleSyncInterval, runtime.SonarrTitleSyncInterval, after.RadarrRevision-before.RadarrRevision, after.SonarrRevision-before.SonarrRevision, after.RadarrSearchRevision-before.RadarrSearchRevision, after.SonarrSearchRevision-before.SonarrSearchRevision)
}

func (f *task8InvalidationFixture) measureTMDB(t *testing.T) task8InvalidationObservation {
	t.Helper()
	f.warm(runtime.TMDBTitleSyncInterval)
	before := f.provider.Snapshot()
	observation := f.before(runtime.TMDBTitleSyncInterval, runtime.RadarrTitleSyncInterval)
	postRoot(t, f.baseURL, "/api/tmdb/title/remove", `[`+strconv.FormatInt(int64(f.tmdbID), 10)+`]`, http.StatusOK)
	after := f.provider.Snapshot()
	return f.after(t, observation, runtime.TMDBTitleSyncInterval, runtime.RadarrTitleSyncInterval, after.SonarrRevision-before.SonarrRevision, after.RadarrRevision-before.RadarrRevision, after.SonarrSearchRevision-before.SonarrSearchRevision, after.RadarrSearchRevision-before.RadarrSearchRevision)
}

func (f *task8InvalidationFixture) warm(marker string) {
	f.results.Set(task8ResultKey, "value")
	f.offsets.Set(task8OffsetKey, []int{1})
	f.markers.Set(marker, struct{}{})
}

func (f *task8InvalidationFixture) before(marker, unrelatedMarker string) task8InvalidationObservation {
	_, resultBefore := f.results.Get(task8ResultKey)
	_, offsetBefore := f.offsets.Get(task8OffsetKey)
	_, markerBefore := f.markers.Get(marker)
	_, unrelatedMarkerBefore := f.markers.Get(unrelatedMarker)
	return task8InvalidationObservation{resultBefore: resultBefore, offsetBefore: offsetBefore, markerBefore: markerBefore, unrelatedMarkerBefore: unrelatedMarkerBefore}
}

func (f *task8InvalidationFixture) after(t *testing.T, o task8InvalidationObservation, marker, unrelatedMarker string, relatedDelta, unrelatedDelta, relatedSearchDelta, unrelatedSearchDelta uint64) task8InvalidationObservation {
	t.Helper()
	_, o.resultAfter = f.results.Get(task8ResultKey)
	_, o.offsetAfter = f.offsets.Get(task8OffsetKey)
	_, o.markerAfter = f.markers.Get(marker)
	_, o.unrelatedMarkerAfter = f.markers.Get(unrelatedMarker)
	o.relatedDelta = relatedDelta
	o.unrelatedDelta = unrelatedDelta
	o.relatedSearchDelta = relatedSearchDelta
	o.unrelatedSearchDelta = unrelatedSearchDelta
	o.isIsolated = o.resultBefore && o.resultAfter && o.offsetBefore && !o.offsetAfter && o.markerBefore && o.markerAfter && o.unrelatedMarkerBefore && o.unrelatedMarkerAfter && o.relatedDelta == 1 && o.unrelatedDelta == 0 && o.relatedSearchDelta == 1 && o.unrelatedSearchDelta == 0
	if !o.isIsolated {
		t.Fatalf("result_before=%t result_after=%t offset_before=%t offset_after=%t marker_before=%t marker_after=%t unrelated_marker_before=%t unrelated_marker_after=%t related_revision_delta=%d unrelated_revision_delta=%d related_search_revision_delta=%d unrelated_search_revision_delta=%d", o.resultBefore, o.resultAfter, o.offsetBefore, o.offsetAfter, o.markerBefore, o.markerAfter, o.unrelatedMarkerBefore, o.unrelatedMarkerAfter, o.relatedDelta, o.unrelatedDelta, o.relatedSearchDelta, o.unrelatedSearchDelta)
	}
	return o
}

func (o task8InvalidationObservation) String(domain string) string {
	return fmt.Sprintf("%s_result_before=%t %s_result_after=%t %s_offset_before=%t %s_offset_after=%t %s_marker_before=%t %s_marker_after=%t %s_unrelated_marker_before=%t %s_unrelated_marker_after=%t %s_related_revision_delta=%d %s_unrelated_revision_delta=%d %s_related_search_revision_delta=%d %s_unrelated_search_revision_delta=%d %s_invalidation_isolated=%t", domain, o.resultBefore, domain, o.resultAfter, domain, o.offsetBefore, domain, o.offsetAfter, domain, o.markerBefore, domain, o.markerAfter, domain, o.unrelatedMarkerBefore, domain, o.unrelatedMarkerAfter, domain, o.relatedDelta, domain, o.unrelatedDelta, domain, o.relatedSearchDelta, domain, o.unrelatedSearchDelta, domain, o.isIsolated)
}
