package app

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"jproxy-go/internal/config"
	"jproxy-go/internal/runtime"
	"jproxy-go/internal/store/sqlite"
)

func TestTask9_rootMuxMeasuresLiveTitleSyncContracts(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "task9.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	seedRootConfigs(t, store)
	entered := make(chan struct{}, 2)
	release := make(chan struct{})
	var mu sync.Mutex
	sonarrCalls, radarrCalls := 0, 0
	var sonarrKeys []string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		switch r.URL.Path {
		case "/api/v3/series":
			sonarrCalls++
			sonarrKeys = append(sonarrKeys, r.URL.Query().Get("apikey"))
		case "/api/v3/movie":
			radarrCalls++
		}
		mu.Unlock()
		switch r.URL.Path {
		case "/api/v3/series":
			entered <- struct{}{}
			<-release
			_, _ = w.Write([]byte(task9SonarrJSON))
		case "/api/v3/movie":
			_, _ = w.Write([]byte(task9RadarrJSON))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(upstream.Close)
	snapshot, err := store.UpdateSystemConfigs(ctx, task9Configs(t, store, upstream.URL, "first-key"))
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(rootHandler(config.Config{Database: config.DatabaseConfig{Enabled: true}, HTTPTimeout: time.Second}, runtime.NewProvider(snapshot, store), store))
	t.Cleanup(server.Close)

	firstDone := make(chan int, 1)
	go func() { firstDone <- task9PostStatus(server.URL, "/api/sonarr/title/sync") }()
	<-entered
	concurrentStatus := task9PostStatus(server.URL, "/api/sonarr/title/sync")
	radarrStatus := task9PostStatus(server.URL, "/api/radarr/title/sync")
	mu.Lock()
	concurrentCalls := sonarrCalls
	mu.Unlock()
	close(release)
	firstStatus := <-firstDone
	sonarrRows := task9SonarrRows(t, store)
	radarrRows := task9RadarrRows(t, store)
	tooFrequentStatus := task9PostStatus(server.URL, "/api/sonarr/title/sync")

	body, err := json.Marshal(task9ConfigPayload(task9Configs(t, store, upstream.URL, "second-key")))
	if err != nil {
		t.Fatal(err)
	}
	update := httptest.NewRecorder()
	server.Config.Handler.ServeHTTP(update, httptest.NewRequest(http.MethodPost, "/api/system/config/update", bytes.NewReader(body)))
	if update.Code != http.StatusOK {
		t.Fatalf("config update=%d", update.Code)
	}
	secondDone := make(chan int, 1)
	go func() { secondDone <- task9PostStatus(server.URL, "/api/sonarr/title/sync") }()
	<-entered
	secondStatus := <-secondDone
	tmdbStatus := task9PostStatus(server.URL, "/api/tmdb/title/sync")
	disabled := httptest.NewRecorder()
	rootHandler(config.Config{HTTPTimeout: time.Second}, runtime.NewStaticProvider(sqlite.Snapshot{}), nil).ServeHTTP(disabled, httptest.NewRequest(http.MethodPost, "/api/sonarr/title/sync", nil))

	mu.Lock()
	gotSonarrCalls, gotRadarrCalls := sonarrCalls, radarrCalls
	keys := append([]string(nil), sonarrKeys...)
	mu.Unlock()
	concurrentRejected := concurrentStatus == http.StatusBadRequest && concurrentCalls == 1
	dynamicConfig := len(keys) == 2 && keys[0] == "first-key" && keys[1] == "second-key"
	if firstStatus != http.StatusOK || !concurrentRejected || radarrStatus != http.StatusOK || gotRadarrCalls != 1 || tooFrequentStatus != http.StatusBadRequest || secondStatus != http.StatusOK || !dynamicConfig || tmdbStatus != http.StatusInternalServerError || disabled.Code != http.StatusNotFound || len(sonarrRows) != 3 || len(radarrRows) != 5 {
		t.Fatalf("statuses=%d/%d/%d/%d/%d/%d calls=%d/%d keys=%d rows=%d/%d", firstStatus, concurrentStatus, radarrStatus, tooFrequentStatus, secondStatus, tmdbStatus, gotSonarrCalls, gotRadarrCalls, len(keys), len(sonarrRows), len(radarrRows))
	}
	routeCount := 0
	for _, route := range managementRouteContracts() {
		if isTodo8Route(route) {
			routeCount++
		}
	}
	secretLeaks := taskCanaryLeaks([]string{task9PostBody(server.URL, "/api/tmdb/title/sync")})
	if routeCount != 10 || secretLeaks != 0 {
		t.Fatalf("routes=%d secret_leaks=%d", routeCount, secretLeaks)
	}
	t.Logf("task9_qa title_route_count=%d sonarr_path_template=/api/v3/series?apikey=[REDACTED] radarr_path_template=/api/v3/movie?apikey=[REDACTED] apikey_match=%t sonarr_row_count=%d sonarr_digest=%x radarr_row_count=%d radarr_digest=%x concurrent_rejected=%t concurrent_upstream_calls=%d cross_domain_concurrent=%t radarr_upstream_calls=%d too_frequent_rejected=%t dynamic_config_used=%t config_update_status=%d tmdb_status=%d db_disabled_status=%d secret_leaks=%d", routeCount, dynamicConfig, len(sonarrRows), task9Digest(t, sonarrRows), len(radarrRows), task9Digest(t, radarrRows), concurrentRejected, concurrentCalls, radarrStatus == http.StatusOK && gotRadarrCalls == 1, gotRadarrCalls, tooFrequentStatus == http.StatusBadRequest, dynamicConfig, update.Code, tmdbStatus, disabled.Code, secretLeaks)
}
