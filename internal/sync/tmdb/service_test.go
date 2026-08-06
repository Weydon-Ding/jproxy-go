package tmdb

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"jproxy-go/internal/store/sqlite"
)

func TestService_Sync_queriesStableIDsAndBothLanguages_thenUpsertsOneBatch(t *testing.T) {
	// Given
	store := newStore(t)
	seedTMDBConfig(t, store, "http://unused", "secret", "zh-CN", "zh-TW")
	for _, id := range []int64{20, 10} {
		if err := store.Repositories().SonarrTitles.Upsert(context.Background(), sqlite.SonarrTitle{ID: sqlite.SonarrTitleID(id), TVDBID: id, MainTitle: "show", Title: "show", CleanTitle: stringPointer("show"), SeasonNumber: 1, Monitored: sqlite.Monitored, ValidStatus: sqlite.Valid}); err != nil {
			t.Fatal(err)
		}
	}
	client := &recordingClient{results: map[string]Alias{
		"10/zh-CN": {TVDBID: 10, Language: "zh-CN", Title: "CN"}, "10/zh-TW": {TVDBID: 10, Language: "zh-TW", Title: "TW"},
		"20/zh-CN": {TVDBID: 20, Language: "zh-CN", Title: "CN"}, "20/zh-TW": {TVDBID: 20, Language: "zh-TW", Title: "TW"},
	}}
	repository := &recordingRepository{}
	service := NewService(Dependencies{Config: NewConfigSource(store.Repositories().SystemConfigs), Client: client, Source: store.Repositories().SonarrTitles, Repository: repository})

	// When
	err := service.Sync(context.Background())

	// Then
	if err != nil || !sameStrings(client.calls, []string{"10/zh-CN", "10/zh-TW", "20/zh-CN", "20/zh-TW"}) || repository.calls != 1 || len(repository.batch.Rows) != 4 {
		t.Fatalf("error=%v calls=%v upserts=%d rows=%#v", err, client.calls, repository.calls, repository.batch.Rows)
	}
}

func TestService_Sync_keepsDuplicateLanguageAndSkipsMissingResult(t *testing.T) {
	// Given
	store := newStore(t)
	seedTMDBConfig(t, store, "http://unused", "secret", "en", "en")
	if err := store.Repositories().SonarrTitles.Upsert(context.Background(), sqlite.SonarrTitle{ID: 1, TVDBID: 1, MainTitle: "show", Title: "show", CleanTitle: stringPointer("show"), SeasonNumber: 1, Monitored: sqlite.Monitored, ValidStatus: sqlite.Valid}); err != nil {
		t.Fatal(err)
	}
	client := &recordingClient{results: map[string]Alias{"1/en": {TVDBID: 1, Language: "en", Title: "title"}}}
	repository := &recordingRepository{}
	service := NewService(Dependencies{Config: NewConfigSource(store.Repositories().SystemConfigs), Client: client, Source: store.Repositories().SonarrTitles, Repository: repository})

	// When
	err := service.Sync(context.Background())

	// Then
	if err != nil || !sameStrings(client.calls, []string{"1/en", "1/en"}) || repository.calls != 1 || len(repository.batch.Rows) != 2 {
		t.Fatalf("error=%v calls=%v upserts=%d rows=%#v", err, client.calls, repository.calls, repository.batch.Rows)
	}
}

func TestService_Sync_readsCurrentConfigurationOnEveryAttempt(t *testing.T) {
	// Given
	store := newStore(t)
	seedTMDBConfig(t, store, "http://first", "first-key", "en", "zh")
	if err := store.Repositories().SonarrTitles.Upsert(context.Background(), sqlite.SonarrTitle{ID: 1, TVDBID: 1, MainTitle: "show", Title: "show", CleanTitle: stringPointer("show"), SeasonNumber: 1, Monitored: sqlite.Monitored, ValidStatus: sqlite.Valid}); err != nil {
		t.Fatal(err)
	}
	client := &recordingClient{}
	repository := &recordingRepository{}
	service := NewService(Dependencies{Config: NewConfigSource(store.Repositories().SystemConfigs), Client: client, Source: store.Repositories().SonarrTitles, Repository: repository})

	// When
	firstErr := service.Sync(context.Background())
	seedTMDBConfig(t, store, "http://second", "second-key", "fr", "de")
	secondErr := service.Sync(context.Background())

	// Then
	if firstErr != nil || secondErr != nil || !sameStrings(client.calls, []string{"1/en", "1/zh", "1/fr", "1/de"}) || repository.calls != 2 {
		t.Fatalf("first=%v second=%v calls=%v upserts=%d", firstErr, secondErr, client.calls, repository.calls)
	}
}

