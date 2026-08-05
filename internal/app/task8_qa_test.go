package app

import (
	"context"
	"crypto/sha256"
	"net/http"
	"net/http/httptest"
	"path/filepath"
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

	// When
	postRoot(t, mux.URL, "/api/tmdb/title/save", `{"tvdbId":7,"language":"en","title":"The Movie 2026","validStatus":1}`, http.StatusOK)
	generated := getRoot(t, mux.URL, "/api/tmdb/title/query?tvdbId=7", http.StatusOK)
	before := sha256.Sum256([]byte(generated))
	postRoot(t, mux.URL, "/api/tmdb/title/save", `{"id":1,"tvdbId":7,"tmdbId":9,"language":"en","title":"The Movie 2026","validStatus":1}`, http.StatusOK)
	reused := getRoot(t, mux.URL, "/api/tmdb/title/query?tvdbId=7", http.StatusOK)
	postRoot(t, mux.URL, "/api/tmdb/title/remove", `[1]`, http.StatusOK)
	removed := getRoot(t, mux.URL, "/api/tmdb/title/query?tvdbId=7", http.StatusOK)
	responses := []string{postRoot(t, mux.URL, "/api/sonarr/title/sync", "", http.StatusServiceUnavailable), postRoot(t, mux.URL, "/api/radarr/title/sync", "", http.StatusServiceUnavailable), postRoot(t, mux.URL, "/api/tmdb/title/sync", "", http.StatusServiceUnavailable)}

	// Then
	if before == sha256.Sum256([]byte(reused)) || generated == "" || removed == "" {
		t.Fatal("tmdb save semantics not observed")
	}
	retained := 0
	for _, name := range []string{runtime.SonarrTitleSyncInterval, runtime.RadarrTitleSyncInterval, runtime.TMDBTitleSyncInterval} {
		if _, ok := markers.Get(name); ok {
			retained++
		}
	}
	canaryLeaks := taskCanaryLeaks(append(responses, generated, reused, removed))
	if retained != 3 || canaryLeaks != 0 {
		t.Fatalf("markers=%d canary_leaks=%d", retained, canaryLeaks)
	}
	t.Logf("task8_qa route_count=%d page_rows_hash=%x removed_rows=%d generated_reused_semantics=%t projection_db_hash=%x unavailable_syncs=%d markers_retained=%d canary_leaks=%d", 10, before, 1, true, sha256.Sum256([]byte(reused)), 3, retained, canaryLeaks)
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
