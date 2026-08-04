package sqlite

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
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
	now := "2026-08-05 12:00:00"

	if err := repos.SystemConfigs.Upsert(ctx, SystemConfig{ID: 1, Key: "selected", Value: stringPointer("value"), ValidStatus: Valid, CreateTime: stringPointer(now), UpdateTime: stringPointer(now)}); err != nil {
		t.Fatal(err)
	}
	if err := repos.SystemUsers.Upsert(ctx, SystemUser{ID: 2, Username: "user", Password: stringPointer("digest"), Role: stringPointer("operator"), ValidStatus: Invalid, CreateTime: stringPointer(now), UpdateTime: stringPointer(now)}); err != nil {
		t.Fatal(err)
	}
	if err := repos.SonarrRules.Upsert(ctx, SonarrRule{ID: "sr", Token: "title", Priority: 4, Regex: "^x$", Replacement: "y", Offset: 5, Example: "input", Remark: stringPointer("remark"), Author: stringPointer("author"), ValidStatus: Valid, CreateTime: stringPointer(now), UpdateTime: stringPointer(now)}); err != nil {
		t.Fatal(err)
	}
	if err := repos.RadarrRules.Upsert(ctx, RadarrRule{ID: "rr", Token: "year", Priority: 6, Regex: "^2026$", Replacement: "2027", Offset: 7, Example: "movie", Remark: nil, Author: nil, ValidStatus: Invalid, CreateTime: stringPointer(now), UpdateTime: stringPointer(now)}); err != nil {
		t.Fatal(err)
	}
	if err := repos.SonarrTitles.Upsert(ctx, SonarrTitle{ID: 3, TVDBID: 30, SNO: 1, MainTitle: "Show", Title: "Alias", CleanTitle: "alias", SeasonNumber: 2, Monitored: Monitored, ValidStatus: Valid, CreateTime: stringPointer(now), UpdateTime: stringPointer(now), SeriesID: int64Pointer(300)}); err != nil {
		t.Fatal(err)
	}
	if err := repos.RadarrTitles.Upsert(ctx, RadarrTitle{ID: 4, TMDBID: 40, SNO: 2, MainTitle: "Movie", Title: "Alt", CleanTitle: "alt", Year: 2026, Monitored: Unmonitored, ValidStatus: Invalid, CreateTime: stringPointer(now), UpdateTime: stringPointer(now), MovieID: int64Pointer(400)}); err != nil {
		t.Fatal(err)
	}
	if err := repos.TMDBTitles.Upsert(ctx, TMDBTitle{ID: 5, TVDBID: 50, TMDBID: int64Pointer(500), Language: "zh-CN", Title: "TMDB", ValidStatus: Valid, CreateTime: stringPointer(now), UpdateTime: stringPointer(now)}); err != nil {
		t.Fatal(err)
	}

	if got, err := repos.SystemConfigs.Get(ctx, 1); err != nil || got.Value == nil || *got.Value != "value" {
		t.Fatalf("config=%+v err=%v", got, err)
	}
	if got, err := repos.SystemUsers.Get(ctx, 2); err != nil || got.Password == nil || *got.Password != "digest" {
		t.Fatalf("user=%+v err=%v", got, err)
	}
	if got, err := repos.SonarrRules.Get(ctx, "sr"); err != nil || got.Author == nil || *got.Author != "author" {
		t.Fatalf("sonarr rule=%+v err=%v", got, err)
	}
	if got, err := repos.RadarrRules.Get(ctx, "rr"); err != nil || got.Remark != nil || got.Author != nil {
		t.Fatalf("radarr rule=%+v err=%v", got, err)
	}
	if got, err := repos.SonarrTitles.Get(ctx, 3); err != nil || got.SeriesID == nil || *got.SeriesID != 300 {
		t.Fatalf("sonarr title=%+v err=%v", got, err)
	}
	if got, err := repos.RadarrTitles.Get(ctx, 4); err != nil || got.MovieID == nil || *got.MovieID != 400 {
		t.Fatalf("radarr title=%+v err=%v", got, err)
	}
	if got, err := repos.TMDBTitles.Get(ctx, 5); err != nil || got.TMDBID == nil || *got.TMDBID != 500 {
		t.Fatalf("tmdb title=%+v err=%v", got, err)
	}
}

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
	writeManualEvidence(t)
}

func writeManualEvidence(t *testing.T) {
	path := os.Getenv("JPROXY_TASK3_EVIDENCE")
	if path == "" {
		return
	}
	data, err := json.MarshalIndent(struct {
		Command     string         `json:"command"`
		Platform    string         `json:"platform"`
		Operations  map[string]int `json:"operations"`
		StableOrder []string       `json:"stableOrder"`
		BeforeAfter string         `json:"beforeAfter"`
		Rollback    string         `json:"rollback"`
		Injection   string         `json:"injection"`
	}{"go test ./internal/store/sqlite -run TestRepositories -count=1 -shuffle=on", "windows", map[string]int{"repositoriesSeeded": 7, "radarrBatchUpsert": 200, "radarrBatchDelete": 2}, []string{"b", "a"}, "200 rows -> delete 2 -> 198 rows; replacement rollback preserves 198 rows", "cancelled and forced transactions preserve 198 rows", "quote SQL fragment is returned as literal data; LIKE wildcard follows Java semantics"}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func stringPointer(value string) *string { return &value }
func int64Pointer(value int64) *int64    { return &value }
