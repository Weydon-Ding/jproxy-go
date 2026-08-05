package titlesync

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"jproxy-go/internal/store/sqlite"
)

func TestSonarrService_replacesStaleRowsAndIsIdempotent_whenUpstreamSucceeds(t *testing.T) {
	// Given
	store := openServiceStore(t)
	defer store.Close()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`[{"id":7,"tvdbId":12,"title":"Main","titleSlug":"slug","monitored":true,"alternateTitles":[{"title":"Alt","sceneSeasonNumber":2}]}]`))
	}))
	defer server.Close()
	seedSonarrConfig(t, store, server.URL, "secret", "")
	old := "old"
	if err := store.Repositories().SonarrTitles.Upsert(context.Background(), sqlite.SonarrTitle{ID: 1, TVDBID: 1, MainTitle: "old", Title: "old", CleanTitle: &old, Monitored: sqlite.Monitored, ValidStatus: sqlite.Valid}); err != nil {
		t.Fatal(err)
	}
	service := NewSonarrService(SonarrServiceDependencies{Config: NewConfigSource(store.Repositories().SystemConfigs), Client: NewSonarrClient(newRequestClient(nil, time.Second)), Repository: store.Repositories().SonarrTitles})

	// When
	err := service.Sync(context.Background())

	// Then
	if err != nil {
		t.Fatal(err)
	}
	page, err := store.Repositories().SonarrTitles.Page(context.Background(), sqlite.SonarrTitleFilter{Page: sqlite.PageInput{Size: 200}})
	if err != nil || len(page.List) != 3 || page.List[2].ID != 120 || page.List[0].Title != "Alt" {
		t.Fatalf("rows=%#v err=%v", page.List, err)
	}
	if err := service.Sync(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestRadarrService_clearsRowsAndUsesNewConfig_whenNextSyncRuns(t *testing.T) {
	// Given
	store := openServiceStore(t)
	defer store.Close()
	first := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) { _, _ = writer.Write([]byte(`[]`)) }))
	defer first.Close()
	second := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Query().Get("apikey") != "new-key" {
			t.Errorf("unexpected key")
		}
		_, _ = writer.Write([]byte(`[{"id":7,"tmdbId":12,"title":"Main","path":"/movies/Main (2024)","cleanTitle":"main","originalTitle":"Original","year":2024,"monitored":false,"alternateTitles":[]}]`))
	}))
	defer second.Close()
	seedRadarrConfig(t, store, first.URL, "old-key", "")
	service := NewRadarrService(RadarrServiceDependencies{Config: NewConfigSource(store.Repositories().SystemConfigs), Client: NewRadarrClient(newRequestClient(nil, time.Second)), Repository: store.Repositories().RadarrTitles})

	// When
	if err := service.Sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	seedRadarrConfig(t, store, second.URL, "new-key", "")
	err := service.Sync(context.Background())

	// Then
	if err != nil {
		t.Fatal(err)
	}
	page, pageErr := store.Repositories().RadarrTitles.Page(context.Background(), sqlite.RadarrTitleFilter{Page: sqlite.PageInput{Size: 200}})
	if pageErr != nil || len(page.List) != 4 || page.List[3].ID != 120 {
		t.Fatalf("rows=%#v err=%v", page.List, pageErr)
	}
}

