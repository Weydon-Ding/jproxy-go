package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"strconv"
	"testing"
)

func TestDatasetTransaction_upsertsOptionalRemoteRuleInputsAtomically(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "remote-transaction.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	repos := store.Repositories()
	if err := repos.SonarrRules.Upsert(ctx, SonarrRule{ID: "old-sonarr", Token: "old", Regex: "x", Example: "x", ValidStatus: Invalid}); err != nil {
		t.Fatal(err)
	}
	if err := repos.RadarrRules.Upsert(ctx, RadarrRule{ID: "old-radarr", Token: "old", Regex: "x", Example: "x", ValidStatus: Invalid}); err != nil {
		t.Fatal(err)
	}

	sonarrInputs := remoteSonarrRuleInputs(batchLimit + 1)
	sonarrInputs[0].Rule.ID = "old-sonarr"
	radarrInputs := remoteRadarrRuleInputs(batchLimit + 1)
	radarrInputs[0].Rule.ID = "old-radarr"
	err = store.InTransaction(ctx, func(tx DatasetTransaction) error {
		if err := tx.UpsertRemoteSonarrRules(ctx, sonarrInputs); err != nil {
			return err
		}
		return tx.UpsertRemoteRadarrRules(ctx, radarrInputs)
	})

	if err != nil {
		t.Fatal(err)
	}
	sonarr, err := repos.SonarrRules.Get(ctx, "old-sonarr")
	if err != nil || sonarr.ValidStatus != Invalid {
		t.Fatalf("existing Sonarr rule = %+v, error = %v", sonarr, err)
	}
	radarr, err := repos.RadarrRules.Get(ctx, "old-radarr")
	if err != nil || radarr.ValidStatus != Invalid {
		t.Fatalf("existing Radarr rule = %+v, error = %v", radarr, err)
	}
	sonarrNew, err := repos.SonarrRules.Get(ctx, "sonarr-2")
	if err != nil || sonarrNew.ValidStatus != Valid {
		t.Fatalf("new Sonarr rule = %+v, error = %v", sonarrNew, err)
	}
	radarrNew, err := repos.RadarrRules.Get(ctx, "radarr-2")
	if err != nil || radarrNew.ValidStatus != Valid {
		t.Fatalf("new Radarr rule = %+v, error = %v", radarrNew, err)
	}
}

func TestDatasetTransaction_rollsBackRemoteRuleInputs_whenRow201IsInvalid(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "remote-rollback.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	repos := store.Repositories()
	if err := repos.SonarrRules.Upsert(ctx, SonarrRule{ID: "author", Token: "author", Regex: "x", Example: "x", ValidStatus: Invalid}); err != nil {
		t.Fatal(err)
	}
	inputs := remoteSonarrRuleInputs(batchLimit + 1)
	inputs[batchLimit].Rule.Priority = 2147483648

	err = store.InTransaction(ctx, func(tx DatasetTransaction) error {
		return tx.UpsertRemoteSonarrRules(ctx, inputs)
	})

	if !errors.Is(err, ErrJavaIntegerRange) {
		t.Fatalf("transaction error = %v, want ErrJavaIntegerRange", err)
	}
	page, err := repos.SonarrRules.Page(ctx, RuleFilter{})
	if err != nil || page.Total != 1 || page.List[0].ID != "author" || page.List[0].ValidStatus != Invalid {
		t.Fatalf("rules after rollback = %+v, error = %v", page, err)
	}
}

func TestDatasetTransaction_rollsBackRemoteRadarrRuleInputs_whenRow201IsInvalid(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "remote-radarr-rollback.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	repos := store.Repositories()
	if err := repos.RadarrRules.Upsert(ctx, RadarrRule{ID: "author", Token: "author", Regex: "x", Example: "x", ValidStatus: Invalid}); err != nil {
		t.Fatal(err)
	}
	inputs := remoteRadarrRuleInputs(batchLimit + 1)
	inputs[batchLimit].Rule.Offset = -2147483649

	err = store.InTransaction(ctx, func(tx DatasetTransaction) error {
		return tx.UpsertRemoteRadarrRules(ctx, inputs)
	})

	if !errors.Is(err, ErrJavaIntegerRange) {
		t.Fatalf("transaction error = %v, want ErrJavaIntegerRange", err)
	}
	page, err := repos.RadarrRules.Page(ctx, RuleFilter{})
	if err != nil || page.Total != 1 || page.List[0].ID != "author" || page.List[0].ValidStatus != Invalid {
		t.Fatalf("rules after rollback = %+v, error = %v", page, err)
	}
}

func remoteSonarrRuleInputs(length int) []SonarrRuleInput {
	inputs := make([]SonarrRuleInput, length)
	for index := range inputs {
		inputs[index] = SonarrRuleInput{Rule: SonarrRule{ID: RuleID("sonarr-" + strconv.Itoa(index+1)), Token: "title", Regex: "x", Example: "x"}}
	}
	return inputs
}

func remoteRadarrRuleInputs(length int) []RadarrRuleInput {
	inputs := make([]RadarrRuleInput, length)
	for index := range inputs {
		inputs[index] = RadarrRuleInput{Rule: RadarrRule{ID: RuleID("radarr-" + strconv.Itoa(index+1)), Token: "title", Regex: "x", Example: "x"}}
	}
	return inputs
}
