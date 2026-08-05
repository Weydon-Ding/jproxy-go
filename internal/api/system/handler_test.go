package system_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"jproxy-go/internal/api/system"
	"jproxy-go/internal/cache"
	"jproxy-go/internal/runtime"
	"jproxy-go/internal/store/sqlite"
)

func TestHandler_servesRawConfigAndRejectsWrongMethod(t *testing.T) {
	// Given
	store := openSeededStore(t)
	provider := runtime.NewProvider(snapshot(t, store), store)
	registry := runtime.NewRegistry(provider, cache.NewTTLCache[string](0, 1), cache.NewTTLCache[[]int](0, 1), cache.NewTTLCache[struct{}](0, 3))
	handler := system.NewHandler(system.Options{Store: store, Provider: provider, Registry: registry, Version: "0.0.0"})

	// When
	query := httptest.NewRequest(http.MethodGet, "/api/system/config/query", nil)
	queryResponse := httptest.NewRecorder()
	handler.ServeHTTP(queryResponse, query)
	wrongMethod := httptest.NewRequest(http.MethodPost, "/api/system/config/query", nil)
	wrongMethodResponse := httptest.NewRecorder()
	handler.ServeHTTP(wrongMethodResponse, wrongMethod)

	// Then
	if queryResponse.Code != http.StatusOK || queryResponse.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("query status=%d content-type=%q", queryResponse.Code, queryResponse.Header().Get("Content-Type"))
	}
	var rows []map[string]any
	if err := json.Unmarshal(queryResponse.Body.Bytes(), &rows); err != nil {
		t.Fatalf("decode query: %v", err)
	}
	if len(rows) != 20 || rows[0]["id"] != float64(1) || rows[0]["key"] != "sonarrUrl" {
		t.Fatalf("query rows=%v", rows)
	}
	if wrongMethodResponse.Code != http.StatusMethodNotAllowed {
		t.Fatalf("wrong method status=%d", wrongMethodResponse.Code)
	}
}

func TestHandler_updatesAllConfigsAtomicallyAndPublishesSnapshot(t *testing.T) {
	// Given
	store := openSeededStore(t)
	provider := runtime.NewProvider(snapshot(t, store), store)
	registry := runtime.NewRegistry(provider, cache.NewTTLCache[string](0, 1), cache.NewTTLCache[[]int](0, 1), cache.NewTTLCache[struct{}](0, 3))
	handler := system.NewHandler(system.Options{Store: store, Provider: provider, Registry: registry})
	body := completePayload(t, store)
	for _, row := range body {
		if row["key"] == "sonarrIndexerFormat" {
			row["value"] = "["
		}
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}

	// When
	request := httptest.NewRequest(http.MethodPost, "/api/system/config/update", bytes.NewReader(encoded))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	// Then
	if response.Code != http.StatusBadRequest || provider.Snapshot().Radarr.Format != "{title}" {
		t.Fatalf("status=%d snapshot=%q body=%q", response.Code, provider.Snapshot().Radarr.Format, response.Body.String())
	}
}

func TestHandler_runsRealHTTPManagementFlowWithFallbacks(t *testing.T) {
	// Given
	store := openSeededStore(t)
	provider := runtime.NewProvider(snapshot(t, store), store)
	registry := runtime.NewRegistry(provider, cache.NewTTLCache[string](0, 1), cache.NewTTLCache[[]int](0, 1), cache.NewTTLCache[struct{}](0, 3))
	remote := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/author.json" {
			writer.WriteHeader(http.StatusBadGateway)
			return
		}
		_, _ = writer.Write([]byte(`{"tag_name":"v9.9.9"}`))
	}))
	defer remote.Close()
	server := httptest.NewServer(system.NewHandler(system.Options{Store: store, Provider: provider, Registry: registry, Version: "1.0.0", VersionURL: remote.URL, AuthorURL: remote.URL}))
	defer server.Close()

	// When
	versionResponse, err := http.Get(server.URL + "/api/system/config/version")
	if err != nil {
		t.Fatal(err)
	}
	versionBody, _ := io.ReadAll(versionResponse.Body)
	_ = versionResponse.Body.Close()
	authorResponse, err := http.Get(server.URL + "/api/system/config/author/list")
	if err != nil {
		t.Fatal(err)
	}
	authorBody, _ := io.ReadAll(authorResponse.Body)
	_ = authorResponse.Body.Close()
	clearResponse, err := http.Post(server.URL+"/api/system/cache/clear", "application/json", bytes.NewBufferString(`{"cacheName":"indexer_result"}`))
	if err != nil {
		t.Fatal(err)
	}
	_ = clearResponse.Body.Close()

	// Then
	if versionResponse.StatusCode != http.StatusOK || string(versionBody) != "1.0.0 🚨" || authorResponse.StatusCode != http.StatusOK || string(authorBody) != `["LuckyPuppy514"]`+"\n" || clearResponse.StatusCode != http.StatusOK {
		t.Fatalf("version=%d/%q author=%d/%q clear=%d", versionResponse.StatusCode, versionBody, authorResponse.StatusCode, authorBody, clearResponse.StatusCode)
	}
}

func openSeededStore(t *testing.T) *sqlite.Store {
	t.Helper()
	store, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "system.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	rows := system.DefaultConfigs()
	values := make([]sqlite.SystemConfig, 0, len(rows))
	for _, row := range rows {
		value := row.Value
		values = append(values, sqlite.SystemConfig{ID: sqlite.SystemConfigID(row.ID), Key: row.Key, Value: &value, ValidStatus: sqlite.Valid})
	}
	if err := store.Repositories().SystemConfigs.UpsertBatch(context.Background(), values); err != nil {
		t.Fatal(err)
	}
	return store
}

func snapshot(t *testing.T, store *sqlite.Store) sqlite.Snapshot {
	t.Helper()
	value, err := store.FormatterSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func completePayload(t *testing.T, store *sqlite.Store) []map[string]any {
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
