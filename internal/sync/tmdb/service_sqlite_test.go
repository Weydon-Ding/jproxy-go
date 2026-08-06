package tmdb

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"jproxy-go/internal/store/sqlite"
)

func TestService_Sync_persistsGeneratedAliasesAtomicallyAndIdempotently(t *testing.T) {
	// Given
	store := newStore(t)
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests++
		id, language := request.URL.Path[len("/3/find/"):], request.URL.Query().Get("language")
		_, _ = fmt.Fprintf(writer, `{"tv_results":[{"id":%s,"name":"title-%s-%s"}]}`, id, id, language)
	}))
	t.Cleanup(server.Close)
	seedTMDBConfig(t, store, server.URL, "secret", "en", "zh")
	seedSyncTitles(t, store, 10, 20)
	service := NewService(Dependencies{Config: NewConfigSource(store.Repositories().SystemConfigs), Client: NewClient(server.Client(), time.Second), Source: store.Repositories().SonarrTitles, Repository: store.Repositories().TMDBTitles})

	// When
	firstErr := service.Sync(context.Background())
	before := tmdbRows(t, store, 10, 20)
	secondErr := service.Sync(context.Background())
	after := tmdbRows(t, store, 10, 20)

	// Then
	if firstErr != nil || secondErr != nil || requests != 4 || tmdbDigest(before) != tmdbDigest(after) {
		t.Fatalf("first=%v second=%v requests=%d before=%#v after=%#v", firstErr, secondErr, requests, before, after)
	}
	want := []sqlite.TMDBTitle{
		{TVDBID: 10, TMDBID: int64Pointer(10), Language: "en", Title: "title-10-en", ValidStatus: sqlite.Valid},
		{TVDBID: 10, TMDBID: int64Pointer(10), Language: "zh", Title: "title-10-zh", ValidStatus: sqlite.Valid},
		{TVDBID: 20, TMDBID: int64Pointer(20), Language: "en", Title: "title-20-en", ValidStatus: sqlite.Valid},
		{TVDBID: 20, TMDBID: int64Pointer(20), Language: "zh", Title: "title-20-zh", ValidStatus: sqlite.Valid},
	}
	if len(before) != len(want) || !sameAliases(before, want) {
		t.Fatalf("rows=%#v", before)
	}
}

func seedSyncTitles(t *testing.T, store *sqlite.Store, ids ...int64) {
	t.Helper()
	for _, id := range ids {
		if err := store.Repositories().SonarrTitles.Upsert(context.Background(), sqlite.SonarrTitle{ID: sqlite.SonarrTitleID(id), TVDBID: id, MainTitle: "show", Title: "show", CleanTitle: stringPointer("show"), SeasonNumber: 1, Monitored: sqlite.Monitored, ValidStatus: sqlite.Valid}); err != nil {
			t.Fatal(err)
		}
	}
}

func tmdbRows(t *testing.T, store *sqlite.Store, ids ...int64) []sqlite.TMDBTitle {
	t.Helper()
	var rows []sqlite.TMDBTitle
	for _, id := range ids {
		found, err := store.Repositories().TMDBTitles.FindByTVDBID(context.Background(), id)
		if err != nil {
			t.Fatal(err)
		}
		rows = append(rows, found...)
	}
	sort.Slice(rows, func(left, right int) bool { return rows[left].ID < rows[right].ID })
	return rows
}

func sameAliases(actual, expected []sqlite.TMDBTitle) bool {
	for index := range expected {
		if actual[index].ID <= 0 || actual[index].ID > 2147483647 || actual[index].TVDBID != expected[index].TVDBID || actual[index].TMDBID == nil || *actual[index].TMDBID != *expected[index].TMDBID || actual[index].Language != expected[index].Language || actual[index].Title != expected[index].Title || actual[index].ValidStatus != expected[index].ValidStatus {
			return false
		}
	}
	return true
}

func tmdbDigest(rows []sqlite.TMDBTitle) string {
	values := make([]string, len(rows))
	for index, row := range rows {
		values[index] = fmt.Sprintf("%d/%d/%d/%s/%s/%d", row.ID, row.TVDBID, *row.TMDBID, row.Language, row.Title, row.ValidStatus)
	}
	return strings.Join(values, "|")
}

func int64Pointer(value int64) *int64 { return &value }
