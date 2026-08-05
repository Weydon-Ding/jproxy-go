package proxy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"jproxy-go/internal/format"
	"jproxy-go/internal/runtime"
	"jproxy-go/internal/store/sqlite"
)

type liveLoader struct{ snapshot sqlite.Snapshot }

func (l *liveLoader) LoadFormatterSnapshot(context.Context) (sqlite.Snapshot, error) {
	return l.snapshot, nil
}

func TestServer_refreshesOnlyRadarrResultCache_whenRadarrRuleIsInvalidated(t *testing.T) {
	// Given
	calls := map[string]int{}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls[r.URL.Path]++
		_, _ = w.Write([]byte(rssWithItems(item("Movie.2024"))))
	}))
	defer upstream.Close()
	loader := &liveLoader{snapshot: runtimeSnapshot("old", "sonarr")}
	provider := runtime.NewProvider(loader.snapshot, loader)
	cfg := testConfig(upstream.URL, upstream.URL)
	cfg.IndexerResultCacheTTL = time.Minute
	cfg.OffsetCacheTTL = time.Minute
	cfg.ResultCacheMaxEntries = 10
	cfg.OffsetCacheMaxEntries = 10
	cfg.RadarrFormatting.Enabled = true
	cfg.SonarrFormatting.Enabled = true
	handler := NewServerWithRuntime(cfg, RuntimeOptions{Provider: provider})
	warm(t, handler, "/radarr/jackett/api?t=search")
	warm(t, handler, "/sonarr/jackett/api?t=search")
	loader.snapshot = runtimeSnapshot("new", "sonarr")

	// When
	if err := handler.CacheRegistry().Invalidate(context.Background(), runtime.RadarrRule); err != nil {
		t.Fatalf("Invalidate() error = %v", err)
	}
	radarr := warm(t, handler, "/radarr/jackett/api?t=search")
	sonarr := warm(t, handler, "/sonarr/jackett/api?t=search")

	// Then
	if !contains(radarr, "new") || !contains(sonarr, "sonarr") || calls["/api"] != 3 {
		t.Fatalf("radarr=%q sonarr=%q calls=%v", radarr, sonarr, calls)
	}
}

func TestServer_keepsDisabledFormatterBytesIdentical_afterInvalidation(t *testing.T) {
	// Given
	const xml = `<rss><channel><item><title>Movie.2024</title></item></channel></rss>`
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(xml)) }))
	defer upstream.Close()
	loader := &liveLoader{snapshot: runtimeSnapshot("new", "sonarr")}
	provider := runtime.NewProvider(loader.snapshot, loader)
	cfg := testConfig(upstream.URL, upstream.URL)
	cfg.IndexerResultCacheTTL = time.Minute
	cfg.OffsetCacheTTL = time.Minute
	cfg.ResultCacheMaxEntries = 10
	cfg.OffsetCacheMaxEntries = 10
	handler := NewServerWithRuntime(cfg, RuntimeOptions{Provider: provider})

	// When
	first := warm(t, handler, "/radarr/jackett/api?t=search")
	if err := handler.CacheRegistry().Invalidate(context.Background(), runtime.RadarrRule); err != nil {
		t.Fatalf("Invalidate() error = %v", err)
	}
	second := warm(t, handler, "/radarr/jackett/api?t=search")

	// Then
	if first != xml || second != xml {
		t.Fatalf("first=%q second=%q", first, second)
	}
}

func TestServer_staticProviderKeepsDisabledBytesAndCache_afterInvalidation(t *testing.T) {
	// Given
	const xml = `<rss><channel><item><title>Movie.2024</title></item></channel></rss>`
	calls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		_, _ = w.Write([]byte(xml))
	}))
	defer upstream.Close()
	cfg := testConfig(upstream.URL, upstream.URL)
	cfg.ResultCacheMaxEntries = 10
	server := NewServerWithRuntime(cfg, RuntimeOptions{Provider: runtime.NewStaticProvider(sqlite.Snapshot{})})

	// When
	first := warm(t, server, "/radarr/jackett/api?t=search")
	if err := server.CacheRegistry().Invalidate(context.Background(), runtime.RadarrRule); err != nil {
		t.Fatalf("Invalidate() error = %v", err)
	}
	second := warm(t, server, "/radarr/jackett/api?t=search")

	// Then
	if first != xml || second != xml || calls != 1 {
		t.Fatalf("first=%q second=%q calls=%d", first, second, calls)
	}
}

func TestServer_keepsRadarrOffsetCache_whenSonarrTitlesAreInvalidated(t *testing.T) {
	// Given
	var radarrQueries []string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		radarrQueries = append(radarrQueries, r.URL.Query().Encode())
		_, _ = w.Write([]byte(rssWithItems(item(r.URL.Query().Get("q")))))
	}))
	defer upstream.Close()
	loader := &liveLoader{snapshot: runtimeSnapshot("radarr", "sonarr")}
	provider := runtime.NewProvider(loader.snapshot, loader)
	cfg := testConfig(upstream.URL, upstream.URL)
	cfg.OffsetCacheTTL = time.Minute
	cfg.OffsetCacheMaxEntries = 10
	server := NewServerWithRuntime(cfg, RuntimeOptions{Provider: provider})
	path := "/radarr/jackett/api?t=search&q=Movie+Title+2024&limit=1&offset="
	warm(t, server, path+"0")
	if err := server.CacheRegistry().Invalidate(context.Background(), runtime.SonarrSearchTitle); err != nil {
		t.Fatalf("Invalidate() error = %v", err)
	}

	// When
	warm(t, server, path+"1")

	// Then
	if len(radarrQueries) != 2 || !strings.Contains(radarrQueries[1], "offset=1") || !strings.Contains(radarrQueries[1], "q=Movie+Title+2024") {
		t.Fatalf("queries=%v", radarrQueries)
	}
	t.Logf("task5_http_qa per_kind_offset=true invalidation=%s upstream_count=%d second_query_has_original_offset=%t", runtime.SonarrSearchTitle, len(radarrQueries), strings.Contains(radarrQueries[1], "offset=1"))
}

func runtimeSnapshot(radarr, sonarr string) sqlite.Snapshot {
	return sqlite.Snapshot{Radarr: format.Config{Format: "{title}", Rules: []format.Rule{{Token: "title", Regex: ".*", Replacement: radarr}}}, Sonarr: format.SonarrConfig{Format: "{title}", Rules: []format.Rule{{Token: "title", Regex: ".*", Replacement: sonarr}}}}
}

func warm(t *testing.T, handler *Server, path string) string {
	t.Helper()
	recorder := httptest.NewRecorder()
	handler.Routes().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%q", recorder.Code, recorder.Body.String())
	}
	return recorder.Body.String()
}

func contains(value, part string) bool { return strings.Contains(value, part) }
