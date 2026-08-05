package app

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"jproxy-go/internal/api/system"
	"jproxy-go/internal/config"
	"jproxy-go/internal/runtime"
	"jproxy-go/internal/store/sqlite"
)

func TestRootHandler_updatesRealStoreAndNextProxyUsesPublishedFormat(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "root.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	seedRootConfigs(t, store)
	if err := store.Repositories().RadarrRules.Upsert(ctx, sqlite.RadarrRule{ID: "title", Token: "title", Regex: `.*`, Replacement: "Movie", ValidStatus: sqlite.Valid}); err != nil {
		t.Fatal(err)
	}
	initial, err := store.FormatterSnapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`<rss><channel><item><title>Movie</title></item></channel></rss>`))
	}))
	t.Cleanup(upstream.Close)
	cfg := config.Config{JackettURL: upstream.URL, ProwlarrURL: upstream.URL, HTTPTimeout: time.Second, Database: config.DatabaseConfig{Enabled: true}, IndexerResultCacheTTL: time.Minute, OffsetCacheTTL: time.Minute, ResultCacheMaxEntries: 8, OffsetCacheMaxEntries: 8}
	cfg.RadarrFormatting.Enabled = true
	provider := runtime.NewProvider(initial, store)
	server := httptest.NewServer(rootHandler(cfg, provider, store))
	t.Cleanup(server.Close)

	before := getBody(t, server.URL+"/radarr/jackett/api")
	rows := rootPayload(t, store)
	for _, row := range rows {
		if row["key"] == "radarrIndexerFormat" {
			row["value"] = "[{title}] {year}"
		}
	}
	payload, err := json.Marshal(rows)
	if err != nil {
		t.Fatal(err)
	}
	response, err := http.Post(server.URL+"/api/system/config/update", "application/json", bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	updateBody, readErr := io.ReadAll(response.Body)
	closeErr := response.Body.Close()
	if readErr != nil || closeErr != nil || response.StatusCode != http.StatusOK || len(updateBody) != 0 {
		t.Fatalf("update=%d body=%q read=%v close=%v", response.StatusCode, updateBody, readErr, closeErr)
	}
	after := getBody(t, server.URL+"/radarr/jackett/api?next=1")
	query := getBody(t, server.URL+"/api/system/config/query")
	if !strings.Contains(before, `>Movie</title>`) || !strings.Contains(after, `>[Movie]</title>`) || !strings.Contains(query, `"radarrIndexerFormat"`) {
		t.Fatalf("before=%q after=%q query=%q", before, after, query)
	}
	t.Logf("task6_e2e route_matrix=health,proxy,query,update proxy_old_hash=%d proxy_new_hash=%d revision=%d", len(before), len(after), provider.Snapshot().RadarrRevision)
}

func TestRootHandler_hidesManagementRoutesOutsideDatabaseMode(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) { _, _ = writer.Write([]byte(`ok`)) }))
	defer upstream.Close()
	cfg := config.Config{JackettURL: upstream.URL, ProwlarrURL: upstream.URL, HTTPTimeout: time.Second}
	response := httptest.NewRecorder()
	rootHandler(cfg, runtime.NewStaticProvider(sqlite.Snapshot{}), nil).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/system/config/query", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("status=%d", response.Code)
	}
}

func TestRootHandler_switchesJackettAndProwlarrAfterCompleteUpdate(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "upstream.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	seedRootConfigs(t, store)
	oldHits, newHits := 0, 0
	old := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		oldHits++
		_, _ = w.Write([]byte(`<rss><channel/></rss>`))
	}))
	replacement := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		newHits++
		_, _ = w.Write([]byte(`<rss><channel/></rss>`))
	}))
	t.Cleanup(old.Close)
	t.Cleanup(replacement.Close)
	seedURLs(t, store, old.URL)
	initial, err := store.FormatterSnapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	provider := runtime.NewProvider(initial, store)
	cfg := config.Config{JackettURL: old.URL, ProwlarrURL: old.URL, HTTPTimeout: time.Second, Database: config.DatabaseConfig{Enabled: true}}
	server := httptest.NewServer(rootHandler(cfg, provider, store))
	t.Cleanup(server.Close)
	getBody(t, server.URL+"/radarr/jackett/api")
	getBody(t, server.URL+"/sonarr/prowlarr/1/api")
	rows := rootPayload(t, store)
	for _, row := range rows {
		if row["key"] == "jackettUrl" || row["key"] == "prowlarrUrl" {
			row["value"] = replacement.URL
		}
	}
	payload, err := json.Marshal(rows)
	if err != nil {
		t.Fatal(err)
	}
	response, err := http.Post(server.URL+"/api/system/config/update", "application/json", bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("update status=%d", response.StatusCode)
	}
	getBody(t, server.URL+"/radarr/jackett/api?new=1")
	getBody(t, server.URL+"/sonarr/prowlarr/1/api?new=1")
	if oldHits != 2 || newHits != 2 || provider.Snapshot().JackettURL != replacement.URL || provider.Snapshot().ProwlarrURL != replacement.URL {
		t.Fatalf("old_hits=%d new_hits=%d snapshot=%q/%q", oldHits, newHits, provider.Snapshot().JackettURL, provider.Snapshot().ProwlarrURL)
	}
	t.Logf("task6_upstream_switch old_hits=%d new_hits=%d revision=%d", oldHits, newHits, provider.Snapshot().RadarrRevision)
}

func seedRootConfigs(t *testing.T, store *sqlite.Store) {
	t.Helper()
	rows := system.DefaultConfigs()
	values := make([]sqlite.SystemConfig, 0, len(rows))
	for _, row := range rows {
		value := row.Value
		values = append(values, sqlite.SystemConfig{ID: sqlite.SystemConfigID(row.ID), Key: row.Key, Value: &value, ValidStatus: sqlite.Valid})
	}
	if err := store.Repositories().SystemConfigs.UpsertBatch(context.Background(), values); err != nil {
		t.Fatal(err)
	}
}

func seedURLs(t *testing.T, store *sqlite.Store, value string) {
	t.Helper()
	if err := store.Repositories().SystemConfigs.UpsertBatch(context.Background(), []sqlite.SystemConfig{{ID: 10, Key: "jackettUrl", Value: &value, ValidStatus: sqlite.Valid}, {ID: 11, Key: "prowlarrUrl", Value: &value, ValidStatus: sqlite.Valid}}); err != nil {
		t.Fatal(err)
	}
}

func rootPayload(t *testing.T, store *sqlite.Store) []map[string]any {
	t.Helper()
	rows, err := store.Repositories().SystemConfigs.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	payload := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		payload = append(payload, map[string]any{"id": row.ID, "key": row.Key, "value": *row.Value, "validStatus": row.ValidStatus})
	}
	return payload
}

func getBody(t *testing.T, address string) string {
	t.Helper()
	response, err := http.Get(address)
	if err != nil {
		t.Fatal(err)
	}
	body, readErr := io.ReadAll(response.Body)
	closeErr := response.Body.Close()
	if readErr != nil || closeErr != nil || response.StatusCode != http.StatusOK {
		t.Fatalf("GET %s status=%d read=%v close=%v", address, response.StatusCode, readErr, closeErr)
	}
	return string(body)
}