func TestService_Sync_stopsBeforeWrite_whenContextIsCancelled(t *testing.T) {
	// Given
	store := newStore(t)
	seedTMDBConfig(t, store, "http://unused", "secret", "en", "zh")
	if err := store.Repositories().SonarrTitles.Upsert(context.Background(), sqlite.SonarrTitle{ID: 1, TVDBID: 1, MainTitle: "show", Title: "show", CleanTitle: stringPointer("show"), SeasonNumber: 1, Monitored: sqlite.Monitored, ValidStatus: sqlite.Valid}); err != nil {
		t.Fatal(err)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	client := &recordingClient{err: context.Canceled}
	repository := &recordingRepository{}
	service := NewService(Dependencies{Config: NewConfigSource(store.Repositories().SystemConfigs), Client: client, Source: store.Repositories().SonarrTitles, Repository: repository})

	// When
	err := service.Sync(cancelled)

	// Then
	if !errors.Is(err, context.Canceled) || repository.calls != 0 {
		t.Fatalf("error=%v upserts=%d", err, repository.calls)
	}
}

func TestService_Sync_avoidsPartialWrites_whenFindFails(t *testing.T) {
	// Given
	store := newStore(t)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/3/find/2" {
			writer.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = writer.Write([]byte(`{"tv_results":[{"id":1,"name":"title"}]}`))
	}))
	t.Cleanup(server.Close)
	seedTMDBConfig(t, store, server.URL, "secret", "en", "zh")
	for _, id := range []int64{1, 2} {
		if err := store.Repositories().SonarrTitles.Upsert(context.Background(), sqlite.SonarrTitle{ID: sqlite.SonarrTitleID(id), TVDBID: id, MainTitle: "show", Title: "show", CleanTitle: stringPointer("show"), SeasonNumber: 1, Monitored: sqlite.Monitored, ValidStatus: sqlite.Valid}); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.Repositories().TMDBTitles.Upsert(context.Background(), sqlite.TMDBTitle{ID: 99, TVDBID: 99, Language: "en", Title: "existing", ValidStatus: sqlite.Valid}); err != nil {
		t.Fatal(err)
	}
	service := NewService(Dependencies{Config: NewConfigSource(store.Repositories().SystemConfigs), Client: NewClient(server.Client(), time.Second), Source: store.Repositories().SonarrTitles, Repository: store.Repositories().TMDBTitles})

	// When
	err := service.Sync(context.Background())

	// Then
	rows, readErr := store.Repositories().TMDBTitles.FindByTVDBID(context.Background(), 99)
	if err == nil || readErr != nil || len(rows) != 1 || rows[0].Title != "existing" {
		t.Fatalf("sync=%v rows=%#v read=%v", err, rows, readErr)
	}
}

func TestService_Sync_persistsSingleLanguageIdempotently(t *testing.T) {
	// Given
	store := newStore(t)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Query().Get("language") == "zh" {
			_, _ = writer.Write([]byte(`{"tv_results":[]}`))
			return
		}
		_, _ = writer.Write([]byte(`{"tv_results":[{"id":1,"name":"title"}]}`))
	}))
	t.Cleanup(server.Close)
	seedTMDBConfig(t, store, server.URL, "secret", "en", "zh")
	if err := store.Repositories().SonarrTitles.Upsert(context.Background(), sqlite.SonarrTitle{ID: 1, TVDBID: 1, MainTitle: "show", Title: "show", CleanTitle: stringPointer("show"), SeasonNumber: 1, Monitored: sqlite.Monitored, ValidStatus: sqlite.Valid}); err != nil {
		t.Fatal(err)
	}
	service := NewService(Dependencies{Config: NewConfigSource(store.Repositories().SystemConfigs), Client: NewClient(server.Client(), time.Second), Source: store.Repositories().SonarrTitles, Repository: store.Repositories().TMDBTitles})

	// When
	firstErr := service.Sync(context.Background())
	secondErr := service.Sync(context.Background())

	// Then
	rows, readErr := store.Repositories().TMDBTitles.FindByTVDBID(context.Background(), 1)
	if firstErr != nil || secondErr != nil || readErr != nil || len(rows) != 1 || rows[0].Language != "en" || rows[0].Title != "title" {
		t.Fatalf("first=%v second=%v rows=%#v read=%v", firstErr, secondErr, rows, readErr)
	}
}

func newStore(t *testing.T) *sqlite.Store {
	t.Helper()
	store, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "tmdb.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func seedTMDBConfig(t *testing.T, store *sqlite.Store, url, key, first, second string) {
	t.Helper()
	rows := []sqlite.SystemConfig{{ID: 5, Key: "sonarrLanguage1", Value: &first, ValidStatus: sqlite.Valid}, {ID: 6, Key: "sonarrLanguage2", Value: &second, ValidStatus: sqlite.Valid}, {ID: 14, Key: "tmdbUrl", Value: &url, ValidStatus: sqlite.Valid}, {ID: 15, Key: "tmdbApikey", Value: &key, ValidStatus: sqlite.Valid}}
	if err := store.Repositories().SystemConfigs.UpsertBatch(context.Background(), rows); err != nil {
		t.Fatal(err)
	}
}

type recordingClient struct {
	calls   []string
	results map[string]Alias
	err     error
}

func (client *recordingClient) Find(_ context.Context, _ Config, id int64, language string) (Alias, bool, error) {
	key := stringID(id) + "/" + language
	client.calls = append(client.calls, key)
	if client.err != nil {
		return Alias{}, false, client.err
	}
	result, found := client.results[key]
	return result, found, nil
}

type recordingRepository struct {
	calls int
	batch sqlite.TMDBTitleBatch
	err   error
}

func (repository *recordingRepository) UpsertBatch(_ context.Context, batch sqlite.TMDBTitleBatch) error {
	repository.calls++
	repository.batch = batch
	return repository.err
}
func stringID(value int64) string        { return strconv.FormatInt(value, 10) }
func stringPointer(value string) *string { return &value }
func sameStrings(actual, expected []string) bool {
	if len(actual) != len(expected) {
		return false
	}
	for index := range actual {
		if actual[index] != expected[index] {
			return false
		}
	}
	return true
}

var _ finder = (*recordingClient)(nil)
var _ batchRepository = (*recordingRepository)(nil)
