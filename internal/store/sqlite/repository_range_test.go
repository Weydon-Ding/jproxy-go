package sqlite

import (
	"context"
	"errors"
	"math"
	"path/filepath"
	"testing"
)

func TestRepositories_acceptJavaIntegerBounds_whenWritingTitleRows(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "bounds.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	rows := []SonarrTitle{
		{ID: SonarrTitleID(math.MinInt32), TVDBID: math.MinInt32, SNO: math.MinInt32, MainTitle: "min", Title: "min", CleanTitle: stringPointer("min"), SeasonNumber: math.MinInt32, Monitored: Monitored, ValidStatus: Valid, SeriesID: int64Pointer(math.MinInt32)},
		{ID: SonarrTitleID(math.MaxInt32), TVDBID: math.MaxInt32, SNO: math.MaxInt32, MainTitle: "max", Title: "max", CleanTitle: stringPointer("max"), SeasonNumber: math.MaxInt32, Monitored: Monitored, ValidStatus: Valid, SeriesID: int64Pointer(math.MaxInt32)},
	}

	err = store.Repositories().SonarrTitles.UpsertBatch(ctx, SonarrTitleBatch{Rows: rows})

	if err != nil {
		t.Fatal(err)
	}
}

func TestRepositories_rejectJavaIntegerOverflowAtomically_whenSecondChunkOverflows(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "overflow.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	rows := make([]RadarrTitle, batchLimit+1)
	for index := range rows {
		rows[index] = RadarrTitle{ID: RadarrTitleID(index + 1), TMDBID: 1, MainTitle: "title", Title: "title", CleanTitle: "title", Year: 2024, Monitored: Monitored, ValidStatus: Valid}
	}
	rows[batchLimit].Year = math.MaxInt32 + 1

	err = store.Repositories().RadarrTitles.UpsertBatch(ctx, RadarrTitleBatch{Rows: rows})

	if !errors.Is(err, ErrJavaIntegerRange) {
		t.Fatalf("UpsertBatch() error = %v, want ErrJavaIntegerRange", err)
	}
	page, pageErr := store.Repositories().RadarrTitles.Page(ctx, RadarrTitleFilter{})
	if pageErr != nil || page.Total != 0 {
		t.Fatalf("rows after rejected batch = %+v, error = %v", page, pageErr)
	}
}
