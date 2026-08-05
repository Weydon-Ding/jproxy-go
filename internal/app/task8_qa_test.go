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
	firstTMDBID, secondTMDBID := int64(701), int64(702)
	if err := store.Repositories().TMDBTitles.Upsert(ctx, sqlite.TMDBTitle{ID: 100, TVDBID: 200, TMDBID: &firstTMDBID, Language: "en", Title: "First", ValidStatus: sqlite.Valid}); err != nil {
		t.Fatal(err)
	}
	if err := store.Repositories().TMDBTitles.Upsert(ctx, sqlite.TMDBTitle{ID: 200, TVDBID: 200, TMDBID: &secondTMDBID, Language: "en", Title: "Second", ValidStatus: sqlite.Valid}); err != nil {
		t.Fatal(err)
	}
	postRoot(t, mux.URL, "/api/tmdb/title/save", `{"tvdbId":200,"language":"en","title":"Reuse target","validStatus":1}`, http.StatusOK)
	reusedRows, err := store.Repositories().TMDBTitles.FindByTVDBID(ctx, 200)
	reuseCandidates := 2
	reusedSourceID := int64(100)
	var reuseTarget sqlite.TMDBTitle
	for _, row := range reusedRows {
		if row.Title == "Reuse target" {
			reuseTarget = row
			break
		}
	}
	tmdbIDReusedAscending := err == nil && len(reusedRows) == 3 && reuseTarget.TMDBID != nil && *reuseTarget.TMDBID == firstTMDBID && *reuseTarget.TMDBID != reusedSourceID
	if !tmdbIDReusedAscending {
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
	invalidation := newTask8InvalidationFixture(t, store, provider, results, offsets, markers, mux.URL, noReuseRows[0].ID)
	sonarrObservation := invalidation.measureSonarr(t)
	radarrObservation := invalidation.measureRadarr(t)
	tmdbObservation := invalidation.measureTMDB(t)
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
	t.Logf("task8_qa route_count=%d db_rows_before=%x generated_id=%t supplied_id_updated=%t reuse_candidates=%d reused_source_id=%d tmdb_id_reused_ascending=%t tmdb_id_no_reuse_null=%t projection_db_hash_unchanged=%t projection_hash=%x page_fields_exact=%t %s %s %s unavailable_syncs=%d markers_retained=%d canary_leaks=%d", routeCount, beforeRows, generatedID, suppliedIDUpdated, reuseCandidates, reusedSourceID, tmdbIDReusedAscending, tmdbIDNoReuseNull, projectionDBHashUnchanged, afterProjection, pageFieldsExact, sonarrObservation.String("sonarr"), radarrObservation.String("radarr"), tmdbObservation.String("tmdb"), unavailableSyncs, retained, canaryLeaks)
}

func task8RootMuxWithRegistry(store managementStore, provider runtime.Provider, registry *runtime.Registry) http.Handler {
	root := http.NewServeMux()
	root.Handle("/api/", managementRoutes(store, provider, registry, titleSyncDependencies{}))
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
