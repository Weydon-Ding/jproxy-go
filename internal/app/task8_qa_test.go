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
	"jproxy-go/internal/proxy"
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
	proxyServer := rootProxyServer(t, provider, markers)
	mux := httptest.NewServer(task8RootMux(store, provider, proxyServer))
	t.Cleanup(mux.Close)
	routeCount := measuredManagementRoutes(t, mux.URL, isTodo8Route)
	if routeCount != 10 {
		t.Fatalf("route_count=%d", routeCount)
	}

	// When
	beforeRows := task8DatabaseDigest(t, store)
	postRoot(t, mux.URL, "/api/tmdb/title/save", `{"tvdbId":7,"language":"en","title":"The Movie 2026","validStatus":1}`, http.StatusOK)
	generatedRows, err := store.Repositories().TMDBTitles.FindByTVDBID(ctx, 7)
	if err != nil || len(generatedRows) != 1 || generatedRows[0].TMDBID != nil {
		t.Fatalf("generated=%+v error=%v", generatedRows, err)
	}
	postRoot(t, mux.URL, "/api/tmdb/title/save", `{"id":`+strconv.FormatInt(int64(generatedRows[0].ID), 10)+`,"tvdbId":7,"tmdbId":9,"language":"en","title":"The Movie 2026","validStatus":1}`, http.StatusOK)
	reusedRows, err := store.Repositories().TMDBTitles.FindByTVDBID(ctx, 7)
	if err != nil || len(reusedRows) != 1 || reusedRows[0].TMDBID == nil || *reusedRows[0].TMDBID != 9 {
		t.Fatalf("supplied=%+v error=%v", reusedRows, err)
	}
	beforeRemove := task8DatabaseDigest(t, store)
	postRoot(t, mux.URL, "/api/tmdb/title/remove", `[`+strconv.FormatInt(int64(generatedRows[0].ID), 10)+`]`, http.StatusOK)
	afterRemove := task8DatabaseDigest(t, store)
	removedRows := int64(len(reusedRows))
	remaining, err := store.Repositories().TMDBTitles.FindByTVDBID(ctx, 7)
	if err != nil || len(remaining) != 0 || beforeRemove == afterRemove {
		t.Fatalf("removed=%+v error=%v", remaining, err)
	}
	page := getRoot(t, mux.URL, "/api/tmdb/title/query", http.StatusOK)
	var responseFields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(page), &responseFields); err != nil || len(responseFields) != 4 || responseFields["size"] != nil {
		t.Fatalf("page=%s error=%v", page, err)
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
	if retained != 3 || canaryLeaks != 0 || !pageFieldsExact || !syncIsolated || removedRows != 1 {
		t.Fatalf("markers=%d canary_leaks=%d", retained, canaryLeaks)
	}
	t.Logf("task8_qa route_count=%d db_rows_before=%x db_rows_after_remove=%x removed_rows=%d supplied_generated_semantics=%t page_fields_exact=%t unavailable_syncs=%d markers_retained=%d canary_leaks=%d", routeCount, beforeRows, afterRemove, removedRows, len(generatedRows) == 1 && len(reusedRows) == 1, pageFieldsExact, unavailableSyncs, retained, canaryLeaks)
}

func rootProxyServer(t *testing.T, provider runtime.Provider, markers *cache.TTLCache[struct{}]) *proxy.Server {
	t.Helper()
	return proxy.NewServerWithRuntime(rootRouteConfig(true), proxy.RuntimeOptions{Provider: provider, Markers: markers})
}

func task8RootMux(store managementStore, provider runtime.Provider, proxyServer *proxy.Server) http.Handler {
	root := http.NewServeMux()
	root.Handle("/api/", managementRoutes(store, provider, proxyServer.CacheRegistry()))
	root.Handle("/", proxyServer.Routes())
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
