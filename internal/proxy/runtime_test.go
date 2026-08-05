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
