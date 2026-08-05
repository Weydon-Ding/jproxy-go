package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"jproxy-go/internal/cache"
	"jproxy-go/internal/proxy"
	"jproxy-go/internal/runtime"
	"jproxy-go/internal/store/sqlite"
)

func TestRootMux_titleQueriesPageFilterAndRemainSideEffectFree(t *testing.T) {
	store, handler, provider, _, _, _ := titleAdversarialRoot(t)
	ctx := context.Background()
	alphaOne := "alpha-one"
	betaTwo := "beta-two"
	alphaThree := "alpha-three"
	for _, row := range []sqlite.SonarrTitle{
		{ID: 1, TVDBID: 11, MainTitle: "Alpha", Title: "Alpha One", CleanTitle: &alphaOne, SeasonNumber: 1, Monitored: sqlite.Monitored, ValidStatus: sqlite.Valid},
		{ID: 2, TVDBID: 12, MainTitle: "Beta", Title: "Beta Two", CleanTitle: &betaTwo, SeasonNumber: 1, Monitored: sqlite.Monitored, ValidStatus: sqlite.Valid},
		{ID: 3, TVDBID: 13, MainTitle: "Alpha", Title: "Alpha Three", CleanTitle: &alphaThree, SeasonNumber: 1, Monitored: sqlite.Monitored, ValidStatus: sqlite.Valid},
	} {
		if err := store.Repositories().SonarrTitles.Upsert(ctx, row); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.Repositories().TMDBTitles.Upsert(ctx, sqlite.TMDBTitle{ID: 1, TVDBID: 11, Language: "en", Title: "Alpha TMDB", ValidStatus: sqlite.Valid}); err != nil {
		t.Fatal(err)
	}

	before := provider.Snapshot()
	first := titlePage(t, handler, "/api/sonarr/title/query?current=1&pageSize=1")
	second := titlePage(t, handler, "/api/sonarr/title/query?current=2&pageSize=1")
	filtered := titlePage(t, handler, "/api/sonarr/title/query?title=Alpha&pageSize=10")
	tmdb := titlePage(t, handler, "/api/tmdb/title/query?tvdbId=11")
	bad := httptest.NewRecorder()
	handler.ServeHTTP(bad, httptest.NewRequest(http.MethodGet, "/api/sonarr/title/query?current=0", nil))
	if first.Total != 3 || first.IDs[0] != 3 || second.IDs[0] != 2 || filtered.Total != 2 || tmdb.Total != 1 || bad.Code != http.StatusBadRequest || !reflect.DeepEqual(provider.Snapshot(), before) {
		t.Fatalf("first=%+v second=%+v filtered=%+v tmdb=%+v bad=%d", first, second, filtered, tmdb, bad.Code)
	}
}

func TestRootMux_titleQueriesIgnoreBlankAndTrimPaddedFilters(t *testing.T) {
	store, handler, _, _, _, _ := titleAdversarialRoot(t)
	ctx := context.Background()
	clean := "alpha"
	if err := store.Repositories().SonarrTitles.Upsert(ctx, sqlite.SonarrTitle{ID: 1, TVDBID: 1, MainTitle: "Alpha", Title: "Alpha Sonarr", CleanTitle: &clean, SeasonNumber: 1, Monitored: sqlite.Monitored, ValidStatus: sqlite.Valid}); err != nil {
		t.Fatal(err)
	}
	if err := store.Repositories().RadarrTitles.Upsert(ctx, sqlite.RadarrTitle{ID: 1, TMDBID: 1, MainTitle: "Alpha", Title: "Alpha Radarr", CleanTitle: clean, Year: 2026, Monitored: sqlite.Monitored, ValidStatus: sqlite.Valid}); err != nil {
		t.Fatal(err)
	}
	if err := store.Repositories().TMDBTitles.Upsert(ctx, sqlite.TMDBTitle{ID: 1, TVDBID: 1, Language: "en", Title: "Alpha TMDB", ValidStatus: sqlite.Valid}); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		"/api/sonarr/title/query?title=%20%20",
		"/api/radarr/title/query?title=%20%20",
		"/api/tmdb/title/query?title=%20%20",
	} {
		if page := titlePage(t, handler, path); page.Total != 1 {
			t.Fatalf("blank filter %s total=%d", path, page.Total)
		}
	}
	for _, path := range []string{
		"/api/sonarr/title/query?title=%20Alpha%20",
		"/api/radarr/title/query?title=%20Alpha%20",
		"/api/tmdb/title/query?title=%20Alpha%20",
	} {
		if page := titlePage(t, handler, path); page.Total != 1 {
			t.Fatalf("padded filter %s total=%d", path, page.Total)
		}
	}
}

