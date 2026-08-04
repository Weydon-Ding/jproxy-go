package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"strconv"
	"testing"
)

func TestRepositoryMethods_pageAndMapperQueries_whenFiltersAndTitlesExist(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "methods.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	repos := store.Repositories()
	for _, row := range []RadarrRule{{ID: "a", Token: " token ", Regex: "a", Example: "a", ValidStatus: Valid}, {ID: "b", Token: "token", Regex: "b", Example: "b", ValidStatus: Valid}} {
		if err := repos.RadarrRules.Upsert(ctx, row); err != nil {
			t.Fatal(err)
		}
	}
	page, err := repos.RadarrRules.Page(ctx, RuleFilter{Page: PageInput{Current: 2, Size: 1}, Token: stringPointer(" token ")})
	if err != nil || page.Total != 2 || len(page.List) != 1 || page.List[0].ID != "a" {
		t.Fatalf("page=%+v err=%v", page, err)
	}
	if err := repos.SonarrTitles.Upsert(ctx, SonarrTitle{ID: 1, TVDBID: 7, MainTitle: "main", Title: "original", CleanTitle: stringPointer("original"), Monitored: Monitored, ValidStatus: Valid}); err != nil {
		t.Fatal(err)
	}
	if err := repos.TMDBTitles.Upsert(ctx, TMDBTitle{ID: 2, TVDBID: 7, Language: "zh", Title: "translated", ValidStatus: Valid}); err != nil {
		t.Fatal(err)
	}
	if err := repos.SonarrTitles.Upsert(ctx, SonarrTitle{ID: 3, TVDBID: 8, MainTitle: "other", Title: "other", CleanTitle: stringPointer("other"), Monitored: Monitored, ValidStatus: Valid}); err != nil {
		t.Fatal(err)
	}
	ids, err := repos.SonarrTitles.NeedTMDBSync(ctx)
	if err != nil || len(ids) != 1 || ids[0] != 8 {
		t.Fatalf("ids=%v err=%v", ids, err)
	}
	joined, err := repos.SonarrTitles.WithTMDBTitles(ctx)
	if err != nil || len(joined) != 3 || joined[0].Title != "translated" || joined[1].Title != "original" || joined[2].Title != "other" {
		t.Fatalf("joined=%+v err=%v", joined, err)
	}
	found, err := repos.TMDBTitles.FindByTVDBID(ctx, 7)
	if err != nil || len(found) != 1 || found[0].Title != "translated" {
		t.Fatalf("found=%+v err=%v", found, err)
	}
}

func TestRepositories_batchesAreAtomic_whenInvalidValueAppearsAfterTwoHundredRows(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "atomic.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	rows := make([]SonarrTitle, 201)
	for index := range rows {
		rows[index] = SonarrTitle{ID: SonarrTitleID(index + 1), TVDBID: int64(index + 1), Title: "title", MainTitle: "main", CleanTitle: stringPointer("clean"), Monitored: Monitored, ValidStatus: Valid}
	}
	rows[200].Monitored = MonitoredStatus(2)
	err = store.Repositories().SonarrTitles.UpsertBatch(ctx, SonarrTitleBatch{Rows: rows})
	if !errors.Is(err, ErrInvalidMonitoredStatus) {
		t.Fatalf("err=%v", err)
	}
	page, err := store.Repositories().SonarrTitles.Page(ctx, SonarrTitleFilter{})
	if err != nil || page.Total != 0 {
		t.Fatalf("page=%+v err=%v", page, err)
	}
}

func TestRepositories_deleteAndSwitch_whenIDsDoNotExistOrStatusIsInvalid(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "delete-switch.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	repo := store.Repositories().SonarrRules
	if err := repo.DeleteBatch(ctx, RuleIDs{IDs: []RuleID{"missing"}}); err != nil {
		t.Fatal(err)
	}
	if err := repo.SwitchValidStatus(ctx, RuleIDs{IDs: []RuleID{"missing"}}, ValidStatus(2)); !errors.Is(err, ErrInvalidValidStatus) {
		t.Fatalf("err=%v", err)
	}
}

func TestStore_rejectsRepositoryOperation_whenClosed(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "closed.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if err := store.Repositories().SystemUsers.Upsert(ctx, SystemUser{ID: 1, Username: "u", ValidStatus: Valid}); err == nil {
		t.Fatal("want closed store error")
	}
}

