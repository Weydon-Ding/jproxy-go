package proxy

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"
)

func TestProxy_returnsBadGateway_whenUpstreamTimesOut(t *testing.T) {
	// Given
	upstream := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer upstream.Close()
	cfg := testConfig(upstream.URL, upstream.URL)
	cfg.HTTPTimeout = 20 * time.Millisecond
	handler := NewServer(cfg).Routes()

	// When
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/sonarr/jackett/api?t=search", nil))

	// Then
	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("status=%d body=%q, want 502", recorder.Code, recorder.Body.String())
	}
}

func TestProxy_preservesMalformedAndEmptyXML_whenFormatterIsDisabled(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		wantCalls int
	}{
		{name: "malformed channel", body: "<rss><channel><item>", wantCalls: 1},
		{name: "empty", body: "", wantCalls: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			calls := 0
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls++
				_, _ = w.Write([]byte(tt.body))
			}))
			defer upstream.Close()
			handler := NewServer(testConfig(upstream.URL, upstream.URL)).Routes()

			// When
			first := httptest.NewRecorder()
			handler.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/radarr/jackett/api?t=search", nil))
			second := httptest.NewRecorder()
			handler.ServeHTTP(second, httptest.NewRequest(http.MethodGet, "/radarr/jackett/api?t=search", nil))

			// Then
			if first.Code != http.StatusOK || first.Body.String() != tt.body || second.Body.String() != tt.body || calls != tt.wantCalls {
				t.Fatalf("status=%d first=%q second=%q calls=%d", first.Code, first.Body.String(), second.Body.String(), calls)
			}
		})
	}
}

func TestProxy_preservesUpstreamBytes_whenSonarrFormatterIsDisabled(t *testing.T) {
	// Given
	xml := `<?xml version="1.0"?><rss><channel><item><title>Show.S02E03</title></item></channel></rss>`
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(xml))
	}))
	defer upstream.Close()
	handler := NewServer(testConfig(upstream.URL, upstream.URL)).Routes()

	// When
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/sonarr/prowlarr/api?t=search", nil))

	// Then
	if recorder.Code != http.StatusOK || recorder.Body.String() != xml {
		t.Fatalf("status=%d body=%q, want byte-identical %q", recorder.Code, recorder.Body.String(), xml)
	}
}

func TestProxy_usesDefaultPagination_whenLimitAndOffsetAreInvalid(t *testing.T) {
	// Given
	items := make([]string, 101)
	for index := range items {
		items[index] = item(strconv.Itoa(index + 1))
	}
	var upstreamQueries []url.Values
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamQueries = append(upstreamQueries, r.URL.Query())
		_, _ = w.Write([]byte(rssWithItems(items...)))
	}))
	defer upstream.Close()
	handler := NewServer(testConfig(upstream.URL, upstream.URL)).Routes()

	// When
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/radarr/prowlarr/api?t=search&q=Movie+Title+2024&limit=bad&offset=bad", nil))

	// Then
	if recorder.Code != http.StatusOK || countItems(recorder.Body.String()) != 100 {
		t.Fatalf("status=%d itemCount=%d", recorder.Code, countItems(recorder.Body.String()))
	}
	if len(upstreamQueries) != 1 || upstreamQueries[0].Get("q") != "Movie Title 2024" || upstreamQueries[0].Get("offset") != "bad" || upstreamQueries[0].Get("limit") != "bad" {
		t.Fatalf("upstream queries=%v", upstreamQueries)
	}
}
