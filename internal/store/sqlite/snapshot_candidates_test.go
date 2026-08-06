package sqlite

import (
	"context"
	"reflect"
	"testing"

	"jproxy-go/internal/search"
)

func TestStoreFormatterSnapshot_loadsDeterministicSearchCandidates(t *testing.T) {
	// Given
	store := snapshotCandidateStore(t)
	ctx := context.Background()
	cleanTitle := "series"
	seasonCleanTitle := "seriess01"
	for _, title := range []SonarrTitle{
		{ID: 1, TVDBID: 10, SNO: 0, MainTitle: "Series", Title: "Series", CleanTitle: &cleanTitle, SeasonNumber: 1, Monitored: Monitored, ValidStatus: Valid},
		{ID: 2, TVDBID: 10, SNO: 1, MainTitle: "Series", Title: "Series S01", CleanTitle: &seasonCleanTitle, SeasonNumber: 1, Monitored: Unmonitored, ValidStatus: Valid},
	} {
		if err := store.Repositories().SonarrTitles.Upsert(ctx, title); err != nil {
			t.Fatal(err)
		}
	}
	for _, alias := range []TMDBTitle{
		{ID: 1, TVDBID: 10, Language: "zh-CN", Title: "剧集", ValidStatus: Valid},
		{ID: 2, TVDBID: 10, Language: "zh-TW", Title: "劇集", ValidStatus: Valid},
		{ID: 3, TVDBID: 10, Language: "en", Title: "Series", ValidStatus: Valid},
	} {
		if err := store.Repositories().TMDBTitles.Upsert(ctx, alias); err != nil {
			t.Fatal(err)
		}
	}
	for _, title := range []RadarrTitle{
		{ID: 1, TMDBID: 20, SNO: 0, MainTitle: "Movie", Title: "Movie", CleanTitle: "movie", Year: 2024, Monitored: Monitored, ValidStatus: Valid},
		{ID: 2, TMDBID: 20, SNO: 1, MainTitle: "Movie", Title: "Movie", CleanTitle: "movie", Year: 2024, Monitored: Unmonitored, ValidStatus: Valid},
		{ID: 3, TMDBID: 21, SNO: 0, MainTitle: "Ignored", Title: "Ignored", Year: 2025, Monitored: Monitored, ValidStatus: Invalid},
	} {
		if err := store.Repositories().RadarrTitles.Upsert(ctx, title); err != nil {
			t.Fatal(err)
		}
	}

	// When
	snapshot, err := store.FormatterSnapshot(ctx)

	// Then
	wantSonarr := []search.Candidate{
		{Query: "Series", MainTitle: "Series", SeasonNumber: -1},
		{Query: "剧集", MainTitle: "Series", SeasonNumber: -1},
		{Query: "劇集", MainTitle: "Series", SeasonNumber: -1},
		{Query: "Series S01", MainTitle: "Series", SeasonNumber: 1},
	}
	wantRadarr := []search.Candidate{
		{Query: "Movie 2024", MainTitle: "Movie", Year: 2024},
		{Query: "Movie", MainTitle: "Movie", Year: 2024},
	}
	if err != nil || !reflect.DeepEqual(snapshot.SonarrCandidates, wantSonarr) || !reflect.DeepEqual(snapshot.RadarrCandidates, wantRadarr) {
		t.Fatalf("err=%v sonarr=%+v radarr=%+v", err, snapshot.SonarrCandidates, snapshot.RadarrCandidates)
	}
}

func snapshotCandidateStore(t *testing.T) *Store {
	t.Helper()
	store, err := Open(context.Background(), t.TempDir()+"/snapshot-candidates.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	format, clean := "{title}", ""
	configs := []SystemConfig{
		{ID: 1, Key: "radarrIndexerFormat", Value: &format, ValidStatus: Valid},
		{ID: 2, Key: "sonarrIndexerFormat", Value: &format, ValidStatus: Valid},
		{ID: 3, Key: "cleanTitleRegex", Value: &clean, ValidStatus: Valid},
	}
	if err := store.Repositories().SystemConfigs.UpsertBatch(context.Background(), configs); err != nil {
		t.Fatal(err)
	}
	return store
}