func TestRepositories_replaceChunks401Rows_whenEveryDatasetExceedsBatchLimit(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "replace-chunks.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	repos := store.Repositories()
	sonarrRules := make([]SonarrRule, 401)
	radarrRules := make([]RadarrRule, 401)
	sonarrTitles := make([]SonarrTitle, 401)
	tmdbTitles := make([]TMDBTitle, 401)
	for index := range sonarrRules {
		id := int64(index + 1)
		sonarrRules[index] = SonarrRule{ID: RuleID("s" + strconv.Itoa(index+1)), Token: "token", Regex: "x", Example: "x", ValidStatus: Valid}
		radarrRules[index] = RadarrRule{ID: RuleID("r" + strconv.Itoa(index+1)), Token: "token", Regex: "x", Example: "x", ValidStatus: Valid}
		sonarrTitles[index] = SonarrTitle{ID: SonarrTitleID(id), TVDBID: id, MainTitle: "main", Title: "title", CleanTitle: stringPointer("clean"), Monitored: Monitored, ValidStatus: Valid}
		tmdbTitles[index] = TMDBTitle{ID: TMDBTitleID(id), TVDBID: id, Language: "zh", Title: "title", ValidStatus: Valid}
	}
	if err := repos.SonarrRules.Replace(ctx, SonarrRuleBatch{Rows: sonarrRules}); err != nil {
		t.Fatal(err)
	}
	if err := repos.RadarrRules.Replace(ctx, RadarrRuleBatch{Rows: radarrRules}); err != nil {
		t.Fatal(err)
	}
	if err := repos.SonarrTitles.Replace(ctx, SonarrTitleBatch{Rows: sonarrTitles}); err != nil {
		t.Fatal(err)
	}
	if err := repos.TMDBTitles.Replace(ctx, TMDBTitleBatch{Rows: tmdbTitles}); err != nil {
		t.Fatal(err)
	}
	sonarrRulesPage, err := repos.SonarrRules.Page(ctx, RuleFilter{Page: PageInput{Size: 200}})
	if err != nil || sonarrRulesPage.Total != 401 {
		t.Fatalf("sonarr rules=%+v err=%v", sonarrRulesPage, err)
	}
	radarrRulesPage, err := repos.RadarrRules.Page(ctx, RuleFilter{Page: PageInput{Size: 200}})
	if err != nil || radarrRulesPage.Total != 401 {
		t.Fatalf("radarr rules=%+v err=%v", radarrRulesPage, err)
	}
	sonarrTitlesPage, err := repos.SonarrTitles.Page(ctx, SonarrTitleFilter{Page: PageInput{Size: 200}})
	if err != nil || sonarrTitlesPage.Total != 401 {
		t.Fatalf("sonarr titles=%+v err=%v", sonarrTitlesPage, err)
	}
	tmdbTitlesPage, err := repos.TMDBTitles.Page(ctx, TMDBTitleFilter{Page: PageInput{Size: 200}})
	if err != nil || tmdbTitlesPage.Total != 401 {
		t.Fatalf("tmdb titles=%+v err=%v", tmdbTitlesPage, err)
	}
}

