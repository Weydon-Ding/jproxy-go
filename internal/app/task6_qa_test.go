package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"jproxy-go/internal/api/system"
	"jproxy-go/internal/cache"
	"jproxy-go/internal/config"
	"jproxy-go/internal/proxy"
	"jproxy-go/internal/runtime"
	"jproxy-go/internal/store/sqlite"
)

func TestTask6_routeAndPublicationMeasurements(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "qa.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	seedRootConfigs(t, store)
	oldHits, newHits := 0, 0
	old := qaUpstream(&oldHits)
	newServer := qaUpstream(&newHits)
	t.Cleanup(old.Close)
	t.Cleanup(newServer.Close)
	seedURLs(t, store, old.URL)
	snapshot, err := store.FormatterSnapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	provider := runtime.NewProvider(snapshot, store)
	markers := cache.NewTTLCache[struct{}](time.Minute, 3)
	markerNames := []string{runtime.SonarrTitleSyncInterval, runtime.TMDBTitleSyncInterval, runtime.RadarrTitleSyncInterval}
	for _, name := range markerNames {
		markers.Set(name, struct{}{})
	}
	cfg := config.Config{JackettURL: old.URL, ProwlarrURL: old.URL, HTTPTimeout: time.Second, Database: config.DatabaseConfig{Enabled: true}}
	proxyServer := proxy.NewServerWithRuntime(cfg, proxy.RuntimeOptions{Provider: provider, Markers: markers})
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/authors" {
			_, _ = w.Write([]byte(`["qa"]`))
			return
		}
		_, _ = w.Write([]byte(`{"tag_name":"v1"}`))
	}))
	t.Cleanup(remote.Close)
	mux := http.NewServeMux()
	mux.Handle("/api/system/", system.NewHandler(system.Options{Store: store, Provider: provider, Registry: proxyServer.CacheRegistry(), Version: "1", VersionURL: remote.URL, AuthorURL: remote.URL + "/authors", AuthorBackupURL: remote.URL + "/backup"}))
	mux.Handle("/", proxyServer.Routes())
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	beforeHash, beforeRows := qaRows(t, server.URL)
	observed := []string{
		qaRequest(t, server.URL, http.MethodGet, "/api/system/config/version", nil, http.StatusOK, "text/plain; charset=utf-8", 1),
		qaRequest(t, server.URL, http.MethodGet, "/api/system/config/query", nil, http.StatusOK, "application/json; charset=utf-8", -1),
		qaRequest(t, server.URL, http.MethodGet, "/api/system/config/author/list", nil, http.StatusOK, "application/json; charset=utf-8", -1),
		qaRequest(t, server.URL, http.MethodPost, "/api/system/cache/clear", []byte(`{"cacheName":"indexer_result"}`), http.StatusOK, "", 0),
		qaRequest(t, server.URL, http.MethodPost, "/api/system/cache/clearAll", nil, http.StatusOK, "", 0),
		qaRequest(t, server.URL, http.MethodGet, "/radarr/jackett/api", nil, http.StatusOK, "application/xml; charset=utf-8", -1),
		qaRequest(t, server.URL, http.MethodGet, "/sonarr/prowlarr/1/api", nil, http.StatusOK, "application/xml; charset=utf-8", -1),
	}
	payload := rootPayload(t, store)
	for _, row := range payload {
		if row["key"] == "jackettUrl" || row["key"] == "prowlarrUrl" {
			row["value"] = newServer.URL
		}
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	observed = append(observed,
		qaRequest(t, server.URL, http.MethodPost, "/api/system/config/update", encoded, http.StatusOK, "", 0),
		qaRequest(t, server.URL, http.MethodGet, "/radarr/jackett/api?next=1", nil, http.StatusOK, "application/xml; charset=utf-8", -1),
		qaRequest(t, server.URL, http.MethodGet, "/sonarr/prowlarr/1/api?next=1", nil, http.StatusOK, "application/xml; charset=utf-8", -1),
	)
	canaryRequest := []byte(`{"qa-db-path":"qa-db-path","qa-dsn":"qa-dsn","qa-api-key":"qa-api-key","qa-password":"qa-password","qa-url":"qa-url","qa-regex":"qa-regex","qa-xml":"qa-xml"}`)
	observed = append(observed, qaRequest(t, server.URL, http.MethodPost, "/api/system/config/update", canaryRequest, http.StatusBadRequest, "application/json", len(`{"error":"request failed"}`)))
	afterHash, afterRows := qaRows(t, server.URL)
	deleted := 0
	for _, name := range markerNames {
		if _, ok := markers.Get(name); !ok {
			deleted++
		}
	}
	if oldHits != 2 || newHits != 2 || deleted != len(markerNames) || provider.Snapshot().JackettURL != newServer.URL || provider.Snapshot().ProwlarrURL != newServer.URL {
		t.Fatalf("upstream=%d/%d markers=%d", oldHits, newHits, deleted)
	}
	canaryLeaks := qaCanaryLeaks(t, observed)
	if canaryLeaks != 0 {
		t.Fatalf("canary leaks=%d", canaryLeaks)
	}
	t.Logf("task6_qa config_rows=%d/%d rows_before=%x rows_after=%x old_hits=%d new_hits=%d revisions=%d/%d markers_deleted=%d canary_leaks=%d", beforeRows, afterRows, beforeHash, afterHash, oldHits, newHits, provider.Snapshot().RadarrRevision, provider.Snapshot().SonarrRevision, deleted, canaryLeaks)
}

