package proxy

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"jproxy-go/internal/config"
)

func TestRoutesHealthAndNotFound(t *testing.T) {
	srv := NewServer(testConfig("http://jackett.invalid", "http://prowlarr.invalid"))

	health := httptest.NewRecorder()
	srv.Routes().ServeHTTP(health, httptest.NewRequest(http.MethodGet, "/health", nil))
	if health.Code != http.StatusOK || health.Body.String() != "ok" {
		t.Fatalf("health response = status %d body %q, want 200 ok", health.Code, health.Body.String())
	}

	missing := httptest.NewRecorder()
	srv.Routes().ServeHTTP(missing, httptest.NewRequest(http.MethodGet, "/unknown", nil))
	if missing.Code != http.StatusNotFound {
		t.Fatalf("unknown route status = %d, want 404", missing.Code)
	}
}

func TestRoutesForwardAllIndexerPrefixesAndQueryParams(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		wantBackend string
	}{
		{name: "sonarr jackett", path: "/sonarr/jackett/api/v2.0/indexers/all/results/torznab", wantBackend: "jackett"},
		{name: "sonarr prowlarr", path: "/sonarr/prowlarr/api/v1/search", wantBackend: "prowlarr"},
		{name: "sonarr prowlarr indexer api", path: "/sonarr/prowlarr/1/api", wantBackend: "prowlarr"},
		{name: "radarr jackett", path: "/radarr/jackett/api/v2.0/indexers/all/results/torznab", wantBackend: "jackett"},
		{name: "radarr prowlarr", path: "/radarr/prowlarr/api/v1/search", wantBackend: "prowlarr"},
		{name: "radarr prowlarr indexer api", path: "/radarr/prowlarr/1/api", wantBackend: "prowlarr"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotPath string
			var gotQuery url.Values
			jackett := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				gotQuery = r.URL.Query()
				_, _ = w.Write([]byte(rssWithItems(item("jackett"))))
			}))
			defer jackett.Close()
			prowlarr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				gotQuery = r.URL.Query()
				_, _ = w.Write([]byte(rssWithItems(item("prowlarr"))))
			}))
			defer prowlarr.Close()

			srv := NewServer(testConfig(jackett.URL, prowlarr.URL))
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, tt.path+"?apikey=secret&t=search&cat=5000", nil)
			srv.Routes().ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, body = %q", rec.Code, rec.Body.String())
			}
			if gotPath != strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(tt.path, "/sonarr/jackett"), "/sonarr/prowlarr"), "/radarr/jackett"), "/radarr/prowlarr") {
				t.Fatalf("upstream path = %q", gotPath)
			}
			if gotQuery.Get("apikey") != "secret" || gotQuery.Get("t") != "search" || gotQuery.Get("cat") != "5000" {
				t.Fatalf("upstream query = %v", gotQuery)
			}
			if !strings.Contains(rec.Body.String(), tt.wantBackend) {
				t.Fatalf("response body %q does not contain backend marker %q", rec.Body.String(), tt.wantBackend)
			}
		})
	}
}

func TestProwlarrIndexerPathTrimsXMLToLimit(t *testing.T) {
	var gotPath string
	var gotQuery url.Values
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.Query()
		_, _ = w.Write([]byte(rssWithItems(item("one"), item("two"), item("three"), item("four"))))
	}))
	defer upstream.Close()

	srv := NewServer(testConfig("http://jackett.invalid", upstream.URL))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/sonarr/prowlarr/1/api?apikey=secret&t=search&q=test&cat=5000&limit=2&offset=0", nil)

	srv.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%q", rec.Code, rec.Body.String())
	}
	if gotPath != "/1/api" {
		t.Fatalf("upstream path = %q, want /1/api", gotPath)
	}
	if gotQuery.Get("apikey") != "secret" || gotQuery.Get("t") != "search" || gotQuery.Get("cat") != "5000" || gotQuery.Get("q") != "test" {
		t.Fatalf("upstream query = %v", gotQuery)
	}
	if countItems(rec.Body.String()) != 2 {
		t.Fatalf("item count = %d, want 2; xml=%s", countItems(rec.Body.String()), rec.Body.String())
	}
	for _, title := range []string{"one", "two"} {
		if !strings.Contains(rec.Body.String(), "<title>"+title+"</title>") {
			t.Fatalf("response missing title %q: %s", title, rec.Body.String())
		}
	}
	if strings.Contains(rec.Body.String(), "<title>three</title>") || strings.Contains(rec.Body.String(), "<title>four</title>") {
		t.Fatalf("response was not trimmed to limit: %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "</channel></rss>") {
		t.Fatalf("response lost feed closing tags: %s", rec.Body.String())
	}
}

