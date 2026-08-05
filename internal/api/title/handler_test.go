package title

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"jproxy-go/internal/format"
	"jproxy-go/internal/runtime"
	"jproxy-go/internal/store/sqlite"
)

func TestHandler_queriesTitlesWithJavaPageContract(t *testing.T) {
	store := newStore(t)
	ctx := context.Background()
	cleanTitle := "needle"
	if err := store.Repositories().SonarrTitles.Upsert(ctx, sqlite.SonarrTitle{ID: 1, TVDBID: 101, MainTitle: "Main", Title: "Needle", CleanTitle: &cleanTitle, SeasonNumber: 1, Monitored: sqlite.Monitored, ValidStatus: sqlite.Valid}); err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(Options{Store: store, Provider: staticProvider("[0-9]+")})

	response := request(handler, http.MethodGet, "/api/sonarr/title/query?current=1&pageSize=1&title=Needle&tvdbId=101", nil)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	var body struct {
		Current  int64 `json:"current"`
		PageSize int64 `json:"pageSize"`
		Total    int64 `json:"total"`
		List     []struct {
			ID     int64  `json:"id"`
			TVDBID int64  `json:"tvdbId"`
			Title  string `json:"title"`
		} `json:"list"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Current != 1 || body.PageSize != 1 || body.Total != 1 || len(body.List) != 1 || body.List[0].ID != 1 || body.List[0].TVDBID != 101 || body.List[0].Title != "Needle" {
		t.Fatalf("unexpected page: %#v", body)
	}
}

func TestHandler_rejectsUnknownAndRepeatedQueryParameters(t *testing.T) {
	handler := NewHandler(Options{Store: newStore(t)})

	for _, path := range []string{
		"/api/sonarr/title/query?unknown=value",
		"/api/radarr/title/query?tmdbId=1&tmdbId=2",
		"/api/tmdb/title/query?current=0",
		"/api/tmdb/title/query?pageSize=201",
	} {
		response := request(handler, http.MethodGet, path, nil)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("%s status = %d, want %d", path, response.Code, http.StatusBadRequest)
		}
	}
}

func TestHandler_savesTMDBTitleCleansQueryProjectionAndInvalidates(t *testing.T) {
	store := newStore(t)
	invalidator := &recordingInvalidator{}
	handler := NewHandler(Options{Store: store, Provider: staticProvider("[0-9]+"), Invalidate: invalidator.invalidate})

	response := request(handler, http.MethodPost, "/api/tmdb/title/save", []byte(`{"tvdbId":123,"tmdbId":456,"language":"en","title":"The Show 2026","validStatus":1}`))

	if response.Code != http.StatusOK {
		t.Fatalf("save status = %d, want %d", response.Code, http.StatusOK)
	}
	if !invalidator.contains(runtime.SonarrSearchTitle, runtime.IndexerSearchOffset, runtime.SonarrResultTitle) {
		t.Fatalf("save invalidation = %#v", invalidator.names)
	}
	response = request(handler, http.MethodGet, "/api/tmdb/title/query?tvdbId=123", nil)
	var body struct {
		List []struct {
			TMDBID     *int64 `json:"tmdbId"`
			CleanTitle string `json:"cleanTitle"`
		} `json:"list"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.List) != 1 || body.List[0].TMDBID == nil || *body.List[0].TMDBID != 456 || body.List[0].CleanTitle != "show" {
		t.Fatalf("unexpected tmdb page: %#v", body)
	}
}

func TestHandler_removesTitlesAndRejectsUnavailableSync(t *testing.T) {
	store := newStore(t)
	ctx := context.Background()
	if err := store.Repositories().RadarrTitles.Upsert(ctx, sqlite.RadarrTitle{ID: 8, TMDBID: 80, MainTitle: "Main", Title: "Movie", Year: 2026, Monitored: sqlite.Monitored, ValidStatus: sqlite.Valid}); err != nil {
		t.Fatal(err)
	}
	invalidator := &recordingInvalidator{}
	handler := NewHandler(Options{Store: store, Invalidate: invalidator.invalidate})

	response := request(handler, http.MethodPost, "/api/radarr/title/remove", []byte(`[8]`))

	if response.Code != http.StatusOK {
		t.Fatalf("remove status = %d, want %d", response.Code, http.StatusOK)
	}
	if !invalidator.contains(runtime.RadarrSearchTitle, runtime.IndexerSearchOffset, runtime.RadarrResultTitle) {
		t.Fatalf("remove invalidation = %#v", invalidator.names)
	}
	page, err := store.Repositories().RadarrTitles.Page(ctx, sqlite.RadarrTitleFilter{})
	if err != nil || page.Total != 0 {
		t.Fatalf("removed page = %#v, error = %v", page, err)
	}
	response = request(handler, http.MethodPost, "/api/tmdb/title/sync", nil)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("sync status = %d, want %d", response.Code, http.StatusServiceUnavailable)
	}
	response = request(handler, http.MethodGet, "/api/tmdb/title/remove", nil)
	if response.Code != http.StatusMethodNotAllowed || response.Header().Get("Allow") != http.MethodPost {
		t.Fatalf("method response = %d, Allow = %q", response.Code, response.Header().Get("Allow"))
	}
}

func TestHandler_servesTMDBQueryOverHTTP(t *testing.T) {
	store := newStore(t)
	ctx := context.Background()
	if err := store.Repositories().TMDBTitles.Upsert(ctx, sqlite.TMDBTitle{ID: 9, TVDBID: 900, Language: "en", Title: "The HTTP Show", ValidStatus: sqlite.Valid}); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(NewHandler(Options{Store: store, Provider: staticProvider("")}))
	t.Cleanup(server.Close)

	response, err := server.Client().Get(server.URL + "/api/tmdb/title/query?tvdbId=900")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = response.Body.Close() })
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || !bytes.Contains(body, []byte(`"title":"The HTTP Show"`)) {
		t.Fatalf("status = %d, body = %s", response.StatusCode, body)
	}
}

func newStore(t *testing.T) *sqlite.Store {
	t.Helper()
	store, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "titles.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func staticProvider(cleanTitleRegex string) runtime.Provider {
	return runtime.NewStaticProvider(sqlite.Snapshot{Sonarr: format.SonarrConfig{CleanTitleRegex: cleanTitleRegex}})
}

func request(handler http.Handler, method, path string, body []byte) *httptest.ResponseRecorder {
	recording := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	handler.ServeHTTP(recording, req)
	return recording
}

type recordingInvalidator struct{ names []string }

func (r *recordingInvalidator) invalidate(_ context.Context, names ...string) error {
	r.names = append(r.names, names...)
	return nil
}

func (r *recordingInvalidator) contains(want ...string) bool {
	if len(r.names) != len(want) {
		return false
	}
	for index, name := range want {
		if r.names[index] != name {
			return false
		}
	}
	return true
}
