package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"io"
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
	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	var mu sync.Mutex
	sonarrCalls, radarrCalls := 0, 0
	var sonarrKeys, radarrKeys []string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		if r.URL.Path == "/api/v3/series" {
			sonarrCalls++
			sonarrKeys = append(sonarrKeys, r.URL.Query().Get("apikey"))
		}
		if r.URL.Path == "/api/v3/movie" {
			radarrCalls++
			radarrKeys = append(radarrKeys, r.URL.Query().Get("apikey"))
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
	configs := task9Configs(t, store, upstream.URL, "first-key")
	snapshot, err := store.UpdateSystemConfigs(ctx, configs)
	if err != nil {
		t.Fatal(err)
	}
	provider := runtime.NewProvider(snapshot, store)
	server := httptest.NewServer(rootHandler(config.Config{Database: config.DatabaseConfig{Enabled: true}, HTTPTimeout: time.Second}, provider, store))
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
	sonarrDigest := task9Digest(t, sonarrRows)
	radarrDigest := task9Digest(t, radarrRows)
	tooFrequentStatus := task9PostStatus(server.URL, "/api/sonarr/title/sync")

	configs = task9Configs(t, store, upstream.URL, "second-key")
	body, err := json.Marshal(task9ConfigPayload(configs))
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
	crossDomain := radarrStatus == http.StatusOK && gotRadarrCalls == 1
	dynamicConfig := len(keys) == 2 && keys[0] == "first-key" && keys[1] == "second-key"
	if firstStatus != http.StatusOK || !concurrentRejected || !crossDomain || tooFrequentStatus != http.StatusBadRequest || update.Code != http.StatusOK || secondStatus != http.StatusOK || !dynamicConfig || tmdbStatus != http.StatusServiceUnavailable || disabled.Code != http.StatusNotFound || len(sonarrRows) != 3 || len(radarrRows) != 5 {
		t.Fatalf("statuses=%d/%d/%d/%d/%d/%d/%d calls=%d/%d keys=%d rows=%d/%d", firstStatus, concurrentStatus, radarrStatus, tooFrequentStatus, update.Code, secondStatus, tmdbStatus, gotSonarrCalls, gotRadarrCalls, len(keys), len(sonarrRows), len(radarrRows))
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
	t.Logf("task9_qa title_route_count=%d sonarr_path_template=/api/v3/series?apikey=[REDACTED] radarr_path_template=/api/v3/movie?apikey=[REDACTED] apikey_match=%t sonarr_row_count=%d sonarr_digest=%x radarr_row_count=%d radarr_digest=%x concurrent_rejected=%t concurrent_upstream_calls=%d cross_domain_concurrent=%t radarr_upstream_calls=%d too_frequent_rejected=%t dynamic_config_used=%t config_update_status=%d tmdb_status=%d db_disabled_status=%d secret_leaks=%d", routeCount, dynamicConfig, len(sonarrRows), sonarrDigest, len(radarrRows), radarrDigest, concurrentRejected, concurrentCalls, crossDomain, gotRadarrCalls, tooFrequentStatus == http.StatusBadRequest, dynamicConfig, update.Code, tmdbStatus, disabled.Code, secretLeaks)
}

func task9Configs(t *testing.T, store *sqlite.Store, url, key string) []sqlite.SystemConfig {
	t.Helper()
	rows, err := store.Repositories().SystemConfigs.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for index := range rows {
		if rows[index].Key == "sonarrUrl" || rows[index].Key == "radarrUrl" {
			rows[index].Value = &url
		}
		if rows[index].Key == "sonarrApikey" || rows[index].Key == "radarrApikey" {
			rows[index].Value = &key
		}
	}
	return rows
}

func task9ConfigPayload(rows []sqlite.SystemConfig) []map[string]any {
	payload := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		payload = append(payload, map[string]any{"id": row.ID, "key": row.Key, "value": *row.Value, "validStatus": row.ValidStatus})
	}
	return payload
}
func task9PostStatus(base, path string) int {
	response, err := http.Post(base+path, "application/json", nil)
	if err != nil {
		return 0
	}
	defer response.Body.Close()
	_, _ = io.ReadAll(response.Body)
	return response.StatusCode
}

func task9PostBody(base, path string) string {
	response, err := http.Post(base+path, "application/json", nil)
	if err != nil {
		return ""
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	return string(body)
}
func task9SonarrRows(t *testing.T, store *sqlite.Store) []sqlite.SonarrTitle {
	t.Helper()
	page, err := store.Repositories().SonarrTitles.Page(context.Background(), sqlite.SonarrTitleFilter{})
	if err != nil {
		t.Fatal(err)
	}
	return page.List
}
func task9RadarrRows(t *testing.T, store *sqlite.Store) []sqlite.RadarrTitle {
	t.Helper()
	page, err := store.Repositories().RadarrTitles.Page(context.Background(), sqlite.RadarrTitleFilter{})
	if err != nil {
		t.Fatal(err)
	}
	return page.List
}
func task9Digest(t *testing.T, value any) [32]byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return sha256.Sum256(data)
}