func TestRouteReturnsBadGatewayOnUpstreamFailure(t *testing.T) {
	srv := NewServer(testConfig("http://127.0.0.1:1", "http://127.0.0.1:1"))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/sonarr/jackett/api?t=search", nil)

	srv.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502; body=%q", rec.Code, rec.Body.String())
	}
}

func TestRouteReturnsBadGatewayAndDoesNotCacheUpstreamStatusError(t *testing.T) {
	calls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		if calls == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(rssWithItems(item("bad"))))
			return
		}
		_, _ = w.Write([]byte(rssWithItems(item("ok"))))
	}))
	defer upstream.Close()

	srv := NewServer(testConfig(upstream.URL, upstream.URL))
	handler := srv.Routes()

	first := httptest.NewRecorder()
	handler.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/sonarr/jackett/api?t=search&apikey=one", nil))
	if first.Code != http.StatusBadGateway {
		t.Fatalf("first status = %d, want 502; body=%q", first.Code, first.Body.String())
	}

	second := httptest.NewRecorder()
	handler.ServeHTTP(second, httptest.NewRequest(http.MethodGet, "/sonarr/jackett/api?t=search&apikey=two", nil))
	if second.Code != http.StatusOK {
		t.Fatalf("second status = %d, want 200; body=%q", second.Code, second.Body.String())
	}
	if calls != 2 {
		t.Fatalf("upstream calls = %d, want 2", calls)
	}
	if !strings.Contains(second.Body.String(), "<title>ok</title>") || strings.Contains(second.Body.String(), "<title>bad</title>") {
		t.Fatalf("second response should come from successful upstream, got %q", second.Body.String())
	}
}

func TestRouteAllowsUpstreamConflictResponse(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(rssWithItems(item("conflict"))))
	}))
	defer upstream.Close()

	srv := NewServer(testConfig(upstream.URL, upstream.URL))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/sonarr/jackett/api?t=search", nil)

	srv.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%q", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "<title>conflict</title>") {
		t.Fatalf("response = %q, want readable 409 body", rec.Body.String())
	}
}

func TestResultCacheIgnoresAPIKey(t *testing.T) {
	calls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		_, _ = w.Write([]byte(rssWithItems(item(fmt.Sprintf("call-%d", calls)))))
	}))
	defer upstream.Close()

	srv := NewServer(testConfig(upstream.URL, upstream.URL))
	handler := srv.Routes()

	first := httptest.NewRecorder()
	handler.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/sonarr/jackett/api?t=search&apikey=one", nil))
	second := httptest.NewRecorder()
	handler.ServeHTTP(second, httptest.NewRequest(http.MethodGet, "/sonarr/jackett/api?t=search&apikey=two", nil))

	if calls != 1 {
		t.Fatalf("upstream calls = %d, want 1", calls)
	}
	if first.Body.String() != second.Body.String() {
		t.Fatalf("cached response mismatch: first=%q second=%q", first.Body.String(), second.Body.String())
	}
}

func TestExpandedSearchMergesAndTrimsRadarrResults(t *testing.T) {
	var queries []string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		queries = append(queries, r.URL.Query().Get("q"))
		switch r.URL.Query().Get("q") {
		case "Movie Title 2024":
			_, _ = w.Write([]byte(rssWithItems(item("one"), item("two"))))
		case "Movie Title":
			_, _ = w.Write([]byte(rssWithItems(item("three"), item("four"))))
		default:
			_, _ = w.Write([]byte(emptyRSS))
		}
	}))
	defer upstream.Close()

	srv := NewServer(testConfig(upstream.URL, upstream.URL))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/radarr/jackett/api?t=search&q=Movie+Title+2024&limit=3", nil)

	srv.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%q", rec.Code, rec.Body.String())
	}
	if countItems(rec.Body.String()) != 3 {
		t.Fatalf("item count = %d, want 3; xml=%s", countItems(rec.Body.String()), rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "<title>four</title>") {
		t.Fatalf("response was not trimmed to limit: %s", rec.Body.String())
	}
	wantQueries := []string{"Movie Title 2024", "Movie Title"}
	if fmt.Sprint(queries) != fmt.Sprint(wantQueries) {
		t.Fatalf("queries = %v, want %v", queries, wantQueries)
	}
}

func testConfig(jackettURL, prowlarrURL string) config.Config {
	return config.Config{
		Addr:                  ":0",
		JackettURL:            jackettURL,
		ProwlarrURL:           prowlarrURL,
		MinCount:              6,
		IndexerResultCacheTTL: time.Minute,
		OffsetCacheTTL:        time.Minute,
		HTTPTimeout:           time.Second,
	}
}
