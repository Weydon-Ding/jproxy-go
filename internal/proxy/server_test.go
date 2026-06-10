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
		{name: "radarr jackett", path: "/radarr/jackett/api/v2.0/indexers/all/results/torznab", wantBackend: "jackett"},
		{name: "radarr prowlarr", path: "/radarr/prowlarr/api/v1/search", wantBackend: "prowlarr"},
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

func TestRouteReturnsBadGatewayOnUpstreamFailure(t *testing.T) {
	srv := NewServer(testConfig("http://127.0.0.1:1", "http://127.0.0.1:1"))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/sonarr/jackett/api?t=search", nil)

	srv.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502; body=%q", rec.Code, rec.Body.String())
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

func TestOffsetCacheMatchesOriginalJproxyBoundaryBehavior(t *testing.T) {
	var queries []string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		queries = append(queries, r.URL.Query().Encode())
		switch r.URL.Query().Get("q") {
		case "Movie Title 2024":
			_, _ = w.Write([]byte(rssWithItems(item("one"))))
		case "Movie Title":
			_, _ = w.Write([]byte(rssWithItems(item("two"), item("three"))))
		default:
			_, _ = w.Write([]byte(emptyRSS))
		}
	}))
	defer upstream.Close()

	srv := NewServer(testConfig(upstream.URL, upstream.URL))
	handler := srv.Routes()

	first := httptest.NewRecorder()
	handler.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/radarr/jackett/api?t=search&q=Movie+Title+2024&limit=1&offset=0&apikey=one", nil))
	if countItems(first.Body.String()) != 1 || !strings.Contains(first.Body.String(), "one") {
		t.Fatalf("first response = %q", first.Body.String())
	}

	second := httptest.NewRecorder()
	handler.ServeHTTP(second, httptest.NewRequest(http.MethodGet, "/radarr/jackett/api?t=search&q=Movie+Title+2024&limit=1&offset=1&apikey=two", nil))
	// Original jproxy uses >= in calculateCurrentIndex, so offset equal to a cached
	// boundary still queries the same title with the same offset.
	if countItems(second.Body.String()) != 1 || !strings.Contains(second.Body.String(), "one") {
		t.Fatalf("second response = %q", second.Body.String())
	}

	if len(queries) != 2 {
		t.Fatalf("queries = %v, want two upstream calls", queries)
	}
	if !strings.Contains(queries[1], "offset=1") || !strings.Contains(queries[1], "q=Movie+Title+2024") {
		t.Fatalf("second upstream query should match original jproxy boundary behavior, got %q", queries[1])
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
