package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
)

func TestSonarrTitles_replaceRollsBackAndStoreRemainsReusable_whenCommitFails(t *testing.T) {
	// Given
	store, err := Open(context.Background(), filepath.Join(t.TempDir(), "commit.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	oldClean := "old"
	if err := store.Repositories().SonarrTitles.Upsert(context.Background(), SonarrTitle{ID: 1, TVDBID: 1, MainTitle: "old", Title: "old", CleanTitle: &oldClean, Monitored: Monitored, ValidStatus: Valid}); err != nil {
		t.Fatal(err)
	}
	commitErr := errors.New("commit failure")
	store.commit = func(*sql.Tx) error { return commitErr }
	clean := "new"
	batch := SonarrTitleBatch{Rows: []SonarrTitle{{ID: 20, TVDBID: 2, MainTitle: "new", Title: "new", CleanTitle: &clean, Monitored: Monitored, ValidStatus: Valid}}}

	// When
	err = store.Repositories().SonarrTitles.Replace(context.Background(), batch)

	// Then
	if !errors.Is(err, commitErr) {
		t.Fatalf("replace error = %v", err)
	}
	if _, err := store.Repositories().SonarrTitles.Get(context.Background(), 1); err != nil {
		t.Fatalf("old row missing: %v", err)
	}
	store.commit = func(tx *sql.Tx) error { return tx.Commit() }
	if err := store.Repositories().SonarrTitles.Replace(context.Background(), batch); err != nil {
		t.Fatalf("retry error = %v", err)
	}
}
