package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func TestRepositories_pageFilterAndInjection_whenTimestampsTie(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "page.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	repo := store.Repositories().SonarrRules
	stamp := stringPointer("2026-08-05 12:00:00")
	for _, row := range []SonarrRule{{ID: "a", Token: "alpha", Priority: 1, Regex: "a", Example: "a", ValidStatus: Valid, UpdateTime: stamp}, {ID: "b", Token: "alpha", Priority: 2, Regex: "b", Example: "b", ValidStatus: Valid, UpdateTime: stamp}, {ID: "c", Token: "quote' OR 1=1 --", Priority: 3, Regex: "c", Example: "c", ValidStatus: Valid, UpdateTime: stamp}} {
		if err := repo.Upsert(ctx, row); err != nil {
			t.Fatal(err)
		}
	}
	page, err := repo.Page(ctx, RuleFilter{Page: PageInput{Current: 1, Size: 2}, Token: stringPointer("alpha")})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 2 || len(page.List) != 2 || page.List[0].ID != "b" || page.List[1].ID != "a" {
		t.Fatalf("page=%+v", page)
	}
	probe, err := repo.Page(ctx, RuleFilter{Page: PageInput{}, Token: stringPointer("quote' OR 1=1 --")})
	if err != nil || probe.Total != 1 || probe.List[0].ID != "c" {
		t.Fatalf("probe=%+v err=%v", probe, err)
	}
	wildcard, err := repo.Page(ctx, RuleFilter{Page: PageInput{}, Token: stringPointer("a%")})
	if err != nil || wildcard.Total != 2 {
		t.Fatalf("Java LIKE wildcard compatibility lost: %+v err=%v", wildcard, err)
	}
}

func TestRepositories_batchAndReplacement_rollBackOnCancellation(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "batch.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	repo := store.Repositories().RadarrTitles
	rows := make([]RadarrTitle, 200)
	for i := range rows {
		rows[i] = RadarrTitle{ID: RadarrTitleID(i + 1), TMDBID: int64(i + 1000), MainTitle: "main", Title: "title", CleanTitle: "clean", Year: 2026, Monitored: Monitored, ValidStatus: Valid}
	}
	if err := repo.UpsertBatch(ctx, RadarrTitleBatch{Rows: rows}); err != nil {
		t.Fatal(err)
	}
	if err := repo.DeleteBatch(ctx, RadarrTitleIDs{IDs: []RadarrTitleID{1, 2}}); err != nil {
		t.Fatal(err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	err = repo.Replace(cancelled, RadarrTitleBatch{Rows: []RadarrTitle{{ID: 999, TMDBID: 999, MainTitle: "new", Title: "new", CleanTitle: "new", Year: 2026, Monitored: Monitored, ValidStatus: Valid}}})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v", err)
	}
	page, err := repo.Page(ctx, RadarrTitleFilter{Page: PageInput{Size: 300}})
	if err != nil || page.Total != 198 {
		t.Fatalf("page=%+v err=%v", page, err)
	}
	if err := store.InTransaction(ctx, func(tx DatasetTransaction) error {
		if err := tx.ReplaceRadarrTitles(ctx, RadarrTitleBatch{Rows: []RadarrTitle{{ID: 999, TMDBID: 999, MainTitle: "new", Title: "new", CleanTitle: "new", Year: 2026, Monitored: Monitored, ValidStatus: Valid}}}); err != nil {
			return err
		}
		return errors.New("force rollback")
	}); err == nil {
		t.Fatal("want rollback error")
	}
	page, err = repo.Page(ctx, RadarrTitleFilter{Page: PageInput{Size: 300}})
	if err != nil || page.Total != 198 {
		t.Fatalf("rollback failed page=%+v err=%v", page, err)
	}
}

func TestStoreInTransaction_rejectsNilCallback_whenCalled(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "nil-callback.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if err := store.InTransaction(ctx, nil); !errors.Is(err, ErrNilTransactionCallback) {
		t.Fatalf("err=%v", err)
	}
}

func TestRadarrTitles_replaceChunks401Rows_whenDatasetExceedsBatchLimit(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "chunks.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	rows := make([]RadarrTitle, 401)
	for index := range rows {
		rows[index] = RadarrTitle{ID: RadarrTitleID(index + 1), TMDBID: int64(index + 1), MainTitle: "m", Title: "t", CleanTitle: "c", Year: 2026, Monitored: Monitored, ValidStatus: Valid}
	}
	if err := store.Repositories().RadarrTitles.Replace(ctx, RadarrTitleBatch{Rows: rows}); err != nil {
		t.Fatal(err)
	}
	page, err := store.Repositories().RadarrTitles.Page(ctx, RadarrTitleFilter{Page: PageInput{Size: 200}})
	if err != nil || page.Total != 401 {
		t.Fatalf("total=%d err=%v", page.Total, err)
	}
}

func stringPointer(value string) *string { return &value }
func int64Pointer(value int64) *int64    { return &value }
