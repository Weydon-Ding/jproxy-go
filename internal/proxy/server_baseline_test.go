package proxy

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestProxyRoutes_forwardAllFourFamilies_whenQueryHasAPIKey(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{name: "sonarr jackett", path: "/sonarr/jackett/api/v2.0/indexers/all/results/torznab"},
		{name: "sonarr prowlarr", path: "/sonarr/prowlarr/1/api"},
		{name: "radarr jackett", path: "/radarr/jackett/api/v2.0/indexers/all/results/torznab"},
		{name: "radarr prowlarr", path: "/radarr/prowlarr/1/api"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			var upstreamPath string
			var upstreamQuery url.Values
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				upstreamPath = r.URL.Path
				upstreamQuery = r.URL.Query()
				_, _ = w.Write([]byte(rssWithItems(item(tt.name))))
			}))
			defer upstream.Close()
			handler := NewServer(testConfig(upstream.URL, upstream.URL)).Routes()

			// When
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, tt.path+"?apikey=key&t=search&cat=5000", nil))

			// Then
			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body=%q", recorder.Code, recorder.Body.String())
			}
			if upstreamPath == tt.path || upstreamQuery.Get("apikey") != "key" || upstreamQuery.Get("t") != "search" || upstreamQuery.Get("cat") != "5000" {
				t.Fatalf("upstream path=%q query=%v", upstreamPath, upstreamQuery)
			}
		})
	}
}

func TestProxy_resultCacheSharesResponse_whenOnlyAPIKeyChanges(t *testing.T) {
	// Given
	calls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		_, _ = w.Write([]byte(rssWithItems(item("cached"))))
	}))
	defer upstream.Close()
	handler := NewServer(testConfig(upstream.URL, upstream.URL)).Routes()

	// When
	first := httptest.NewRecorder()
	handler.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/sonarr/jackett/api?t=search&apikey=first", nil))
	second := httptest.NewRecorder()
	handler.ServeHTTP(second, httptest.NewRequest(http.MethodGet, "/sonarr/jackett/api?t=search&apikey=second", nil))

	// Then
	if calls != 1 || first.Body.String() != second.Body.String() {
		t.Fatalf("calls=%d first=%q second=%q", calls, first.Body.String(), second.Body.String())
	}
}

func TestProxy_mergesAndTrimsExpandedResults_whenLimitIsSmallerThanCombinedFeed(t *testing.T) {
	// Given
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("q") == "Movie Title" {
			_, _ = w.Write([]byte(rssWithItems(item("three"), item("four"))))
			return
		}
		_, _ = w.Write([]byte(rssWithItems(item("one"), item("two"))))
	}))
	defer upstream.Close()
	handler := NewServer(testConfig(upstream.URL, upstream.URL)).Routes()

	// When
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/radarr/jackett/api?t=search&q=Movie+Title+2024&limit=3", nil))

	// Then
	if recorder.Code != http.StatusOK || countItems(recorder.Body.String()) != 3 || strings.Contains(recorder.Body.String(), "<title>four</title>") {
		t.Fatalf("status=%d xml=%q", recorder.Code, recorder.Body.String())
	}
}