func TestSonarrService_preservesOldRowsAndSkipsReplace_whenPrecommitFailure(t *testing.T) {
	for _, response := range []string{"status", "malformed", "multiple", "overflow"} {
		t.Run(response, func(t *testing.T) {
			store := openServiceStore(t)
			defer store.Close()
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				switch response {
				case "status":
					writer.WriteHeader(http.StatusInternalServerError)
				case "malformed":
					_, _ = writer.Write([]byte(`[`))
				case "multiple":
					_, _ = writer.Write([]byte(`[] []`))
				default:
					_, _ = writer.Write([]byte(`[{"id":1,"tvdbId":2147483647,"title":"a","titleSlug":"b","monitored":true,"alternateTitles":[]}]`))
				}
			}))
			defer server.Close()
			seedSonarrConfig(t, store, server.URL, "canary-key", "")
			old := "old"
			if err := store.Repositories().SonarrTitles.Upsert(context.Background(), sqlite.SonarrTitle{ID: 1, TVDBID: 1, MainTitle: "old", Title: "old", CleanTitle: &old, Monitored: sqlite.Monitored, ValidStatus: sqlite.Valid}); err != nil {
				t.Fatal(err)
			}
			service := NewSonarrService(SonarrServiceDependencies{Config: NewConfigSource(store.Repositories().SystemConfigs), Client: NewSonarrClient(newRequestClient(nil, time.Second)), Repository: store.Repositories().SonarrTitles})
			if err := service.Sync(context.Background()); err == nil || strings.Contains(err.Error(), "canary-key") {
				t.Fatalf("want redacted failure: %v", err)
			}
			if _, err := store.Repositories().SonarrTitles.Get(context.Background(), 1); err != nil {
				t.Fatalf("old row lost: %v", err)
			}
		})
	}
}

func TestRadarrService_replacesMoreThanOneChunk_whenUpstreamHasManyMovies(t *testing.T) {
	// Given
	store := openServiceStore(t)
	defer store.Close()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(radarrMoviesJSON(51)))
	}))
	defer server.Close()
	seedRadarrConfig(t, store, server.URL, "secret", "")
	service := NewRadarrService(RadarrServiceDependencies{Config: NewConfigSource(store.Repositories().SystemConfigs), Client: NewRadarrClient(newRequestClient(nil, time.Second)), Repository: store.Repositories().RadarrTitles})

	// When
	err := service.Sync(context.Background())

	// Then
	if err != nil {
		t.Fatal(err)
	}
	page, err := store.Repositories().RadarrTitles.Page(context.Background(), sqlite.RadarrTitleFilter{Page: sqlite.PageInput{Size: 300}})
	if err != nil || len(page.List) != 200 || page.Total != 204 {
		t.Fatalf("rows=%d total=%d err=%v", len(page.List), page.Total, err)
	}
}

func openServiceStore(t *testing.T) *sqlite.Store {
	t.Helper()
	store, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "service.db"))
	if err != nil {
		t.Fatal(err)
	}
	return store
}
func seedSonarrConfig(t *testing.T, store *sqlite.Store, rawURL, key, regex string) {
	t.Helper()
	seedConfig(t, store, []sqlite.SystemConfig{{ID: 1, Key: "sonarrUrl", Value: &rawURL, ValidStatus: sqlite.Valid}, {ID: 2, Key: "sonarrApikey", Value: &key, ValidStatus: sqlite.Valid}, {ID: 16, Key: "cleanTitleRegex", Value: &regex, ValidStatus: sqlite.Valid}})
}
func seedRadarrConfig(t *testing.T, store *sqlite.Store, rawURL, key, regex string) {
	t.Helper()
	seedConfig(t, store, []sqlite.SystemConfig{{ID: 7, Key: "radarrUrl", Value: &rawURL, ValidStatus: sqlite.Valid}, {ID: 8, Key: "radarrApikey", Value: &key, ValidStatus: sqlite.Valid}, {ID: 16, Key: "cleanTitleRegex", Value: &regex, ValidStatus: sqlite.Valid}})
}
func seedConfig(t *testing.T, store *sqlite.Store, rows []sqlite.SystemConfig) {
	t.Helper()
	if err := store.Repositories().SystemConfigs.UpsertBatch(context.Background(), rows); err != nil {
		t.Fatal(fmt.Errorf("seed config: %w", err))
	}
}

func radarrMoviesJSON(count int) string {
	values := make([]string, count)
	for index := range values {
		id := index + 100
		values[index] = fmt.Sprintf(`{"id":%d,"tmdbId":%d,"title":"Movie %d","path":"/movies/Movie %d (2024)","cleanTitle":"movie %d","originalTitle":"Original %d","year":2024,"monitored":false,"alternateTitles":[]}`, id, id, id, id, id, id)
	}
	return "[" + strings.Join(values, ",") + "]"
}
