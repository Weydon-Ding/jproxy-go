package app

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"
	"time"

	"jproxy-go/internal/cache"
	"jproxy-go/internal/format"
	"jproxy-go/internal/runtime"
	"jproxy-go/internal/store/sqlite"
)

func TestTask8_rootSurfaceMeasurements(t *testing.T) {
	// Given
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "task8.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	seedRootConfigs(t, store)
	snapshot, err := store.FormatterSnapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	provider := runtime.NewProvider(snapshot, store)
	markers := cache.NewTTLCache[struct{}](time.Minute, 3)
	for _, name := range []string{runtime.SonarrTitleSyncInterval, runtime.RadarrTitleSyncInterval, runtime.TMDBTitleSyncInterval} {
		markers.Set(name, struct{}{})
	}
	results := cache.NewTTLCache[string](time.Minute, 4)
	offsets := cache.NewTTLCache[[]int](time.Minute, 4)
	registry := runtime.NewRegistry(provider, results, offsets, markers)
	mux := httptest.NewServer(task8RootMuxWithRegistry(store, provider, registry))
	t.Cleanup(mux.Close)
	routeCount := measuredManagementRoutes(t, mux.URL, isTodo8Route)
	if routeCount != 10 {
		t.Fatalf("route_count=%d", routeCount)
	}

	// When
	beforeRows := task8DatabaseDigest(t, store)
	postRoot(t, mux.URL, "/api/tmdb/title/save", `{"tvdbId":100,"language":"en","title":"Generated","validStatus":1}`, http.StatusOK)
	generatedRows, err := store.Repositories().TMDBTitles.FindByTVDBID(ctx, 100)
	generatedID := err == nil && len(generatedRows) == 1 && generatedRows[0].ID != 0 && generatedRows[0].TMDBID == nil
	if !generatedID {
		t.Fatalf("generated=%+v error=%v", generatedRows, err)
	}
	postRoot(t, mux.URL, "/api/tmdb/title/save", `{"id":`+strconv.FormatInt(int64(generatedRows[0].ID), 10)+`,"tvdbId":100,"tmdbId":9,"language":"en","title":"Updated","validStatus":1}`, http.StatusOK)
	updatedRows, err := store.Repositories().TMDBTitles.FindByTVDBID(ctx, 100)
	suppliedIDUpdated := err == nil && len(updatedRows) == 1 && updatedRows[0].ID == generatedRows[0].ID && updatedRows[0].Title == "Updated" && updatedRows[0].TMDBID != nil && *updatedRows[0].TMDBID == 9
	if !suppliedIDUpdated {
		t.Fatalf("supplied=%+v error=%v", updatedRows, err)
	}
	postRoot(t, mux.URL, "/api/tmdb/title/save", `{"tvdbId":200,"tmdbId":700,"language":"en","title":"Reusable","validStatus":1}`, http.StatusOK)
	postRoot(t, mux.URL, "/api/tmdb/title/save", `{"tvdbId":200,"language":"en","title":"Reuse target","validStatus":1}`, http.StatusOK)
	reusedRows, err := store.Repositories().TMDBTitles.FindByTVDBID(ctx, 200)
	tmdbIDReused := err == nil && len(reusedRows) == 2 && reusedRows[0].TMDBID != nil && reusedRows[1].TMDBID != nil && *reusedRows[0].TMDBID == 700 && *reusedRows[1].TMDBID == 700
	if !tmdbIDReused {
		t.Fatalf("reused=%+v error=%v", reusedRows, err)
	}
	postRoot(t, mux.URL, "/api/tmdb/title/save", `{"tvdbId":300,"language":"en","title":"No reuse","validStatus":1}`, http.StatusOK)
	noReuseRows, err := store.Repositories().TMDBTitles.FindByTVDBID(ctx, 300)
	tmdbIDNoReuseNull := err == nil && len(noReuseRows) == 1 && noReuseRows[0].TMDBID == nil
	if !tmdbIDNoReuseNull {
		t.Fatalf("no_reuse=%+v error=%v", noReuseRows, err)
	}
	postRoot(t, mux.URL, "/api/tmdb/title/save", `{"tvdbId":400,"language":"en","title":"The Movie","validStatus":1}`, http.StatusOK)
	beforeProjection := task8DatabaseDigest(t, store)
	page := getRoot(t, mux.URL, "/api/tmdb/title/query?tvdbId=400", http.StatusOK)
	afterProjection := task8DatabaseDigest(t, store)
	var responseFields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(page), &responseFields); err != nil || len(responseFields) != 4 || responseFields["size"] != nil {
		t.Fatalf("page=%s error=%v", page, err)
	}
	var projected struct {
		List []struct {
			CleanTitle string `json:"cleanTitle"`
		} `json:"list"`
	}
	if err := json.Unmarshal([]byte(page), &projected); err != nil || len(projected.List) != 1 || projected.List[0].CleanTitle != format.CleanTitle("The Movie", provider.Snapshot().Sonarr.CleanTitleRegex) {
		t.Fatalf("projection=%s error=%v", page, err)
	}
	projectionDBHashUnchanged := beforeProjection == afterProjection
	if !projectionDBHashUnchanged {
		t.Fatal("projection changed tmdb rows")
	}
	sonarrClean := "sonarr"
	if err := store.Repositories().SonarrTitles.Upsert(ctx, sqlite.SonarrTitle{ID: 1, TVDBID: 1, MainTitle: "Sonarr", Title: "Sonarr", CleanTitle: &sonarrClean, SeasonNumber: 1, Monitored: sqlite.Monitored, ValidStatus: sqlite.Valid}); err != nil {
		t.Fatal(err)
	}
	if err := store.Repositories().RadarrTitles.Upsert(ctx, sqlite.RadarrTitle{ID: 1, TMDBID: 1, MainTitle: "Radarr", Title: "Radarr", Year: 2026, Monitored: sqlite.Monitored, ValidStatus: sqlite.Valid}); err != nil {
		t.Fatal(err)
	}
	results.Set("result", "value")
	offsets.Set("offset", []int{1})
	beforeSonarr := provider.Snapshot()
	beforeSonarrCache := cacheState(results, offsets, markers)
	postRoot(t, mux.URL, "/api/sonarr/title/remove", `[1]`, http.StatusOK)
	afterSonarr := provider.Snapshot()
	afterSonarrCache := cacheState(results, offsets, markers)
	sonarrInvalidationIsolated := afterSonarr.SonarrRevision > beforeSonarr.SonarrRevision && afterSonarr.RadarrRevision == beforeSonarr.RadarrRevision && afterSonarrCache == 2
	if !sonarrInvalidationIsolated {
		t.Fatal("sonarr invalidation was not isolated")
	}
	results.Set("result", "value")
	offsets.Set("offset", []int{1})
	beforeRadarr := provider.Snapshot()
	beforeRadarrCache := cacheState(results, offsets, markers)
	postRoot(t, mux.URL, "/api/radarr/title/remove", `[1]`, http.StatusOK)
	afterRadarr := provider.Snapshot()
	afterRadarrCache := cacheState(results, offsets, markers)
	radarrInvalidationIsolated := afterRadarr.RadarrRevision > beforeRadarr.RadarrRevision && afterRadarr.SonarrRevision == beforeRadarr.SonarrRevision && afterRadarrCache == 2
	if !radarrInvalidationIsolated {
		t.Fatal("radarr invalidation was not isolated")
	}
	results.Set("result", "value")
	offsets.Set("offset", []int{1})
	beforeTMDB := provider.Snapshot()
	beforeTMDBCache := cacheState(results, offsets, markers)
	postRoot(t, mux.URL, "/api/tmdb/title/remove", `[`+strconv.FormatInt(int64(noReuseRows[0].ID), 10)+`]`, http.StatusOK)
	afterTMDB := provider.Snapshot()
	afterTMDBCache := cacheState(results, offsets, markers)
	tmdbInvalidationIsolated := afterTMDB.SonarrRevision > beforeTMDB.SonarrRevision && afterTMDB.RadarrRevision == beforeTMDB.RadarrRevision && afterTMDBCache == 2
	if !tmdbInvalidationIsolated {
		t.Fatal("tmdb invalidation was not isolated")
	}
	responses := make([]string, 0, 3)
	unavailableSyncs := 0
	beforeSync := provider.Snapshot()
	for _, path := range []string{"/api/sonarr/title/sync", "/api/radarr/title/sync", "/api/tmdb/title/sync"} {
		responses = append(responses, postRoot(t, mux.URL, path, "", http.StatusServiceUnavailable))
		unavailableSyncs++
	}

	// Then
	retained := 0
	for _, name := range []string{runtime.SonarrTitleSyncInterval, runtime.RadarrTitleSyncInterval, runtime.TMDBTitleSyncInterval} {
		if _, ok := markers.Get(name); ok {
			retained++
		}
	}
	canaryBodies := []string{postRoot(t, mux.URL, "/api/tmdb/title/save", `{"tvdbId":1,"language":"task-canary-token","title":"task-canary-db-path task-canary-dsn task-canary-api-key task-canary-password task-canary-url task-canary-regex task-canary-xml task-canary-upload-name","validStatus":2}`, http.StatusBadRequest)}
	canaryLeaks := taskCanaryLeaks(append(append(responses, page), canaryBodies...))
	pageFieldsExact := len(responseFields) == 4 && responseFields["current"] != nil && responseFields["pageSize"] != nil && responseFields["total"] != nil && responseFields["list"] != nil
	syncIsolated := reflect.DeepEqual(provider.Snapshot(), beforeSync)
	if retained != 3 || canaryLeaks != 0 || !pageFieldsExact || !syncIsolated {
		t.Fatalf("markers=%d canary_leaks=%d", retained, canaryLeaks)
	}
	t.Logf("task8_qa route_count=%d db_rows_before=%x generated_id=%t supplied_id_updated=%t tmdb_id_reused=%t tmdb_id_no_reuse_null=%t projection_db_hash_unchanged=%t projection_hash=%x page_fields_exact=%t sonarr_invalidation_isolated=%t radarr_invalidation_isolated=%t tmdb_invalidation_isolated=%t sonarr_revision_delta=%d radarr_revision_delta=%d tmdb_sonarr_revision_delta=%d sonarr_cache_delta=%d radarr_cache_delta=%d tmdb_cache_delta=%d unavailable_syncs=%d markers_retained=%d canary_leaks=%d", routeCount, beforeRows, generatedID, suppliedIDUpdated, tmdbIDReused, tmdbIDNoReuseNull, projectionDBHashUnchanged, afterProjection, pageFieldsExact, sonarrInvalidationIsolated, radarrInvalidationIsolated, tmdbInvalidationIsolated, afterSonarr.SonarrRevision-beforeSonarr.SonarrRevision, afterRadarr.RadarrRevision-beforeRadarr.RadarrRevision, afterTMDB.SonarrRevision-beforeTMDB.SonarrRevision, afterSonarrCache-beforeSonarrCache, afterRadarrCache-beforeRadarrCache, afterTMDBCache-beforeTMDBCache, unavailableSyncs, retained, canaryLeaks)
}

func task8RootMuxWithRegistry(store managementStore, provider runtime.Provider, registry *runtime.Registry) http.Handler {
	root := http.NewServeMux()
	root.Handle("/api/", managementRoutes(store, provider, registry))
	return root
}

func task8DatabaseDigest(t *testing.T, store *sqlite.Store) [32]byte {
	t.Helper()
	rows, err := store.Repositories().TMDBTitles.Page(context.Background(), sqlite.TMDBTitleFilter{})
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(rows.List)
	if err != nil {
		t.Fatal(err)
	}
	return sha256.Sum256(data)
}
