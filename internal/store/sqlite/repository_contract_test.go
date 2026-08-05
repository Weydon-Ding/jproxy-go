package sqlite_test

import (
	"context"
	"path/filepath"
	"testing"

	"jproxy-go/internal/store/sqlite"
)

type configFake struct{}
type userFake struct{}
type sonarrRuleFake struct{}
type radarrRuleFake struct{}
type sonarrTitleFake struct{}
type radarrTitleFake struct{}
type tmdbTitleFake struct{}

func (configFake) Get(context.Context, sqlite.SystemConfigID) (sqlite.SystemConfig, error) {
	return sqlite.SystemConfig{}, nil
}
func (configFake) Upsert(context.Context, sqlite.SystemConfig) error        { return nil }
func (configFake) UpsertBatch(context.Context, []sqlite.SystemConfig) error { return nil }
func (configFake) List(context.Context) ([]sqlite.SystemConfig, error)      { return nil, nil }
func (configFake) ValueByKey(context.Context, string) (string, error)       { return "", nil }

func (userFake) Get(context.Context, sqlite.SystemUserID) (sqlite.SystemUser, error) {
	return sqlite.SystemUser{}, nil
}
func (userFake) Upsert(context.Context, sqlite.SystemUser) error { return nil }
func (userFake) FindByUsername(context.Context, string) (sqlite.SystemUser, error) {
	return sqlite.SystemUser{}, nil
}

func (sonarrRuleFake) Get(context.Context, sqlite.RuleID) (sqlite.SonarrRule, error) {
	return sqlite.SonarrRule{}, nil
}
func (sonarrRuleFake) Upsert(context.Context, sqlite.SonarrRule) error            { return nil }
func (sonarrRuleFake) UpsertRemote(context.Context, sqlite.SonarrRuleInput) error { return nil }
func (sonarrRuleFake) Page(context.Context, sqlite.RuleFilter) (sqlite.PageResult[sqlite.SonarrRule], error) {
	return sqlite.PageResult[sqlite.SonarrRule]{}, nil
}
func (sonarrRuleFake) UpsertBatch(context.Context, sqlite.SonarrRuleBatch) error { return nil }
func (sonarrRuleFake) DeleteBatch(context.Context, sqlite.RuleIDs) error         { return nil }
func (sonarrRuleFake) SwitchValidStatus(context.Context, sqlite.RuleIDs, sqlite.ValidStatus) error {
	return nil
}
func (sonarrRuleFake) Replace(context.Context, sqlite.SonarrRuleBatch) error { return nil }
func (sonarrRuleFake) Export(context.Context, sqlite.RuleIDs) ([]sqlite.SonarrRule, error) {
	return nil, nil
}
func (sonarrRuleFake) Tokens(context.Context) ([]string, error)             { return nil, nil }
func (sonarrRuleFake) Import(context.Context, sqlite.SonarrRuleBatch) error { return nil }

func (radarrRuleFake) Get(context.Context, sqlite.RuleID) (sqlite.RadarrRule, error) {
	return sqlite.RadarrRule{}, nil
}
func (radarrRuleFake) Upsert(context.Context, sqlite.RadarrRule) error            { return nil }
func (radarrRuleFake) UpsertRemote(context.Context, sqlite.RadarrRuleInput) error { return nil }
func (radarrRuleFake) Page(context.Context, sqlite.RuleFilter) (sqlite.PageResult[sqlite.RadarrRule], error) {
	return sqlite.PageResult[sqlite.RadarrRule]{}, nil
}
func (radarrRuleFake) UpsertBatch(context.Context, sqlite.RadarrRuleBatch) error { return nil }
func (radarrRuleFake) DeleteBatch(context.Context, sqlite.RuleIDs) error         { return nil }
func (radarrRuleFake) SwitchValidStatus(context.Context, sqlite.RuleIDs, sqlite.ValidStatus) error {
	return nil
}
func (radarrRuleFake) Replace(context.Context, sqlite.RadarrRuleBatch) error { return nil }
func (radarrRuleFake) Export(context.Context, sqlite.RuleIDs) ([]sqlite.RadarrRule, error) {
	return nil, nil
}
func (radarrRuleFake) Tokens(context.Context) ([]string, error)             { return nil, nil }
func (radarrRuleFake) Import(context.Context, sqlite.RadarrRuleBatch) error { return nil }

func (sonarrTitleFake) Get(context.Context, sqlite.SonarrTitleID) (sqlite.SonarrTitle, error) {
	return sqlite.SonarrTitle{}, nil
}
func (sonarrTitleFake) Upsert(context.Context, sqlite.SonarrTitle) error { return nil }
func (sonarrTitleFake) Page(context.Context, sqlite.SonarrTitleFilter) (sqlite.PageResult[sqlite.SonarrTitle], error) {
	return sqlite.PageResult[sqlite.SonarrTitle]{}, nil
}
func (sonarrTitleFake) UpsertBatch(context.Context, sqlite.SonarrTitleBatch) error   { return nil }
func (sonarrTitleFake) DeleteBatch(context.Context, sqlite.SonarrTitleIDs) error     { return nil }
func (sonarrTitleFake) Replace(context.Context, sqlite.SonarrTitleBatch) error       { return nil }
func (sonarrTitleFake) NeedTMDBSync(context.Context) ([]int64, error)                { return nil, nil }
func (sonarrTitleFake) WithTMDBTitles(context.Context) ([]sqlite.SonarrTitle, error) { return nil, nil }

