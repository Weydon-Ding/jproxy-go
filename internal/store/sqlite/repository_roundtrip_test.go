package sqlite

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"
)

func TestRepositories_roundTripEveryJavaField_whenSeeded(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "repositories.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	repos := store.Repositories()
	now := "2026-08-05T12:00:00Z"
	config := SystemConfig{ID: 1, Key: "selected", Value: stringPointer("value"), ValidStatus: Valid, CreateTime: stringPointer(now), UpdateTime: stringPointer(now)}
	user := SystemUser{ID: 2, Username: "user", Password: stringPointer("digest"), Role: stringPointer("operator"), ValidStatus: Invalid, CreateTime: stringPointer(now), UpdateTime: stringPointer(now)}
	sonarrRule := SonarrRule{ID: "sr", Token: "title", Priority: 4, Regex: "^x$", Replacement: "y", Offset: 5, Example: "input", Remark: stringPointer("remark"), Author: stringPointer("author"), ValidStatus: Valid, CreateTime: stringPointer(now), UpdateTime: stringPointer(now)}
	radarrRule := RadarrRule{ID: "rr", Token: "year", Priority: 6, Regex: "^2026$", Replacement: "2027", Offset: 7, Example: "movie", ValidStatus: Invalid, CreateTime: stringPointer(now), UpdateTime: stringPointer(now)}
	sonarrTitle := SonarrTitle{ID: 3, TVDBID: 30, SNO: 1, MainTitle: "Show", Title: "Alias", CleanTitle: stringPointer("alias"), SeasonNumber: 2, Monitored: Monitored, ValidStatus: Valid, CreateTime: stringPointer(now), UpdateTime: stringPointer(now), SeriesID: int64Pointer(300)}
	radarrTitle := RadarrTitle{ID: 4, TMDBID: 40, SNO: 2, MainTitle: "Movie", Title: "Alt", CleanTitle: "alt", Year: 2026, Monitored: Unmonitored, ValidStatus: Invalid, CreateTime: stringPointer(now), UpdateTime: stringPointer(now), MovieID: int64Pointer(400)}
	tmdbTitle := TMDBTitle{ID: 5, TVDBID: 50, TMDBID: int64Pointer(500), Language: "zh-CN", Title: "TMDB", ValidStatus: Valid, CreateTime: stringPointer(now), UpdateTime: stringPointer(now)}
	operations := []func() error{
		func() error { return repos.SystemConfigs.Upsert(ctx, config) },
		func() error { return repos.SystemUsers.Upsert(ctx, user) },
		func() error { return repos.SonarrRules.Upsert(ctx, sonarrRule) },
		func() error { return repos.RadarrRules.Upsert(ctx, radarrRule) },
		func() error { return repos.SonarrTitles.Upsert(ctx, sonarrTitle) },
		func() error { return repos.RadarrTitles.Upsert(ctx, radarrTitle) },
		func() error { return repos.TMDBTitles.Upsert(ctx, tmdbTitle) },
	}
	for _, operation := range operations {
		if err := operation(); err != nil {
			t.Fatal(err)
		}
	}
	assertEqualRecord(t, config, mustConfig(t, repos, config.ID))
	assertEqualRecord(t, user, mustUser(t, repos, user.ID))
	assertEqualRecord(t, sonarrRule, mustSonarrRule(t, repos, sonarrRule.ID))
	assertEqualRecord(t, radarrRule, mustRadarrRule(t, repos, radarrRule.ID))
	assertEqualRecord(t, sonarrTitle, mustSonarrTitle(t, repos, sonarrTitle.ID))
	assertEqualRecord(t, radarrTitle, mustRadarrTitle(t, repos, radarrTitle.ID))
	assertEqualRecord(t, tmdbTitle, mustTMDBTitle(t, repos, tmdbTitle.ID))
}

func assertEqualRecord[T any](t *testing.T, want, got T) {
	t.Helper()
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("roundtrip got=%+v want=%+v", got, want)
	}
}

func mustConfig(t *testing.T, repos Repositories, id SystemConfigID) SystemConfig {
	t.Helper()
	value, err := repos.SystemConfigs.Get(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return value
}
func mustUser(t *testing.T, repos Repositories, id SystemUserID) SystemUser {
	t.Helper()
	value, err := repos.SystemUsers.Get(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return value
}
func mustSonarrRule(t *testing.T, repos Repositories, id RuleID) SonarrRule {
	t.Helper()
	value, err := repos.SonarrRules.Get(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return value
}
func mustRadarrRule(t *testing.T, repos Repositories, id RuleID) RadarrRule {
	t.Helper()
	value, err := repos.RadarrRules.Get(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return value
}
func mustSonarrTitle(t *testing.T, repos Repositories, id SonarrTitleID) SonarrTitle {
	t.Helper()
	value, err := repos.SonarrTitles.Get(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return value
}
func mustRadarrTitle(t *testing.T, repos Repositories, id RadarrTitleID) RadarrTitle {
	t.Helper()
	value, err := repos.RadarrTitles.Get(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return value
}
func mustTMDBTitle(t *testing.T, repos Repositories, id TMDBTitleID) TMDBTitle {
	t.Helper()
	value, err := repos.TMDBTitles.Get(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return value
}