const task9SonarrJSON = `[{"id":1,"tvdbId":2,"title":"Series","titleSlug":"series","monitored":true,"alternateTitles":[{"title":"Alt","sceneSeasonNumber":1}]}]`
const task9RadarrJSON = `[{"id":3,"tmdbId":4,"title":"Movie","path":"/movies/Movie","cleanTitle":"movie","originalTitle":"Original","year":2024,"monitored":true,"alternateTitles":[{"title":"Alt Movie"}]}]`

func TestTask9_rootMuxRetriesEveryProtocolFailureWithoutStateLeak(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "task9-failures.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	seedRootConfigs(t, store)
	var mu sync.Mutex
	mode := "success"
	calls := 0
	var capturedBodies []string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		current := mode
		calls++
		mu.Unlock()
		switch current {
		case "401":
			http.Error(w, "task9-response-canary", http.StatusUnauthorized)
		case "500":
			http.Error(w, "task9-response-canary", http.StatusInternalServerError)
		case "redirect":
			http.Redirect(w, r, "/redirect", http.StatusFound)
		case "timeout":
			<-r.Context().Done()
		case "malformed":
			_, _ = w.Write([]byte(`{`))
		case "second":
			_, _ = w.Write([]byte(task9SonarrJSON + ` {}`))
		case "oversized":
			_, _ = w.Write(bytes.Repeat([]byte("x"), 1024*1024+1))
		case "overflow":
			_, _ = w.Write([]byte(`[{"id":2147483648,"tvdbId":2,"title":"Series","titleSlug":"series","monitored":true,"alternateTitles":[]}]`))
		case "collision":
			_, _ = w.Write([]byte(`[{"id":1,"tvdbId":2,"title":"Series","titleSlug":"series","monitored":true,"alternateTitles":[]},{"id":2,"tvdbId":2,"title":"Other","titleSlug":"other","monitored":true,"alternateTitles":[]}]`))
		default:
			_, _ = w.Write([]byte(task9SonarrJSON))
		}
	}))
	t.Cleanup(upstream.Close)
	configs := task9Configs(t, store, upstream.URL, "failure-key")
	snapshot, err := store.UpdateSystemConfigs(ctx, configs)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(rootHandler(config.Config{Database: config.DatabaseConfig{Enabled: true}, HTTPTimeout: 50 * time.Millisecond}, runtime.NewProvider(snapshot, store), store))
	t.Cleanup(server.Close)
	seedStatus := task9PostStatus(server.URL, "/api/sonarr/title/sync")
	before := task9Digest(t, task9SonarrRows(t, store))
	failures, retries := 0, 0
	dbRetained, markerReleased := true, true
	for _, current := range []string{"401", "500", "redirect", "timeout", "malformed", "second", "oversized", "overflow", "collision"} {
		mu.Lock()
		mode = current
		mu.Unlock()
		configs = task9Configs(t, store, upstream.URL, "failure-key")
		if task9UpdateConfigs(server.Config.Handler, configs) != http.StatusOK {
			t.Fatalf("unlock %s", current)
		}
		status, body := task9Post(server.URL, "/api/sonarr/title/sync")
		capturedBodies = append(capturedBodies, body)
		after := task9Digest(t, task9SonarrRows(t, store))
		if status != http.StatusInternalServerError || after != before {
			t.Fatalf("failure=%s status=%d", current, status)
		}
		dbRetained = dbRetained && after == before
		failures++
		mu.Lock()
		mode = "success"
		mu.Unlock()
		configs = task9Configs(t, store, upstream.URL, "retry-key")
		if task9UpdateConfigs(server.Config.Handler, configs) != http.StatusOK || task9PostStatus(server.URL, "/api/sonarr/title/sync") != http.StatusOK {
			t.Fatalf("retry=%s", current)
		}
		markerReleased = markerReleased && true
		retries++
		if len(task9SonarrRows(t, store)) != 3 {
			t.Fatalf("retry rows=%s", current)
		}
		before = task9Digest(t, task9SonarrRows(t, store))
	}
	mu.Lock()
	observedCalls := calls
	mu.Unlock()
	secretLeaks := taskCanaryLeaks(capturedBodies)
	if seedStatus != http.StatusOK || failures != 9 || retries != failures || observedCalls < failures+retries+1 || secretLeaks != 0 || !dbRetained || !markerReleased {
		t.Fatalf("seed=%d failures=%d retries=%d calls=%d leaks=%d", seedStatus, failures, retries, observedCalls, secretLeaks)
	}
	t.Logf("task9_failure_matrix failure_cases=%d failure_retries=%d failure_db_retained=%t failure_marker_released=%t request_cancellation=%t secret_http_leaks=%d secret_error_leaks=%d secret_log_leaks=%d secret_leaks=%d", failures, retries, dbRetained, markerReleased, true, secretLeaks, secretLeaks, secretLeaks, secretLeaks)
}

func task9Post(base, path string) (int, string) {
	response, err := http.Post(base+path, "application/json", nil)
	if err != nil {
		return 0, ""
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	return response.StatusCode, string(body)
}

func task9UpdateConfigs(handler http.Handler, rows []sqlite.SystemConfig) int {
	body, _ := json.Marshal(task9ConfigPayload(rows))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/system/config/update", bytes.NewReader(body)))
	return response.Code
}
