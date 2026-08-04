package proxy

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"jproxy-go/internal/config"
	"jproxy-go/internal/format"
)

func TestRadarrFormattingIsDisabledAndReturnsByteIdenticalXML(t *testing.T) {
	// Given
	xml := `<rss version="2.0"><channel><item><title>Movie.2024</title></item></channel></rss>`
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(xml))
	}))
	defer upstream.Close()

	// When
	srv := NewServer(testConfig(upstream.URL, upstream.URL))
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/radarr/jackett/api?t=search", nil))

	// Then
	if rec.Code != http.StatusOK || rec.Body.String() != xml {
		t.Fatalf("response = status %d body %q, want byte-identical %q", rec.Code, rec.Body.String(), xml)
	}
}

func TestRadarrFormattingCachesFormattedResponseAfterExpandedSearch(t *testing.T) {
	// Given
	calls := 0
	var queries []string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		queries = append(queries, r.URL.Query().Get("q"))
		_, _ = w.Write([]byte(rssWithItems(item("Movie.2024.1080p"))))
	}))
	defer upstream.Close()
	cfg := testConfig(upstream.URL, upstream.URL)
	cfg.RadarrFormatting = config.RadarrFormattingConfig{
		Enabled: true,
		Config: format.Config{
			Format: "{title} {year}",
			Rules: []format.Rule{
				{Token: "title", Regex: `^(.+?)\.\d{4}.*$`, Replacement: "$1"},
				{Token: "year", Regex: `.*?(\d{4}).*`, Replacement: "$1"},
			},
		},
	}
	srv := NewServer(cfg)
	handler := srv.Routes()

	// When
	first := httptest.NewRecorder()
	handler.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/radarr/jackett/api?t=search&q=Movie+2024&apikey=one", nil))
	second := httptest.NewRecorder()
	handler.ServeHTTP(second, httptest.NewRequest(http.MethodGet, "/radarr/jackett/api?t=search&q=Movie+2024&apikey=two", nil))

	// Then
	if calls != 2 || fmt.Sprint(queries) != "[Movie 2024 Movie]" || !strings.Contains(first.Body.String(), "<title>Movie 2024</title>") || first.Body.String() != second.Body.String() {
		t.Fatalf("calls=%d queries=%v first=%q second=%q", calls, queries, first.Body.String(), second.Body.String())
	}
}

func TestRadarrFormattingDoesNotAffectSonarrOrUpstreamErrors(t *testing.T) {
	// Given
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("error") == "true" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(rssWithItems(item("Movie.2024"))))
	}))
	defer upstream.Close()
	cfg := testConfig(upstream.URL, upstream.URL)
	cfg.RadarrFormatting = config.RadarrFormattingConfig{Enabled: true, Config: format.Config{Format: "{title}", Rules: []format.Rule{{Token: "title", Regex: `^(.+?)\.\d{4}.*$`, Replacement: "$1"}}}}
	srv := NewServer(cfg)

	// When
	sonarr := httptest.NewRecorder()
	srv.Routes().ServeHTTP(sonarr, httptest.NewRequest(http.MethodGet, "/sonarr/jackett/api?t=search", nil))
	radarrError := httptest.NewRecorder()
	srv.Routes().ServeHTTP(radarrError, httptest.NewRequest(http.MethodGet, "/radarr/jackett/api?t=search&error=true", nil))

	// Then
	if !strings.Contains(sonarr.Body.String(), "<title>Movie.2024</title>") || radarrError.Code != http.StatusBadGateway {
		t.Fatalf("sonarr=%q radarr error status=%d", sonarr.Body.String(), radarrError.Code)
	}
}
