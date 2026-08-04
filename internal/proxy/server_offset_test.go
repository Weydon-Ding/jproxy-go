package proxy

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOffsetCacheMatchesOriginalJproxyBoundaryBehavior(t *testing.T) {
	// Given
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

	// When
	first := httptest.NewRecorder()
	handler.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/radarr/jackett/api?t=search&q=Movie+Title+2024&limit=1&offset=0&apikey=one", nil))
	second := httptest.NewRecorder()
	handler.ServeHTTP(second, httptest.NewRequest(http.MethodGet, "/radarr/jackett/api?t=search&q=Movie+Title+2024&limit=1&offset=1&apikey=two", nil))

	// Then
	if countItems(first.Body.String()) != 1 || !strings.Contains(first.Body.String(), "one") {
		t.Fatalf("first response = %q", first.Body.String())
	}
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