func (radarrTitleFake) Get(context.Context, sqlite.RadarrTitleID) (sqlite.RadarrTitle, error) {
	return sqlite.RadarrTitle{}, nil
}
func (radarrTitleFake) Upsert(context.Context, sqlite.RadarrTitle) error { return nil }
func (radarrTitleFake) Page(context.Context, sqlite.RadarrTitleFilter) (sqlite.PageResult[sqlite.RadarrTitle], error) {
	return sqlite.PageResult[sqlite.RadarrTitle]{}, nil
}
func (radarrTitleFake) UpsertBatch(context.Context, sqlite.RadarrTitleBatch) error { return nil }
func (radarrTitleFake) DeleteBatch(context.Context, sqlite.RadarrTitleIDs) error   { return nil }
func (radarrTitleFake) Replace(context.Context, sqlite.RadarrTitleBatch) error     { return nil }

func (tmdbTitleFake) Get(context.Context, sqlite.TMDBTitleID) (sqlite.TMDBTitle, error) {
	return sqlite.TMDBTitle{}, nil
}
func (tmdbTitleFake) Upsert(context.Context, sqlite.TMDBTitle) error { return nil }
func (tmdbTitleFake) Save(context.Context, sqlite.TMDBTitleSaveInput) (sqlite.TMDBTitleSaveResult, error) {
	return sqlite.TMDBTitleSaveResult{}, nil
}
func (tmdbTitleFake) Page(context.Context, sqlite.TMDBTitleFilter) (sqlite.PageResult[sqlite.TMDBTitle], error) {
	return sqlite.PageResult[sqlite.TMDBTitle]{}, nil
}
func (tmdbTitleFake) UpsertBatch(context.Context, sqlite.TMDBTitleBatch) error { return nil }
func (tmdbTitleFake) DeleteBatch(context.Context, sqlite.TMDBTitleIDs) error   { return nil }
func (tmdbTitleFake) Replace(context.Context, sqlite.TMDBTitleBatch) error     { return nil }
func (tmdbTitleFake) FindByTVDBID(context.Context, int64) ([]sqlite.TMDBTitle, error) {
	return nil, nil
}

func TestRepositoryContracts_areExportedAndFakeable(_ *testing.T) {
	var config sqlite.SystemConfigRepository = configFake{}
	var user sqlite.SystemUserRepository = userFake{}
	var sonarrRule sqlite.SonarrRuleRepository = sonarrRuleFake{}
	var radarrRule sqlite.RadarrRuleRepository = radarrRuleFake{}
	var sonarrTitle sqlite.SonarrTitleRepository = sonarrTitleFake{}
	var radarrTitle sqlite.RadarrTitleRepository = radarrTitleFake{}
	var tmdbTitle sqlite.TMDBTitleRepository = tmdbTitleFake{}
	repositories := sqlite.Repositories{SystemConfigs: config, SystemUsers: user, SonarrRules: sonarrRule, RadarrRules: radarrRule, SonarrTitles: sonarrTitle, RadarrTitles: radarrTitle, TMDBTitles: tmdbTitle}
	_ = repositories
}

func TestRepositories_areUsableThroughExternalStoreConsumer(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "consumer.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	repositories := store.Repositories()
	if err := repositories.SystemConfigs.Upsert(ctx, sqlite.SystemConfig{ID: 1, Key: "k", ValidStatus: sqlite.Valid}); err != nil {
		t.Fatal(err)
	}
	if err := repositories.SystemUsers.Upsert(ctx, sqlite.SystemUser{ID: 1, Username: "u", ValidStatus: sqlite.Valid}); err != nil {
		t.Fatal(err)
	}
	if err := repositories.SonarrRules.Upsert(ctx, sqlite.SonarrRule{ID: "s", Token: "t", Regex: "x", Example: "x", ValidStatus: sqlite.Valid}); err != nil {
		t.Fatal(err)
	}
	if err := repositories.RadarrRules.Upsert(ctx, sqlite.RadarrRule{ID: "r", Token: "t", Regex: "x", Example: "x", ValidStatus: sqlite.Valid}); err != nil {
		t.Fatal(err)
	}
	cleanTitle := "c"
	if err := repositories.SonarrTitles.Upsert(ctx, sqlite.SonarrTitle{ID: 1, TVDBID: 1, MainTitle: "m", Title: "t", CleanTitle: &cleanTitle, Monitored: sqlite.Monitored, ValidStatus: sqlite.Valid}); err != nil {
		t.Fatal(err)
	}
	if err := repositories.RadarrTitles.Upsert(ctx, sqlite.RadarrTitle{ID: 1, TMDBID: 1, MainTitle: "m", Title: "t", CleanTitle: "c", Monitored: sqlite.Monitored, ValidStatus: sqlite.Valid}); err != nil {
		t.Fatal(err)
	}
	if err := repositories.TMDBTitles.Upsert(ctx, sqlite.TMDBTitle{ID: 1, TVDBID: 1, Language: "en", Title: "t", ValidStatus: sqlite.Valid}); err != nil {
		t.Fatal(err)
	}
}
