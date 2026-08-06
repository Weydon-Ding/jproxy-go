package sqlite

import (
	"context"
	"path/filepath"
	"testing"
)

func TestTMDBTitleBatch_rollsBackGeneratedRows_whenLaterRowIsInvalid(t *testing.T) {
	// Given
	store, err := Open(context.Background(), filepath.Join(t.TempDir(), "tmdb-batch.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	rows := make([]TMDBTitle, batchLimit+1)
	for index := range rows {
		rows[index] = TMDBTitle{TVDBID: int64(index + 1), Language: "en", Title: "title", ValidStatus: Valid}
	}
	rows[batchLimit].ValidStatus = ValidStatus(2)

	// When
	err = store.Repositories().TMDBTitles.UpsertBatch(context.Background(), TMDBTitleBatch{Rows: rows})

	// Then
	page, pageErr := store.Repositories().TMDBTitles.Page(context.Background(), TMDBTitleFilter{})
	if err == nil || pageErr != nil || page.Total != 0 {
		t.Fatalf("upsert=%v page=%#v pageErr=%v", err, page, pageErr)
	}
}
