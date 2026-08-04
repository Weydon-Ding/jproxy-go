package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func TestStoreInTransaction_rollsBackAndReturnsCallbackError_whenCallbackWrites(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "callback-error.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	want := errors.New("callback failed")

	err = store.InTransaction(ctx, func(tx DatasetTransaction) error {
		if writeErr := tx.UpsertSonarrRules(ctx, SonarrRuleBatch{Rows: []SonarrRule{{ID: "rollback", Token: "rollback", Regex: "x", Example: "x", ValidStatus: Valid}}}); writeErr != nil {
			return writeErr
		}
		return want
	})

	if !errors.Is(err, want) {
		t.Fatalf("InTransaction() error = %v, want callback error", err)
	}
	page, pageErr := store.Repositories().SonarrRules.Page(ctx, RuleFilter{})
	if pageErr != nil || page.Total != 0 {
		t.Fatalf("page after rollback = %+v, error = %v", page, pageErr)
	}
}

func TestStoreInTransaction_rollsBackAndRepanics_whenCallbackPanics(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "callback-panic.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	panicValue := "callback panic"

	func() {
		defer func() {
			if got := recover(); got != panicValue {
				t.Fatalf("panic = %v, want %q", got, panicValue)
			}
		}()
		_ = store.InTransaction(ctx, func(tx DatasetTransaction) error {
			if writeErr := tx.UpsertSonarrRules(ctx, SonarrRuleBatch{Rows: []SonarrRule{{ID: "panic", Token: "panic", Regex: "x", Example: "x", ValidStatus: Valid}}}); writeErr != nil {
				return writeErr
			}
			panic(panicValue)
		})
	}()

	page, pageErr := store.Repositories().SonarrRules.Page(ctx, RuleFilter{})
	if pageErr != nil || page.Total != 0 {
		t.Fatalf("page after panic rollback = %+v, error = %v", page, pageErr)
	}
}

func TestStoreInTransaction_rollsBack_whenContextCancelledAfterBegin(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "callback-cancel.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	cancelled, cancel := context.WithCancel(ctx)
	defer cancel()

	err = store.InTransaction(ctx, func(tx DatasetTransaction) error {
		cancel()
		return tx.UpsertSonarrRules(cancelled, SonarrRuleBatch{Rows: []SonarrRule{{ID: "cancel", Token: "cancel", Regex: "x", Example: "x", ValidStatus: Valid}}})
	})

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("InTransaction() error = %v, want context.Canceled", err)
	}
	page, pageErr := store.Repositories().SonarrRules.Page(ctx, RuleFilter{})
	if pageErr != nil || page.Total != 0 {
		t.Fatalf("page after cancellation rollback = %+v, error = %v", page, pageErr)
	}
}

func TestStoreInTransaction_restoresDataset_whenCancelledAfterReplaceWritesFirstChunk(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "replace-cancel.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	repos := store.Repositories()
	if err := repos.RadarrTitles.Upsert(ctx, RadarrTitle{ID: 1, MainTitle: "old", Title: "old", CleanTitle: "old", Monitored: Monitored, ValidStatus: Valid}); err != nil {
		t.Fatal(err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	defer cancel()

	err = store.InTransaction(ctx, func(tx DatasetTransaction) error {
		if replaceErr := tx.ReplaceRadarrTitles(ctx, RadarrTitleBatch{Rows: []RadarrTitle{{ID: 2, MainTitle: "first", Title: "first", CleanTitle: "first", Monitored: Monitored, ValidStatus: Valid}}}); replaceErr != nil {
			return replaceErr
		}
		cancel()
		return tx.UpsertRadarrTitles(cancelled, RadarrTitleBatch{Rows: []RadarrTitle{{ID: 3, MainTitle: "second", Title: "second", CleanTitle: "second", Monitored: Monitored, ValidStatus: Valid}}})
	})

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("InTransaction() error = %v, want context.Canceled", err)
	}
	page, pageErr := repos.RadarrTitles.Page(ctx, RadarrTitleFilter{})
	if pageErr != nil || page.Total != 1 || page.List[0].ID != 1 {
		t.Fatalf("dataset after cancellation = %+v, error = %v", page, pageErr)
	}
	if err := repos.SystemConfigs.Upsert(ctx, SystemConfig{ID: 1, Key: "reused", ValidStatus: Valid}); err != nil {
		t.Fatalf("connection was not reusable after rollback: %v", err)
	}
}
