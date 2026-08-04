package sqlite

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSonarrTitles_needTMDBSync_matchesJavaPrimaryLeftJoinSemantics(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "mapper.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	repos := store.Repositories()
	rows := []SonarrTitle{
		{ID: 1, TVDBID: 10, SNO: 0, MainTitle: "primary", Title: "primary", CleanTitle: stringPointer("primary"), SeasonNumber: -1, Monitored: Monitored, ValidStatus: Valid},
		{ID: 2, TVDBID: 10, SNO: 1, MainTitle: "primary", Title: "alternate", CleanTitle: stringPointer("alternate"), SeasonNumber: 1, Monitored: Monitored, ValidStatus: Valid},
		{ID: 3, TVDBID: 20, SNO: 1, MainTitle: "alternate-only", Title: "alternate-only", CleanTitle: stringPointer("alternate-only"), SeasonNumber: 1, Monitored: Monitored, ValidStatus: Valid},
		{ID: 4, TVDBID: 30, SNO: 0, MainTitle: "mapped", Title: "mapped", CleanTitle: stringPointer("mapped"), SeasonNumber: -1, Monitored: Monitored, ValidStatus: Valid},
	}
	if err := repos.SonarrTitles.UpsertBatch(ctx, SonarrTitleBatch{Rows: rows}); err != nil {
		t.Fatal(err)
	}
	if err := repos.TMDBTitles.Upsert(ctx, TMDBTitle{ID: 1, TVDBID: 30, Language: "en", Title: "mapped alias", ValidStatus: Valid}); err != nil {
		t.Fatal(err)
	}

	got, err := repos.SonarrTitles.NeedTMDBSync(ctx)

	if err != nil {
		t.Fatal(err)
	}
	if want := []int64{10}; !reflect.DeepEqual(got, want) {
		t.Fatalf("NeedTMDBSync() = %v, want %v", got, want)
	}
}

func TestSonarrTitles_withTMDBTitles_matchesJavaGroupedUnion(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "union.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	repos := store.Repositories()
	rows := []SonarrTitle{
		{ID: 1, TVDBID: 10, SNO: 0, MainTitle: "Main", Title: "Primary", CleanTitle: stringPointer("same"), SeasonNumber: -1, Monitored: Monitored, ValidStatus: Valid},
		{ID: 2, TVDBID: 10, SNO: 1, MainTitle: "Main", Title: "Alternate", CleanTitle: stringPointer("alternate"), SeasonNumber: 2, Monitored: Monitored, ValidStatus: Valid},
		{ID: 3, TVDBID: 20, SNO: 0, MainTitle: "Other", Title: "Disabled", CleanTitle: stringPointer("other"), SeasonNumber: -1, Monitored: Unmonitored, ValidStatus: Valid},
	}
	if err := repos.SonarrTitles.UpsertBatch(ctx, SonarrTitleBatch{Rows: rows}); err != nil {
		t.Fatal(err)
	}
	aliases := []TMDBTitle{
		{ID: 1, TVDBID: 10, Language: "en", Title: "Alias Longer", ValidStatus: Valid},
		{ID: 2, TVDBID: 10, Language: "fr", Title: "Alias Longer", ValidStatus: Valid},
	}
	if err := repos.TMDBTitles.UpsertBatch(ctx, TMDBTitleBatch{Rows: aliases}); err != nil {
		t.Fatal(err)
	}

	got, err := repos.SonarrTitles.WithTMDBTitles(ctx)

	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 {
		t.Fatalf("WithTMDBTitles() length = %d, want 4: %+v", len(got), got)
	}
	if got[0].Title != "Alias Longer" || got[0].CleanTitle != nil || got[0].SeasonNumber != -1 {
		t.Fatalf("alias = %+v, want Java UNION alias projection", got[0])
	}
	if got[1].Title != "Alternate" || got[2].Title != "Primary" || got[3].Title != "Disabled" {
		t.Fatalf("titles = %+v, want monitored then title-length ordering", got)
	}
}

func TestTMDBTitles_findByTVDBID_returnsAllRowsBeyondPageLimit(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "aliases.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	rows := make([]TMDBTitle, 401)
	for index := range rows {
		rows[index] = TMDBTitle{ID: TMDBTitleID(index + 1), TVDBID: 7, Language: "en", Title: "alias", ValidStatus: Valid}
	}
	if err := store.Repositories().TMDBTitles.UpsertBatch(ctx, TMDBTitleBatch{Rows: rows}); err != nil {
		t.Fatal(err)
	}

	got, err := store.Repositories().TMDBTitles.FindByTVDBID(ctx, 7)

	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 401 {
		t.Fatalf("FindByTVDBID() length = %d, want 401", len(got))
	}
}
