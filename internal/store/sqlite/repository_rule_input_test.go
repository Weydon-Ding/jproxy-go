package sqlite

import (
	"context"
	"path/filepath"
	"testing"
)

func TestRuleRepositories_preserveDisabledStatus_whenRemoteInputOmitsStatus(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "remote-rules.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	repos := store.Repositories()
	for _, rule := range []SonarrRule{{ID: "sonarr", Token: "title", Regex: "x", Example: "x", ValidStatus: Invalid}} {
		if err := repos.SonarrRules.Upsert(ctx, rule); err != nil {
			t.Fatal(err)
		}
	}
	if err := repos.RadarrRules.Upsert(ctx, RadarrRule{ID: "radarr", Token: "title", Regex: "x", Example: "x", ValidStatus: Invalid}); err != nil {
		t.Fatal(err)
	}

	err = repos.SonarrRules.UpsertRemote(ctx, SonarrRuleInput{Rule: SonarrRule{ID: "sonarr", Token: "updated", Regex: "x", Example: "x"}})
	if err != nil {
		t.Fatal(err)
	}
	err = repos.RadarrRules.UpsertRemote(ctx, RadarrRuleInput{Rule: RadarrRule{ID: "radarr", Token: "updated", Regex: "x", Example: "x"}})
	if err != nil {
		t.Fatal(err)
	}

	sonarr, sonarrErr := repos.SonarrRules.Get(ctx, "sonarr")
	radarr, radarrErr := repos.RadarrRules.Get(ctx, "radarr")
	if sonarrErr != nil || radarrErr != nil || sonarr.ValidStatus != Invalid || radarr.ValidStatus != Invalid {
		t.Fatalf("remote upsert statuses = %d/%d, errors = %v/%v", sonarr.ValidStatus, radarr.ValidStatus, sonarrErr, radarrErr)
	}
}