func TestRepositories_pageUsesStableTiedTimestampOrder_whenSecondPageRequested(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "stable-page.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	stamp := stringPointer("2026-08-05T12:00:00Z")
	for _, row := range []RadarrTitle{{ID: 1, Title: "title", MainTitle: "main", CleanTitle: "clean", Monitored: Monitored, ValidStatus: Valid, UpdateTime: stamp}, {ID: 2, Title: "title", MainTitle: "main", CleanTitle: "clean", Monitored: Monitored, ValidStatus: Valid, UpdateTime: stamp}, {ID: 3, Title: "title", MainTitle: "main", CleanTitle: "clean", Monitored: Monitored, ValidStatus: Valid, UpdateTime: stamp}} {
		if err := store.Repositories().RadarrTitles.Upsert(ctx, row); err != nil {
			t.Fatal(err)
		}
	}
	first, err := store.Repositories().RadarrTitles.Page(ctx, RadarrTitleFilter{Page: PageInput{Current: 1, Size: 2}, Title: stringPointer(" title ")})
	if err != nil || first.Total != 3 || len(first.List) != 2 || first.List[0].ID != 3 || first.List[1].ID != 2 {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	second, err := store.Repositories().RadarrTitles.Page(ctx, RadarrTitleFilter{Page: PageInput{Current: 2, Size: 2}, Title: stringPointer("title")})
	if err != nil || len(second.List) != 1 || second.List[0].ID != 1 {
		t.Fatalf("second=%+v err=%v", second, err)
	}
}

func TestRepositories_pageTitlesAndRollsBackBatches_whenSecondChunkIsInvalid(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "title-pages-and-batches.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	repos := store.Repositories()
	stamp := stringPointer("2026-08-05T12:00:00Z")
	for _, row := range []SonarrTitle{{ID: 1, TVDBID: 10, MainTitle: "main", Title: "alpha", CleanTitle: stringPointer("alpha"), Monitored: Monitored, ValidStatus: Valid, UpdateTime: stamp}, {ID: 2, TVDBID: 11, MainTitle: "main", Title: "alpha", CleanTitle: stringPointer("alpha"), Monitored: Monitored, ValidStatus: Valid, UpdateTime: stamp}} {
		if err := repos.SonarrTitles.Upsert(ctx, row); err != nil {
			t.Fatal(err)
		}
	}
	sonarrPage, err := repos.SonarrTitles.Page(ctx, SonarrTitleFilter{Page: PageInput{Size: 1, Current: 2}, Title: stringPointer(" alpha ")})
	if err != nil || sonarrPage.Total != 2 || len(sonarrPage.List) != 1 || sonarrPage.List[0].ID != 1 {
		t.Fatalf("sonarr page=%+v err=%v", sonarrPage, err)
	}
	for _, row := range []TMDBTitle{{ID: 1, TVDBID: 10, Language: "zh", Title: "alpha", ValidStatus: Valid, UpdateTime: stamp}, {ID: 2, TVDBID: 11, Language: "zh", Title: "alpha", ValidStatus: Valid, UpdateTime: stamp}} {
		if err := repos.TMDBTitles.Upsert(ctx, row); err != nil {
			t.Fatal(err)
		}
	}
	tmdbPage, err := repos.TMDBTitles.Page(ctx, TMDBTitleFilter{Page: PageInput{Size: 1, Current: 2}, Title: stringPointer(" alpha ")})
	if err != nil || tmdbPage.Total != 2 || len(tmdbPage.List) != 1 || tmdbPage.List[0].ID != 1 {
		t.Fatalf("tmdb page=%+v err=%v", tmdbPage, err)
	}

	rules := make([]SonarrRule, 201)
	for index := range rules {
		rules[index] = SonarrRule{ID: RuleID("batch" + strconv.Itoa(index)), Token: "token", Regex: "x", Example: "x", ValidStatus: Valid}
	}
	rules[200].ValidStatus = ValidStatus(2)
	if err := repos.SonarrRules.UpsertBatch(ctx, SonarrRuleBatch{Rows: rules}); !errors.Is(err, ErrInvalidValidStatus) {
		t.Fatalf("rule batch err=%v", err)
	}
	rulesPage, err := repos.SonarrRules.Page(ctx, RuleFilter{Token: stringPointer("batch")})
	if err != nil || rulesPage.Total != 0 {
		t.Fatalf("rules page=%+v err=%v", rulesPage, err)
	}
	titles := make([]TMDBTitle, 201)
	for index := range titles {
		titles[index] = TMDBTitle{ID: TMDBTitleID(index + 10), TVDBID: int64(index + 100), Language: "zh", Title: "batch", ValidStatus: Valid}
	}
	titles[200].ValidStatus = ValidStatus(2)
	if err := repos.TMDBTitles.UpsertBatch(ctx, TMDBTitleBatch{Rows: titles}); !errors.Is(err, ErrInvalidValidStatus) {
		t.Fatalf("tmdb batch err=%v", err)
	}
	batchTitlesPage, err := repos.TMDBTitles.Page(ctx, TMDBTitleFilter{Title: stringPointer("batch")})
	if err != nil || batchTitlesPage.Total != 0 {
		t.Fatalf("batch titles page=%+v err=%v", batchTitlesPage, err)
	}
}
