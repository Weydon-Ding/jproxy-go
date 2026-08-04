package sqlite

import (
	"context"
	"encoding/json"
	"errors"
	"os"
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
	radarrRule := RadarrRule{ID: "rr", Token: "year", Priority: 6, Regex: "^2026$", Replacement: "2027", Offset: 7, Example: "movie", Remark: nil, Author: nil, ValidStatus: Invalid, CreateTime: stringPointer(now), UpdateTime: stringPointer(now)}
	sonarrTitle := SonarrTitle{ID: 3, TVDBID: 30, SNO: 1, MainTitle: "Show", Title: "Alias", CleanTitle: "alias", SeasonNumber: 2, Monitored: Monitored, ValidStatus: Valid, CreateTime: stringPointer(now), UpdateTime: stringPointer(now), SeriesID: int64Pointer(300)}
	radarrTitle := RadarrTitle{ID: 4, TMDBID: 40, SNO: 2, MainTitle: "Movie", Title: "Alt", CleanTitle: "alt", Year: 2026, Monitored: Unmonitored, ValidStatus: Invalid, CreateTime: stringPointer(now), UpdateTime: stringPointer(now), MovieID: int64Pointer(400)}
	tmdbTitle := TMDBTitle{ID: 5, TVDBID: 50, TMDBID: int64Pointer(500), Language: "zh-CN", Title: "TMDB", ValidStatus: Valid, CreateTime: stringPointer(now), UpdateTime: stringPointer(now)}

	if err := repos.SystemConfigs.Upsert(ctx, config); err != nil {
		t.Fatal(err)
	}
	if err := repos.SystemUsers.Upsert(ctx, user); err != nil {
		t.Fatal(err)
	}
	if err := repos.SonarrRules.Upsert(ctx, sonarrRule); err != nil {
		t.Fatal(err)
	}
	if err := repos.RadarrRules.Upsert(ctx, radarrRule); err != nil {
		t.Fatal(err)
	}
	if err := repos.SonarrTitles.Upsert(ctx, sonarrTitle); err != nil {
		t.Fatal(err)
	}
	if err := repos.RadarrTitles.Upsert(ctx, radarrTitle); err != nil {
		t.Fatal(err)
	}
	if err := repos.TMDBTitles.Upsert(ctx, tmdbTitle); err != nil {
		t.Fatal(err)
	}

	assertEqualRecord(t, config, mustConfig(t, repos, 1))
	assertEqualRecord(t, user, mustUser(t, repos, 2))
	assertEqualRecord(t, sonarrRule, mustSonarrRule(t, repos, "sr"))
	assertEqualRecord(t, radarrRule, mustRadarrRule(t, repos, "rr"))
	assertEqualRecord(t, sonarrTitle, mustSonarrTitle(t, repos, 3))
	assertEqualRecord(t, radarrTitle, mustRadarrTitle(t, repos, 4))
	assertEqualRecord(t, tmdbTitle, mustTMDBTitle(t, repos, 5))
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

func TestRepositoryEvidence_measuresRealOperations_whenRequested(t *testing.T) {
	path := os.Getenv("JPROXY_TASK3_EVIDENCE")
	if path == "" {
		t.Skip("evidence path is not configured")
	}
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "evidence.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	repos := store.Repositories()
	stamp := "2026-08-05T12:00:00Z"
	config := SystemConfig{ID: 1, Key: "evidence", Value: stringPointer("value"), ValidStatus: Valid, CreateTime: &stamp, UpdateTime: &stamp}
	user := SystemUser{ID: 1, Username: "evidence", Password: stringPointer("digest"), Role: nil, ValidStatus: Valid, CreateTime: &stamp, UpdateTime: &stamp}
	sonarrRule := SonarrRule{ID: "injection' OR 1=1 --", Token: "alpha' OR 1=1 --", Regex: "x", Example: "x", ValidStatus: Valid, CreateTime: &stamp, UpdateTime: &stamp}
	radarrRule := RadarrRule{ID: "r", Token: "title", Regex: "x", Example: "x", ValidStatus: Valid, CreateTime: &stamp, UpdateTime: &stamp}
	sonarrTitle := SonarrTitle{ID: 1, TVDBID: 7, MainTitle: "main", Title: "title", CleanTitle: "clean", SeasonNumber: 2, Monitored: Monitored, ValidStatus: Valid, CreateTime: &stamp, UpdateTime: &stamp, SeriesID: int64Pointer(8)}
	radarrTitle := RadarrTitle{ID: 1, TMDBID: 9, MainTitle: "main", Title: "title", CleanTitle: "clean", Year: 2026, Monitored: Monitored, ValidStatus: Valid, CreateTime: &stamp, UpdateTime: &stamp, MovieID: int64Pointer(10)}
	tmdbTitle := TMDBTitle{ID: 1, TVDBID: 7, TMDBID: int64Pointer(9), Language: "zh", Title: "translated", ValidStatus: Valid, CreateTime: &stamp, UpdateTime: &stamp}
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
	roundTrips := []bool{
		reflect.DeepEqual(config, mustConfig(t, repos, config.ID)),
		reflect.DeepEqual(user, mustUser(t, repos, user.ID)),
		reflect.DeepEqual(sonarrRule, mustSonarrRule(t, repos, sonarrRule.ID)),
		reflect.DeepEqual(radarrRule, mustRadarrRule(t, repos, radarrRule.ID)),
		reflect.DeepEqual(sonarrTitle, mustSonarrTitle(t, repos, sonarrTitle.ID)),
		reflect.DeepEqual(radarrTitle, mustRadarrTitle(t, repos, radarrTitle.ID)),
		reflect.DeepEqual(tmdbTitle, mustTMDBTitle(t, repos, tmdbTitle.ID)),
	}
	for _, roundTrip := range roundTrips {
		if !roundTrip {
			t.Fatal("seeded record did not round-trip")
		}
	}
	if err := repos.SonarrRules.Upsert(ctx, SonarrRule{ID: "wildcard", Token: "alpine", Regex: "x", Example: "x", ValidStatus: Valid, CreateTime: &stamp, UpdateTime: &stamp}); err != nil {
		t.Fatal(err)
	}
	injectionPage, err := repos.SonarrRules.Page(ctx, RuleFilter{Token: stringPointer(sonarrRule.Token)})
	if err != nil {
		t.Fatal(err)
	}
	wildcardPage, err := repos.SonarrRules.Page(ctx, RuleFilter{Token: stringPointer("al%")})
	if err != nil {
		t.Fatal(err)
	}
	before, err := repos.RadarrTitles.Page(ctx, RadarrTitleFilter{})
	if err != nil {
		t.Fatal(err)
	}
	replacementRows := make([]RadarrTitle, batchLimit*2+1)
	for index := range replacementRows {
		replacementRows[index] = RadarrTitle{ID: RadarrTitleID(index + 10), TMDBID: int64(index + 10), MainTitle: "replacement", Title: "replacement", CleanTitle: "replacement", Year: 2026, Monitored: Monitored, ValidStatus: Valid}
	}
	if err := repos.RadarrTitles.Replace(ctx, RadarrTitleBatch{Rows: replacementRows}); err != nil {
		t.Fatal(err)
	}
	replacementPage, err := repos.RadarrTitles.Page(ctx, RadarrTitleFilter{Page: PageInput{Size: int64(len(replacementRows))}})
	if err != nil {
		t.Fatal(err)
	}
	failed := repos.RadarrTitles.Replace(ctx, RadarrTitleBatch{Rows: []RadarrTitle{{ID: 2, Monitored: Monitored, ValidStatus: Valid}, {ID: 3, Monitored: MonitoredStatus(9), ValidStatus: Valid}}})
	if !errors.Is(failed, ErrInvalidMonitoredStatus) {
		t.Fatal(failed)
	}
	afterFailure, err := repos.RadarrTitles.Page(ctx, RadarrTitleFilter{})
	if err != nil {
		t.Fatal(err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	cancelErr := repos.RadarrTitles.Replace(cancelled, RadarrTitleBatch{Rows: []RadarrTitle{{ID: 4, Monitored: Monitored, ValidStatus: Valid}}})
	afterCancel, err := repos.RadarrTitles.Page(ctx, RadarrTitleFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if err := repos.SystemConfigs.Upsert(ctx, SystemConfig{ID: 1, Key: "evidence", Value: stringPointer("next"), ValidStatus: Valid}); err != nil {
		t.Fatal(err)
	}
	updated := mustConfig(t, repos, 1)
	evidence := map[string]any{
		"operationsExecuted":               len(operations),
		"roundTrips":                       roundTrips,
		"injectionLiteralMatches":          injectionPage.Total,
		"wildcardMatches":                  wildcardPage.Total,
		"replacementRowsPersisted":         replacementPage.Total,
		"batchFailureBeforeTotal":          before.Total,
		"batchFailureAfterTotal":           afterFailure.Total,
		"cancellationBeforeTotal":          afterFailure.Total,
		"cancellationAfterTotal":           afterCancel.Total,
		"cancellationReturnedContextError": errors.Is(cancelErr, context.Canceled),
		"timestampCreatePreserved":         updated.CreateTime != nil && *updated.CreateTime == stamp,
		"timestampUpdateRefreshed":         updated.UpdateTime != nil && *updated.UpdateTime != stamp,
	}
	data, err := json.MarshalIndent(evidence, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
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