func TestRootMux_titleRemovalsInvalidateOnlyTheirRuntimeDomains(t *testing.T) {
	store, handler, provider, results, offsets, markers := titleAdversarialRoot(t)
	ctx := context.Background()
	cleanTitle := "sonarr"
	if err := store.Repositories().SonarrTitles.Upsert(ctx, sqlite.SonarrTitle{ID: 1, TVDBID: 1, MainTitle: "Sonarr", Title: "Sonarr", CleanTitle: &cleanTitle, SeasonNumber: 1, Monitored: sqlite.Monitored, ValidStatus: sqlite.Valid}); err != nil {
		t.Fatal(err)
	}
	if err := store.Repositories().RadarrTitles.Upsert(ctx, sqlite.RadarrTitle{ID: 1, TMDBID: 1, MainTitle: "Radarr", Title: "Radarr", Year: 2026, Monitored: sqlite.Monitored, ValidStatus: sqlite.Valid}); err != nil {
		t.Fatal(err)
	}
	results.Set("result", "value")
	offsets.Set("offset", []int{1})
	markers.Set(runtime.SonarrTitleSyncInterval, struct{}{})
	before := provider.Snapshot()
	remove := httptest.NewRecorder()
	handler.ServeHTTP(remove, httptest.NewRequest(http.MethodPost, "/api/sonarr/title/remove", httptest.NewRequest(http.MethodPost, "/", nil).Body))
	if remove.Code != http.StatusBadRequest {
		t.Fatal(remove.Code)
	}
	remove = httptest.NewRecorder()
	handler.ServeHTTP(remove, httptest.NewRequest(http.MethodPost, "/api/sonarr/title/remove", strings.NewReader(`[1]`)))
	afterSonarr := provider.Snapshot()
	if remove.Code != http.StatusOK || afterSonarr.SonarrRevision <= before.SonarrRevision || afterSonarr.RadarrRevision != before.RadarrRevision || cacheState(results, offsets, markers) != 2 {
		t.Fatalf("sonarr status=%d before=%+v after=%+v cache=%d", remove.Code, before, afterSonarr, cacheState(results, offsets, markers))
	}
	results.Set("result", "value")
	offsets.Set("offset", []int{1})
	remove = httptest.NewRecorder()
	handler.ServeHTTP(remove, httptest.NewRequest(http.MethodPost, "/api/radarr/title/remove", strings.NewReader(`[1]`)))
	afterRadarr := provider.Snapshot()
	if remove.Code != http.StatusOK || afterRadarr.RadarrRevision <= afterSonarr.RadarrRevision || afterRadarr.SonarrRevision != afterSonarr.SonarrRevision || cacheState(results, offsets, markers) != 2 {
		t.Fatalf("radarr status=%d after_sonarr=%+v after_radarr=%+v cache=%d", remove.Code, afterSonarr, afterRadarr, cacheState(results, offsets, markers))
	}
}

type rootTitlePage struct {
	Total int64 `json:"total"`
	IDs   []int64
}

func titlePage(t *testing.T, handler http.Handler, path string) rootTitlePage {
	t.Helper()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
	var decoded struct {
		Total int64 `json:"total"`
		List  []struct {
			ID int64 `json:"id"`
		} `json:"list"`
	}
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &decoded) != nil {
		t.Fatalf("query %s status=%d body=%s", path, response.Code, response.Body.String())
	}
	result := rootTitlePage{Total: decoded.Total, IDs: make([]int64, len(decoded.List))}
	for index, row := range decoded.List {
		result.IDs[index] = row.ID
	}
	return result
}

func cacheState(results *cache.TTLCache[string], offsets *cache.TTLCache[[]int], markers *cache.TTLCache[struct{}]) int {
	count := 0
	if _, ok := results.Get("result"); ok {
		count++
	}
	if _, ok := offsets.Get("offset"); ok {
		count++
	}
	if _, ok := markers.Get(runtime.SonarrTitleSyncInterval); ok {
		count++
	}
	return count
}

func titleAdversarialRoot(t *testing.T) (*sqlite.Store, http.Handler, runtime.Provider, *cache.TTLCache[string], *cache.TTLCache[[]int], *cache.TTLCache[struct{}]) {
	t.Helper()
	store, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "title-adversarial.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	seedRootConfigs(t, store)
	snapshot, err := store.FormatterSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	provider := runtime.NewProvider(snapshot, store)
	results := cache.NewTTLCache[string](time.Minute, 8)
	offsets := cache.NewTTLCache[[]int](time.Minute, 8)
	markers := cache.NewTTLCache[struct{}](time.Minute, 3)
	proxyServer := proxy.NewServerWithRuntime(rootRouteConfig(true), proxy.RuntimeOptions{Provider: provider, Markers: markers})
	registry := runtime.NewRegistry(provider, results, offsets, markers)
	root := http.NewServeMux()
	root.Handle("/api/", managementRoutes(store, provider, registry))
	root.Handle("/", proxyServer.Routes())
	return store, root, provider, results, offsets, markers
}
