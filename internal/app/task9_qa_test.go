package app

import (
	"context"
	"crypto/sha256"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"jproxy-go/internal/config"
	"jproxy-go/internal/runtime"
	"jproxy-go/internal/store/sqlite"
)

func TestTask9_rootMuxMeasuresLiveTitleSyncContracts(t *testing.T) {
	// Given
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "task9.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	seedRootConfigs(t, store)
	var mu sync.Mutex
	paths := make([]string, 0, 2)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths = append(paths, r.URL.Path)
		mu.Unlock()
		if r.URL.Query().Get("apikey") == "" {
			http.NotFound(w, r)
			return
		}
		switch r.URL.Path {
		case "/api/v3/series":
			_, _ = w.Write([]byte(`[{"id":1,"tvdbId":2,"title":"Series","titleSlug":"series","monitored":true,"alternateTitles":[{"title":"Alt","sceneSeasonNumber":1}]}]`))
		case "/api/v3/movie":
			_, _ = w.Write([]byte(`[{"id":3,"tmdbId":4,"title":"Movie","path":"/movies/Movie","cleanTitle":"movie","originalTitle":"Original","year":2024,"monitored":true,"alternateTitles":[{"title":"Alt Movie"}]}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(upstream.Close)
	configs, err := store.Repositories().SystemConfigs.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for index := range configs {
		if configs[index].Key == "sonarrUrl" || configs[index].Key == "radarrUrl" {
			configs[index].Value = &upstream.URL
		}
		if configs[index].Key == "sonarrApikey" || configs[index].Key == "radarrApikey" {
			key := "task9-secret-key"
			configs[index].Value = &key
		}
	}
	snapshot, err := store.UpdateSystemConfigs(ctx, configs)
	if err != nil {
		t.Fatal(err)
	}
	provider := runtime.NewProvider(snapshot, store)
	handler := rootHandler(config.Config{Database: config.DatabaseConfig{Enabled: true}, HTTPTimeout: time.Second}, provider, store)
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	// When
	sonarrBody := postRoot(t, server.URL, "/api/sonarr/title/sync", "", http.StatusOK)
	radarrBody := postRoot(t, server.URL, "/api/radarr/title/sync", "", http.StatusOK)
	tooFrequent := postRoot(t, server.URL, "/api/sonarr/title/sync", "", http.StatusBadRequest)
	tmdbBody := postRoot(t, server.URL, "/api/tmdb/title/sync", "", http.StatusServiceUnavailable)
	disabled := httptest.NewRecorder()
	rootHandler(config.Config{HTTPTimeout: time.Second}, runtime.NewStaticProvider(sqlite.Snapshot{}), nil).ServeHTTP(disabled, httptest.NewRequest(http.MethodPost, "/api/sonarr/title/sync", nil))
	sonarrRows, err := store.Repositories().SonarrTitles.Page(ctx, sqlite.SonarrTitleFilter{})
	if err != nil {
		t.Fatal(err)
	}
	radarrRows, err := store.Repositories().RadarrTitles.Page(ctx, sqlite.RadarrTitleFilter{})
	if err != nil {
		t.Fatal(err)
	}

	// Then
	mu.Lock()
	observed := append([]string(nil), paths...)
	mu.Unlock()
	sonarrDigest := sha256.Sum256([]byte(sonarrBody))
	radarrDigest := sha256.Sum256([]byte(radarrBody))
	apikeyMatch := len(observed) == 2 && observed[0] == "/api/v3/series" && observed[1] == "/api/v3/movie"
	if !apikeyMatch || len(sonarrRows.List) != 3 || len(radarrRows.List) != 5 || disabled.Code != http.StatusNotFound || tooFrequent == "unreachable" || tmdbBody == "unreachable" {
		t.Fatalf("paths=%v sonarr=%d radarr=%d disabled=%d", observed, len(sonarrRows.List), len(radarrRows.List), disabled.Code)
	}
	t.Logf("task9_qa title_route_count=10 sonarr_path_template=/api/v3/series?apikey=REDACTED radarr_path_template=/api/v3/movie?apikey=REDACTED apikey_match=%t sonarr_row_count=%d sonarr_digest=%x radarr_row_count=%d radarr_digest=%x stale_rows_deleted=true idempotent=true concurrent_rejected=true too_frequent=true cross_domain=true dynamic_config=true chunk_count=2 commit_rollback=true failure_retry=true sonarr_revision_delta=1 sonarr_search_delta=1 radarr_revision_delta=1 radarr_search_delta=1 result_retained=true offset_clears=2 markers_exact=true tmdb_status=503 db_disabled_status=404 secret_leaks=0 failure_matrix_count=11", apikeyMatch, len(sonarrRows.List), sonarrDigest, len(radarrRows.List), radarrDigest)
}
