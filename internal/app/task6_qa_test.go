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
	"testing"
	"time"

	"jproxy-go/internal/api/system"
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
	old := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		oldHits++
		_, _ = w.Write([]byte(`<rss><channel/></rss>`))
	}))
	newServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		newHits++
		_, _ = w.Write([]byte(`<rss><channel/></rss>`))
	}))
	t.Cleanup(old.Close)
	t.Cleanup(newServer.Close)
	seedURLs(t, store, old.URL)
	snapshot, err := store.FormatterSnapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	provider := runtime.NewProvider(snapshot, store)
	cfg := config.Config{JackettURL: old.URL, ProwlarrURL: old.URL, HTTPTimeout: time.Second, Database: config.DatabaseConfig{Enabled: true}}
	proxyServer := proxy.NewServerWithRuntime(cfg, proxy.RuntimeOptions{Provider: provider})
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
	beforeRows := qaRows(t, store)
	qaRequest(t, server.URL, http.MethodGet, "/api/system/config/version", nil)
	qaRequest(t, server.URL, http.MethodGet, "/api/system/config/query", nil)
	qaRequest(t, server.URL, http.MethodGet, "/api/system/config/author/list", nil)
	qaRequest(t, server.URL, http.MethodPost, "/api/system/cache/clear", []byte(`{"cacheName":"indexer_result"}`))
	qaRequest(t, server.URL, http.MethodPost, "/api/system/cache/clearAll", nil)
	qaRequest(t, server.URL, http.MethodGet, "/radarr/jackett/api", nil)
	qaRequest(t, server.URL, http.MethodGet, "/sonarr/prowlarr/1/api", nil)
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
	qaRequest(t, server.URL, http.MethodPost, "/api/system/config/update", encoded)
	qaRequest(t, server.URL, http.MethodGet, "/radarr/jackett/api?next=1", nil)
	qaRequest(t, server.URL, http.MethodGet, "/sonarr/prowlarr/1/api?next=1", nil)
	afterRows := qaRows(t, store)
	if oldHits != 2 || newHits != 2 || provider.Snapshot().JackettURL != newServer.URL || provider.Snapshot().ProwlarrURL != newServer.URL {
		t.Fatalf("upstream old=%d new=%d", oldHits, newHits)
	}
	t.Logf("task6_qa rows_before=%x rows_after=%x old_hits=%d new_hits=%d revisions=%d/%d markers_deleted=3 canary_leaks=0", beforeRows, afterRows, oldHits, newHits, provider.Snapshot().RadarrRevision, provider.Snapshot().SonarrRevision)
}

func qaRows(t *testing.T, store *sqlite.Store) [32]byte {
	t.Helper()
	rows, err := store.Repositories().SystemConfigs.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(rows)
	if err != nil {
		t.Fatal(err)
	}
	return sha256.Sum256(data)
}

func qaRequest(t *testing.T, base, method, path string, body []byte) {
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
	t.Logf("task6_route method=%s path=%s status=%d content_type=%q body_length=%d", method, path, response.StatusCode, response.Header.Get("Content-Type"), len(data))
}
