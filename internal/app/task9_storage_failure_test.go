package app

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strconv"
	"sync"
	"testing"
	"time"

	"jproxy-go/internal/config"
	"jproxy-go/internal/runtime"
	"jproxy-go/internal/store/sqlite"
)

func TestTask9_rootMuxRollsBackRadarrReplace_whenSecondChunkWriteFails(t *testing.T) {
	fixture := newTask9StorageFixture(t, "task9-radarr-chunk.db")
	fixture.syncRadarr(t, task9RadarrJSON, http.StatusOK)
	if task9UpdateConfigs(fixture.handler, task9Configs(t, fixture.store, fixture.upstreamURL, "storage-key")) != http.StatusOK {
		t.Fatal("clear radarr success marker")
	}
	beforeRows := task9RadarrRows(t, fixture.store)
	beforeRuntime := fixture.provider.Snapshot()
	fixture.exec(t, `CREATE TRIGGER task9_radarr_second_chunk BEFORE INSERT ON radarr_title WHEN NEW.tmdb_id=51 BEGIN SELECT RAISE(ABORT, 'task9 second chunk'); END`)
	fixture.syncRadarr(t, task9RadarrBatchJSON(51), http.StatusInternalServerError)
	afterRows := task9RadarrRows(t, fixture.store)
	if task9Digest(t, afterRows) != task9Digest(t, beforeRows) || !reflect.DeepEqual(fixture.provider.Snapshot(), beforeRuntime) {
		t.Fatal("second chunk failure changed committed state")
	}
	fixture.exec(t, `DROP TRIGGER task9_radarr_second_chunk`)
	fixture.syncRadarr(t, task9RadarrBatchJSON(51), http.StatusOK)
	if task9RadarrCount(t, fixture.store) != 204 {
		t.Fatalf("retry rows=%d", task9RadarrCount(t, fixture.store))
	}
	t.Logf("task9_second_chunk status=500 db_retained=%t runtime_retained=%t retry_status=200 retry_rows=%d", true, true, task9RadarrCount(t, fixture.store))
}

type task9StorageFixture struct {
	store       *sqlite.Store
	provider    runtime.Provider
	handler     http.Handler
	database    *sql.DB
	mu          sync.Mutex
	body        string
	upstreamURL string
}

func newTask9StorageFixture(t *testing.T, file string) *task9StorageFixture {
	t.Helper()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), file)
	store, err := sqlite.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	seedRootConfigs(t, store)
	fixture := &task9StorageFixture{store: store}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fixture.mu.Lock()
		body := fixture.body
		fixture.mu.Unlock()
		switch r.URL.Path {
		case "/api/v3/series", "/api/v3/movie":
			_, _ = w.Write([]byte(body))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(upstream.Close)
	snapshot, err := store.UpdateSystemConfigs(ctx, task9Configs(t, store, upstream.URL, "storage-key"))
	if err != nil {
		t.Fatal(err)
	}
	database, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	provider := runtime.NewProvider(snapshot, store)
	fixture.provider = provider
	fixture.handler = rootHandler(config.Config{Database: config.DatabaseConfig{Enabled: true}, HTTPTimeout: time.Second}, provider, store)
	fixture.database = database
	fixture.upstreamURL = upstream.URL
	return fixture
}

func (f *task9StorageFixture) syncRadarr(t *testing.T, value string, want int) {
	t.Helper()
	f.setUpstream(t, value)
	f.sync(t, "/api/radarr/title/sync", want)
}

func (f *task9StorageFixture) setUpstream(t *testing.T, value string) {
	t.Helper()
	f.mu.Lock()
	f.body = value
	f.mu.Unlock()
}

func (f *task9StorageFixture) sync(t *testing.T, path string, want int) {
	t.Helper()
	response := httptest.NewRecorder()
	f.handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, path, nil))
	if response.Code != want {
		t.Fatalf("path=%s status=%d want=%d", path, response.Code, want)
	}
}

func (f *task9StorageFixture) exec(t *testing.T, statement string) {
	t.Helper()
	if _, err := f.database.ExecContext(context.Background(), statement); err != nil {
		t.Fatal(err)
	}
}

func task9RadarrBatchJSON(count int) string {
	rows := make([]string, count)
	for index := range rows {
		id := index + 1
		rows[index] = `{"id":` + strconv.Itoa(id) + `,"tmdbId":` + strconv.Itoa(id) + `,"title":"Movie ` + strconv.Itoa(id) + `","path":"/movies/Movie ` + strconv.Itoa(id) + `","cleanTitle":"movie","originalTitle":"Original","year":2024,"monitored":true,"alternateTitles":[]}`
	}
	return "[" + joinTask9Rows(rows) + "]"
}

func joinTask9Rows(rows []string) string {
	if len(rows) == 0 {
		return ""
	}
	result := rows[0]
	for _, row := range rows[1:] {
		result += "," + row
	}
	return result
}