func qaUpstream(hits *int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { *hits++; _, _ = w.Write([]byte(`<rss><channel/></rss>`)) }))
}

func qaRows(t *testing.T, base string) ([32]byte, int) {
	t.Helper()
	body := qaRequest(t, base, http.MethodGet, "/api/system/config/query", nil, http.StatusOK, "application/json; charset=utf-8", -1)
	var rows []struct {
		ID  int64  `json:"id"`
		Key string `json:"key"`
	}
	if err := json.Unmarshal([]byte(body), &rows); err != nil {
		t.Fatal(err)
	}
	seen := make(map[int64]string, len(rows))
	for _, row := range rows {
		if row.ID == 0 || row.Key == "" || seen[row.ID] != "" {
			t.Fatalf("invalid config row=%+v", row)
		}
		seen[row.ID] = row.Key
	}
	if len(rows) != 20 {
		t.Fatalf("config rows=%d", len(rows))
	}
	return sha256.Sum256([]byte(body)), len(rows)
}

func qaRequest(t *testing.T, base, method, path string, body []byte, status int, contentType string, bodyLength int) string {
	t.Helper()
	request, err := http.NewRequest(method, base+path, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(response.Body)
	closeErr := response.Body.Close()
	if err != nil || closeErr != nil {
		t.Fatal(err, closeErr)
	}
	if response.StatusCode != status || response.Header.Get("Content-Type") != contentType || bodyLength >= 0 && len(data) != bodyLength {
		t.Fatalf("route %s %s status=%d type=%q length=%d", method, path, response.StatusCode, response.Header.Get("Content-Type"), len(data))
	}
	t.Logf("task6_route method=%s path=%s status=%d content_type=%q body_length=%d", method, path, response.StatusCode, response.Header.Get("Content-Type"), len(data))
	return string(data)
}

func qaCanaryLeaks(t *testing.T, observed []string) int {
	t.Helper()
	canaries := []string{"qa-db-path", "qa-dsn", "qa-api-key", "qa-password", "qa-url", "qa-regex", "qa-xml"}
	var logs strings.Builder
	logger := ConfiguredLogger(&logs)
	logger.Error("qa.failure", "path", canaries[0], "dsn", canaries[1], "apikey", canaries[2], "password", canaries[3], "url", canaries[4], "regex", canaries[5], "xml", canaries[6])
	observed = append(observed, logs.String(), string(FailureKindOf(errors.New(strings.Join(canaries, ",")))))
	leaks := 0
	for _, canary := range canaries {
		for _, text := range observed {
			if strings.Contains(text, canary) {
				leaks++
				break
			}
		}
	}
	return leaks
}
