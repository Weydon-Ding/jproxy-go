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

	"jproxy-go/internal/runtime"
	"jproxy-go/internal/store/sqlite"
)

func TestRootHandler_syncsTMDBAliasesWithNextDatabaseConfiguration(t *testing.T) {
	// Given
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "tmdb-sync.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	seedRootConfigs(t, store)
	var mu sync.Mutex
	var keys []string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		keys = append(keys, r.URL.Query().Get("api_key"))
		mu.Unlock()
		_, _ = w.Write([]byte(`{"tv_results":[{"id":9,"name":"Alias"}]}`))
	}))
	t.Cleanup(upstream.Close)
	setTMDBConfig(t, store, upstream.URL, "first-key")
	seedTMDBSource(t, store, 1, 100)
	snapshot, err := store.FormatterSnapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	provider := runtime.NewProvider(snapshot, store)
	handler := rootHandler(rootRouteConfig(true), provider, store)

	// When
	first := httptest.NewRecorder()
	handler.ServeHTTP(first, httptest.NewRequest(http.MethodPost, "/api/tmdb/title/sync", nil))
	firstRevision := provider.Snapshot().SonarrSearchRevision
	seedTMDBSource(t, store, 2, 200)
	rows := task9Configs(t, store, upstream.URL, "unused")
	for index := range rows {
		if rows[index].Key == "tmdbUrl" {
			rows[index].Value = &upstream.URL
		}
		if rows[index].Key == "tmdbApikey" {
			key := "second-key"
			rows[index].Value = &key
		}
	}
	body, _ := json.Marshal(task9ConfigPayload(rows))
	update := httptest.NewRecorder()
	handler.ServeHTTP(update, httptest.NewRequest(http.MethodPost, "/api/system/config/update", bytes.NewReader(body)))
	second := httptest.NewRecorder()
	handler.ServeHTTP(second, httptest.NewRequest(http.MethodPost, "/api/tmdb/title/sync", nil))
	aliases, queryErr := store.Repositories().TMDBTitles.Page(ctx, sqlite.TMDBTitleFilter{})

	// Then
	mu.Lock()
	observed := append([]string(nil), keys...)
	mu.Unlock()
	if first.Code != http.StatusOK || update.Code != http.StatusOK || second.Code != http.StatusOK || queryErr != nil || aliases.Total != 4 || len(observed) != 4 || observed[0] != "first-key" || observed[2] != "second-key" || provider.Snapshot().SonarrSearchRevision <= firstRevision {
		t.Fatalf("status=%d/%d/%d aliases=%d keys=%v revision=%d err=%v", first.Code, update.Code, second.Code, aliases.Total, observed, provider.Snapshot().SonarrSearchRevision, queryErr)
	}
}

func setTMDBConfig(t *testing.T, store *sqlite.Store, url, key string) {
	t.Helper()
	rows := task9Configs(t, store, "http://127.0.0.1:1", "unused")
	for index := range rows {
		if rows[index].Key == "tmdbUrl" {
			rows[index].Value = &url
		}
		if rows[index].Key == "tmdbApikey" {
			rows[index].Value = &key
		}
	}
	if _, err := store.UpdateSystemConfigs(context.Background(), rows); err != nil {
		t.Fatal(err)
	}
}

func seedTMDBSource(t *testing.T, store *sqlite.Store, id, tvdbID int64) {
	t.Helper()
	cleanTitle := "main"
	err := store.Repositories().SonarrTitles.Upsert(context.Background(), sqlite.SonarrTitle{ID: sqlite.SonarrTitleID(id), TVDBID: tvdbID, SNO: 0, MainTitle: "Main", Title: "Main", CleanTitle: &cleanTitle, SeasonNumber: 1, Monitored: sqlite.Monitored, ValidStatus: sqlite.Valid})
	if err != nil {
		t.Fatal(err)
	}
}
