package proxy

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"jproxy-go/internal/config"
	"jproxy-go/internal/format"
	store "jproxy-go/internal/store/sqlite"

	_ "modernc.org/sqlite"
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

func TestRadarrFormatting_skipsDisabledRules(t *testing.T) {
	// Given
	xml := `<rss><channel><item><title>Movie.2024</title></item></channel></rss>`
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(xml))
	}))
	defer upstream.Close()
	disabled := 0
	cfg := testConfig(upstream.URL, upstream.URL)
	cfg.RadarrFormatting = config.RadarrFormattingConfig{
		Enabled: true,
		Config: format.Config{
			Format: "{title}",
			Rules:  []format.Rule{{Token: "title", Regex: `^(.+?)\.\d{4}$`, Replacement: "Movie", ValidStatus: &disabled}},
		},
	}

	// When
	rec := httptest.NewRecorder()
	NewServer(cfg).Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/radarr/jackett/api?t=search", nil))

	// Then
	if rec.Code != http.StatusOK || rec.Body.String() != xml {
		t.Fatalf("response = status %d body %q, want unchanged %q", rec.Code, rec.Body.String(), xml)
	}
}

func TestSonarrFormattingRunsBeforeCacheAndDoesNotAffectRadarr(t *testing.T) {
	// Given
	calls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		_, _ = w.Write([]byte(rssWithItems(item("Show.S02E03.1080p"))))
	}))
	defer upstream.Close()
	cfg := testConfig(upstream.URL, upstream.URL)
	cfg.SonarrFormatting = config.SonarrFormattingConfig{
		Enabled: true,
		Config: format.SonarrConfig{
			Format: "{title} {season}",
			Rules:  []format.Rule{{Token: "title", Regex: `^(.+?)\.S\d+E\d+.*$`, Replacement: "$1"}},
		},
	}
	handler := NewServer(cfg).Routes()

	// When
	firstSonarr := httptest.NewRecorder()
	handler.ServeHTTP(firstSonarr, httptest.NewRequest(http.MethodGet, "/sonarr/jackett/api?t=search&apikey=one", nil))
	secondSonarr := httptest.NewRecorder()
	handler.ServeHTTP(secondSonarr, httptest.NewRequest(http.MethodGet, "/sonarr/jackett/api?t=search&apikey=two", nil))
	radarr := httptest.NewRecorder()
	handler.ServeHTTP(radarr, httptest.NewRequest(http.MethodGet, "/radarr/jackett/api?t=search", nil))

	// Then
	if calls != 2 || !strings.Contains(firstSonarr.Body.String(), "<title>Show</title>") || firstSonarr.Body.String() != secondSonarr.Body.String() || !strings.Contains(radarr.Body.String(), "<title>Show.S02E03.1080p</title>") {
		t.Fatalf("calls=%d sonarr first=%q second=%q radarr=%q", calls, firstSonarr.Body.String(), secondSonarr.Body.String(), radarr.Body.String())
	}
}

func TestDatabaseFormatterSnapshot_formatsRadarrAndSonarrThroughHTTPHandlers(t *testing.T) {
	// Given
	path := createProxyFormatterDatabase(t)
	snapshot, err := store.Load(context.Background(), path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	radarrUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(rssWithItems(item("Movie.2024.1080p"))))
	}))
	defer radarrUpstream.Close()
	sonarrUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(rssWithItems(item("Show.S02E03.1080p"))))
	}))
	defer sonarrUpstream.Close()
	t.Setenv("JACKETT_URL", radarrUpstream.URL)
	t.Setenv("PROWLARR_URL", sonarrUpstream.URL)
	cfg := config.Config{
		JackettURL:       radarrUpstream.URL,
		ProwlarrURL:      sonarrUpstream.URL,
		RadarrFormatting: config.RadarrFormattingConfig{Enabled: true, Config: snapshot.Radarr},
		SonarrFormatting: config.SonarrFormattingConfig{Enabled: true, Config: snapshot.Sonarr},
	}
	handler := NewServer(cfg).Routes()

	// When
	radarr := httptest.NewRecorder()
	handler.ServeHTTP(radarr, httptest.NewRequest(http.MethodGet, "/radarr/jackett/api?t=search", nil))
	sonarr := httptest.NewRecorder()
	handler.ServeHTTP(sonarr, httptest.NewRequest(http.MethodGet, "/sonarr/prowlarr/api?t=search", nil))

	// Then
	if radarr.Code != http.StatusOK || !strings.Contains(radarr.Body.String(), "<title>Movie 2024</title>") {
		t.Fatalf("Radarr response = status %d body %q", radarr.Code, radarr.Body.String())
	}
	if sonarr.Code != http.StatusOK || !strings.Contains(sonarr.Body.String(), "<title>Show</title>") {
		t.Fatalf("Sonarr response = status %d body %q", sonarr.Code, sonarr.Body.String())
	}
}

func createProxyFormatterDatabase(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "formatters.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := db.Close(); closeErr != nil {
			t.Errorf("close test database: %v", closeErr)
		}
	})
	statements := []string{
		`CREATE TABLE system_config (key TEXT, value TEXT, valid_status INTEGER)`,
		`CREATE TABLE radarr_rule (token TEXT, priority INTEGER, regex TEXT, replacement TEXT, offset INTEGER, valid_status INTEGER)`,
		`CREATE TABLE sonarr_rule (token TEXT, priority INTEGER, regex TEXT, replacement TEXT, offset INTEGER, valid_status INTEGER)`,
		`CREATE TABLE radarr_title (main_title TEXT, title TEXT, clean_title TEXT, year INTEGER, valid_status INTEGER)`,
		`CREATE TABLE sonarr_title (main_title TEXT, title TEXT, clean_title TEXT, season_number INTEGER, valid_status INTEGER)`,
		`INSERT INTO system_config VALUES ('radarrIndexerFormat', '{title} {year}', 1), ('sonarrIndexerFormat', '{title}', 1), ('cleanTitleRegex', '', 1)`,
		`INSERT INTO radarr_rule VALUES ('title', 1000, '^(.+?)\.\d{4}.*$', '$1', 0, 1), ('year', 1000, '.*?(\d{4}).*', '$1', 0, 1)`,
		`INSERT INTO sonarr_rule VALUES ('title', 1000, '^(.+?)\.S\d+E\d+.*$', '$1', 0, 1)`,
	}
	for _, statement := range statements {
		if _, execErr := db.Exec(statement); execErr != nil {
			t.Fatalf("execute test schema: %v", execErr)
		}
	}
	return path
}
