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

func TestRuleRepositories_acceptJavaIntegerBounds_whenWritingPriorityAndOffset(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "rule-bounds.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	repos := store.Repositories()
	sonarr := SonarrRule{ID: "sonarr", Token: "title", Priority: math.MinInt32, Offset: math.MaxInt32, Regex: "x", Example: "x", ValidStatus: Valid}
	radarr := RadarrRule{ID: "radarr", Token: "title", Priority: math.MaxInt32, Offset: math.MinInt32, Regex: "x", Example: "x", ValidStatus: Valid}

	if err := repos.SonarrRules.Upsert(ctx, sonarr); err != nil {
		t.Fatal(err)
	}
	if err := repos.RadarrRules.Upsert(ctx, radarr); err != nil {
		t.Fatal(err)
	}
}

func TestRuleRepositories_rejectJavaIntegerOverflow_whenWritingPriorityOrOffset(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "rule-overflow.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	repos := store.Repositories()
	cases := []struct {
		name  string
		write func() error
	}{
		{"sonarr priority", func() error {
			return repos.SonarrRules.Upsert(ctx, SonarrRule{ID: "sp", Priority: math.MaxInt32 + 1, Regex: "x", Example: "x", ValidStatus: Valid})
		}},
		{"sonarr offset", func() error {
			return repos.SonarrRules.Upsert(ctx, SonarrRule{ID: "so", Offset: math.MinInt32 - 1, Regex: "x", Example: "x", ValidStatus: Valid})
		}},
		{"radarr priority", func() error {
			return repos.RadarrRules.Upsert(ctx, RadarrRule{ID: "rp", Priority: math.MaxInt32 + 1, Regex: "x", Example: "x", ValidStatus: Valid})
		}},
		{"radarr offset", func() error {
			return repos.RadarrRules.Upsert(ctx, RadarrRule{ID: "ro", Offset: math.MinInt32 - 1, Regex: "x", Example: "x", ValidStatus: Valid})
		}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if err := testCase.write(); !errors.Is(err, ErrJavaIntegerRange) {
				t.Fatalf("write error = %v, want ErrJavaIntegerRange", err)
			}
		})
	}
}
