package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func TestSystemConfigUpsertBatch_rollsBack_whenSecondRowViolatesConstraint(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "config-batch.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	valid := Valid
	err = store.Repositories().SystemConfigs.UpsertBatch(ctx, []SystemConfig{{ID: 1, Key: "first", ValidStatus: valid}, {ID: 2, Key: "second", ValidStatus: ValidStatus(9)}})
	if err == nil {
		t.Fatal("want constraint failure")
	}
	configs, err := store.Repositories().SystemConfigs.List(ctx)
	if err != nil || len(configs) != 0 {
		t.Fatalf("configs=%+v err=%v", configs, err)
	}
}

func TestRepositories_rejectInvalidStatus_whenWriting(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "invalid-status.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	invalid := ValidStatus(9)
	err = store.Repositories().SystemConfigs.Upsert(ctx, SystemConfig{ID: 1, Key: "k", ValidStatus: invalid})
	if !errors.Is(err, ErrInvalidValidStatus) {
		t.Fatalf("err=%v", err)
	}
}

func TestSystemConfigUpsert_preservesCreateTimeAndRefreshesUpdateTime_whenExisting(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "timestamps.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	oldCreate, oldUpdate := "2001-01-01T00:00:00Z", "2002-01-01T00:00:00Z"
	if err := store.Repositories().SystemConfigs.Upsert(ctx, SystemConfig{ID: 1, Key: "k", Value: stringPointer("old"), ValidStatus: Valid, CreateTime: &oldCreate, UpdateTime: &oldUpdate}); err != nil {
		t.Fatal(err)
	}
	if err := store.Repositories().SystemConfigs.Upsert(ctx, SystemConfig{ID: 1, Key: "k", Value: stringPointer("new"), ValidStatus: Valid}); err != nil {
		t.Fatal(err)
	}
	got, err := store.Repositories().SystemConfigs.Get(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if got.CreateTime == nil || *got.CreateTime != oldCreate {
		if got.CreateTime == nil {
			t.Fatal("create_time is nil")
		}
		t.Fatalf("create_time=%q want=%q", *got.CreateTime, oldCreate)
	}
	if got.UpdateTime == nil || *got.UpdateTime == oldUpdate {
		if got.UpdateTime == nil {
			t.Fatal("update_time is nil")
		}
		t.Fatalf("update_time=%q old=%q", *got.UpdateTime, oldUpdate)
	}
}

func TestRepositories_preserveCreateTimeAndRefreshUpdateTime_whenEveryTableUpdates(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "all-timestamps.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	createTime := "2001-01-01T00:00:00Z"
	updateTime := "2002-01-01T00:00:00Z"
	repos := store.Repositories()

	if err := repos.SystemUsers.Upsert(ctx, SystemUser{ID: 1, Username: "old-user", ValidStatus: Valid, CreateTime: &createTime, UpdateTime: &updateTime}); err != nil {
		t.Fatal(err)
	}
	if err := repos.SystemUsers.Upsert(ctx, SystemUser{ID: 1, Username: "new-user", ValidStatus: Valid}); err != nil {
		t.Fatal(err)
	}
	user, err := repos.SystemUsers.Get(ctx, 1)
	if err != nil || user.CreateTime == nil || user.UpdateTime == nil || *user.CreateTime != createTime || *user.UpdateTime == updateTime {
		t.Fatalf("user=%+v err=%v", user, err)
	}

	if err := repos.SonarrRules.Upsert(ctx, SonarrRule{ID: "s", Token: "title", Regex: "x", Example: "x", ValidStatus: Valid, CreateTime: &createTime, UpdateTime: &updateTime}); err != nil {
		t.Fatal(err)
	}
	if err := repos.SonarrRules.Upsert(ctx, SonarrRule{ID: "s", Token: "title", Regex: "y", Example: "y", ValidStatus: Valid}); err != nil {
		t.Fatal(err)
	}
	rule, err := repos.SonarrRules.Get(ctx, "s")
	if err != nil || rule.CreateTime == nil || rule.UpdateTime == nil || *rule.CreateTime != createTime || *rule.UpdateTime == updateTime {
		t.Fatalf("rule=%+v err=%v", rule, err)
	}

	if err := repos.RadarrRules.Upsert(ctx, RadarrRule{ID: "r", Token: "year", Regex: "x", Example: "x", ValidStatus: Valid, CreateTime: &createTime, UpdateTime: &updateTime}); err != nil {
		t.Fatal(err)
	}
	if err := repos.RadarrRules.Upsert(ctx, RadarrRule{ID: "r", Token: "year", Regex: "y", Example: "y", ValidStatus: Valid}); err != nil {
		t.Fatal(err)
	}
	radarrRule, err := repos.RadarrRules.Get(ctx, "r")
	if err != nil || radarrRule.CreateTime == nil || radarrRule.UpdateTime == nil || *radarrRule.CreateTime != createTime || *radarrRule.UpdateTime == updateTime {
		t.Fatalf("radarr rule=%+v err=%v", radarrRule, err)
	}

	if err := repos.SonarrTitles.Upsert(ctx, SonarrTitle{ID: 1, Title: "old", MainTitle: "old", CleanTitle: stringPointer("old"), Monitored: Monitored, ValidStatus: Valid, CreateTime: &createTime, UpdateTime: &updateTime}); err != nil {
		t.Fatal(err)
	}
	if err := repos.SonarrTitles.Upsert(ctx, SonarrTitle{ID: 1, Title: "new", MainTitle: "new", CleanTitle: stringPointer("new"), Monitored: Monitored, ValidStatus: Valid}); err != nil {
		t.Fatal(err)
	}
	sonarrTitle, err := repos.SonarrTitles.Get(ctx, 1)
	if err != nil || sonarrTitle.CreateTime == nil || sonarrTitle.UpdateTime == nil || *sonarrTitle.CreateTime != createTime || *sonarrTitle.UpdateTime == updateTime {
		t.Fatalf("sonarr title=%+v err=%v", sonarrTitle, err)
	}

	if err := repos.RadarrTitles.Upsert(ctx, RadarrTitle{ID: 1, Title: "old", MainTitle: "old", CleanTitle: "old", Monitored: Monitored, ValidStatus: Valid, CreateTime: &createTime, UpdateTime: &updateTime}); err != nil {
		t.Fatal(err)
	}
	if err := repos.RadarrTitles.Upsert(ctx, RadarrTitle{ID: 1, Title: "new", MainTitle: "new", CleanTitle: "new", Monitored: Monitored, ValidStatus: Valid}); err != nil {
		t.Fatal(err)
	}
	radarrTitle, err := repos.RadarrTitles.Get(ctx, 1)
	if err != nil || radarrTitle.CreateTime == nil || radarrTitle.UpdateTime == nil || *radarrTitle.CreateTime != createTime || *radarrTitle.UpdateTime == updateTime {
		t.Fatalf("radarr title=%+v err=%v", radarrTitle, err)
	}

	if err := repos.TMDBTitles.Upsert(ctx, TMDBTitle{ID: 1, Title: "old", Language: "zh", ValidStatus: Valid, CreateTime: &createTime, UpdateTime: &updateTime}); err != nil {
		t.Fatal(err)
	}
	if err := repos.TMDBTitles.Upsert(ctx, TMDBTitle{ID: 1, Title: "new", Language: "zh", ValidStatus: Valid}); err != nil {
		t.Fatal(err)
	}
	tmdbTitle, err := repos.TMDBTitles.Get(ctx, 1)
	if err != nil || tmdbTitle.CreateTime == nil || tmdbTitle.UpdateTime == nil || *tmdbTitle.CreateTime != createTime || *tmdbTitle.UpdateTime == updateTime {
		t.Fatalf("tmdb title=%+v err=%v", tmdbTitle, err)
	}
}

func TestRepositories_rejectInvalidStatuses_whenRepresentativeWritesRun(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "invalid-statuses.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	repos := store.Repositories()
	if err := repos.SystemUsers.Upsert(ctx, SystemUser{ID: 1, Username: "user", ValidStatus: ValidStatus(2)}); !errors.Is(err, ErrInvalidValidStatus) {
		t.Fatalf("user err=%v", err)
	}
	if err := repos.SonarrRules.Upsert(ctx, SonarrRule{ID: "rule", ValidStatus: ValidStatus(2)}); !errors.Is(err, ErrInvalidValidStatus) {
		t.Fatalf("rule err=%v", err)
	}
	if err := repos.RadarrTitles.Upsert(ctx, RadarrTitle{ID: 1, Monitored: MonitoredStatus(2), ValidStatus: Valid}); !errors.Is(err, ErrInvalidMonitoredStatus) {
		t.Fatalf("title err=%v", err)
	}
}
