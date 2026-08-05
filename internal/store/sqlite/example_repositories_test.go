package sqlite

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestExampleRepositories_preserveEmptyLines_whenBatchUpsertsJavaSplitRows(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "empty-lines.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	rows := []SonarrExample{{Hash: exampleHash("a"), OriginalText: "a", ValidStatus: Invalid}, {Hash: exampleHash(""), OriginalText: "", ValidStatus: Invalid}, {Hash: exampleHash("b"), OriginalText: "b", ValidStatus: Invalid}}
	if err := store.Repositories().SonarrExamples.UpsertBatch(ctx, SonarrExampleBatch{Rows: rows}); err != nil {
		t.Fatal(err)
	}
	page, err := store.Repositories().SonarrExamples.Page(ctx, ExampleFilter{Page: PageInput{Current: 1, Size: 10}})
	if err != nil || page.Total != 3 {
		t.Fatalf("page=%+v err=%v", page, err)
	}
	for _, row := range page.List {
		if row.Hash == "D41D8CD98F00B204E9800998ECF8427E" && row.OriginalText == "" {
			return
		}
	}
	t.Fatal("empty Java split row was not preserved")
}

func exampleHash(text string) string {
	sum := md5.Sum([]byte(text))
	return strings.ToUpper(hex.EncodeToString(sum[:]))
}

func TestExampleRepositories_pageUpsertDeleteAndRollback_whenRowsAreManaged(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "examples.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	// Given: two rows with a deterministic tied update timestamp.
	stamp := stringPointer("2026-08-05T12:00:00Z")
	rows := []SonarrExample{{Hash: "B", OriginalText: "beta", ValidStatus: Valid, UpdateTime: stamp}, {Hash: "A", OriginalText: "alpha", ValidStatus: Valid, UpdateTime: stamp}}

	// When: the rows are atomically upserted and queried with a trimmed filter.
	repo := store.Repositories().SonarrExamples
	if err := repo.UpsertBatch(ctx, SonarrExampleBatch{Rows: rows}); err != nil {
		t.Fatal(err)
	}
	page, err := repo.Page(ctx, ExampleFilter{Page: PageInput{Current: 1, Size: 1}, OriginalText: stringPointer(" alpha ")})

	// Then: paging exposes the matching row and Java-compatible order.
	if err != nil || page.Total != 1 || len(page.List) != 1 || page.List[0].Hash != "A" {
		t.Fatalf("page=%+v err=%v", page, err)
	}

	// Given: a cancelled replacement after data exists.
	cancelled, cancel := context.WithCancel(ctx)
	cancel()

	// When: replacement is cancelled.
	err = repo.Replace(cancelled, SonarrExampleBatch{Rows: []SonarrExample{{Hash: "C", OriginalText: "cancelled", ValidStatus: Valid}}})

	// Then: the original rows remain committed.
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v", err)
	}
	all, err := repo.Page(ctx, ExampleFilter{Page: PageInput{Size: 10}})
	if err != nil || all.Total != 2 {
		t.Fatalf("page=%+v err=%v", all, err)
	}
}

func TestExampleRepositories_rejectBatchOverLimit_whenUpserting(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "example-limit.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	rows := make([]RadarrExample, 201)
	for index := range rows {
		rows[index] = RadarrExample{Hash: string(rune(index + 1)), OriginalText: "row", ValidStatus: Valid}
	}

	// When: a bounded batch is exceeded.
	err = store.Repositories().RadarrExamples.UpsertBatch(ctx, RadarrExampleBatch{Rows: rows})

	// Then: no partial batch is accepted.
	if !errors.Is(err, ErrBatchTooLarge) {
		t.Fatalf("err=%v", err)
	}
}
