package sqlite

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type repositoryEvidence struct {
	OperationsExecuted                   int    `json:"operationsExecuted"`
	RoundTrips                           []bool `json:"roundTrips"`
	InjectionLiteralMatches              int64  `json:"injectionLiteralMatches"`
	WildcardMatches                      int64  `json:"wildcardMatches"`
	ReplacementRowsPersisted             int64  `json:"replacementRowsPersisted"`
	Row201FailureBaselineTotal           int64  `json:"row201FailureBaselineTotal"`
	Row201FailureAfterRollbackTotal      int64  `json:"row201FailureAfterRollbackTotal"`
	CancellationBaselineTotal            int64  `json:"cancellationBaselineTotal"`
	CancellationAfterRollbackTotal       int64  `json:"cancellationAfterRollbackTotal"`
	CancellationReturnedContextError     bool   `json:"cancellationReturnedContextError"`
	CancellationWroteFirstReplacementRow bool   `json:"cancellationWroteFirstReplacementRow"`
	TimestampCreatePreserved             bool   `json:"timestampCreatePreserved"`
	TimestampUpdateRefreshed             bool   `json:"timestampUpdateRefreshed"`
	RemoteBatchPreservesDisabledStatus   bool   `json:"remoteBatchPreservesDisabledStatus"`
	RemoteBatchDefaultsNewStatus         bool   `json:"remoteBatchDefaultsNewStatus"`
	RemoteBatchRollbackPreservesAuthor   bool   `json:"remoteBatchRollbackPreservesAuthor"`
}

func TestRepositoryEvidence_writesMeasuredOperations_whenRequested(t *testing.T) {
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
	stamp := "2001-01-01T00:00:00Z"
	operations := []func() error{
		func() error {
			return repos.SystemConfigs.Upsert(ctx, SystemConfig{ID: 1, Key: "config", Value: stringPointer("value"), ValidStatus: Valid, CreateTime: &stamp, UpdateTime: &stamp})
		},
		func() error {
			return repos.SystemUsers.Upsert(ctx, SystemUser{ID: 1, Username: "user", ValidStatus: Valid, CreateTime: &stamp, UpdateTime: &stamp})
		},
		func() error {
			return repos.SonarrRules.Upsert(ctx, SonarrRule{ID: "quote' OR 1=1 --", Token: "quote' OR 1=1 --", Regex: "x", Example: "x", ValidStatus: Valid, CreateTime: &stamp, UpdateTime: &stamp})
		},
		func() error {
			return repos.RadarrRules.Upsert(ctx, RadarrRule{ID: "radarr", Token: "title", Regex: "x", Example: "x", ValidStatus: Valid, CreateTime: &stamp, UpdateTime: &stamp})
		},
		func() error {
			return repos.SonarrTitles.Upsert(ctx, SonarrTitle{ID: 1, TVDBID: 1, MainTitle: "sonarr", Title: "sonarr", CleanTitle: stringPointer("sonarr"), Monitored: Monitored, ValidStatus: Valid, CreateTime: &stamp, UpdateTime: &stamp})
		},
		func() error {
			return repos.RadarrTitles.Upsert(ctx, RadarrTitle{ID: 1, TMDBID: 1, MainTitle: "radarr", Title: "radarr", CleanTitle: "radarr", Monitored: Monitored, ValidStatus: Valid, CreateTime: &stamp, UpdateTime: &stamp})
		},
		func() error {
			return repos.TMDBTitles.Upsert(ctx, TMDBTitle{ID: 1, TVDBID: 1, Language: "en", Title: "tmdb", ValidStatus: Valid, CreateTime: &stamp, UpdateTime: &stamp})
		},
	}
	for _, operation := range operations {
		if err := operation(); err != nil {
			t.Fatal(err)
		}
	}
	roundTrips := []bool{
		mustConfig(t, repos, 1).Key == "config",
		mustUser(t, repos, 1).Username == "user",
		mustSonarrRule(t, repos, "quote' OR 1=1 --").Token == "quote' OR 1=1 --",
		mustRadarrRule(t, repos, "radarr").Token == "title",
		mustSonarrTitle(t, repos, 1).Title == "sonarr",
		mustRadarrTitle(t, repos, 1).ID == 1,
		mustTMDBTitle(t, repos, 1).Title == "tmdb",
	}
	if err := repos.SonarrRules.Upsert(ctx, SonarrRule{ID: "wildcard", Token: "alpine", Regex: "x", Example: "x", ValidStatus: Valid}); err != nil {
		t.Fatal(err)
	}
	injection, err := repos.SonarrRules.Page(ctx, RuleFilter{Token: stringPointer("quote' OR 1=1 --")})
	if err != nil {
		t.Fatal(err)
	}
	wildcard, err := repos.SonarrRules.Page(ctx, RuleFilter{Token: stringPointer("al%")})
	if err != nil {
		t.Fatal(err)
	}
	if err := repos.RadarrTitles.Replace(ctx, RadarrTitleBatch{Rows: validRadarrTitles(batchLimit*2 + 1)}); err != nil {
		t.Fatal(err)
	}
	replaced, err := repos.RadarrTitles.Page(ctx, RadarrTitleFilter{Page: PageInput{Size: batchLimit}})
	if err != nil {
		t.Fatal(err)
	}
	baseline, err := repos.RadarrTitles.Page(ctx, RadarrTitleFilter{})
	if err != nil {
		t.Fatal(err)
	}
	invalidRows := validRadarrTitles(batchLimit + 1)
	invalidRows[batchLimit].ValidStatus = ValidStatus(2)
	if err := repos.RadarrTitles.Replace(ctx, RadarrTitleBatch{Rows: invalidRows}); !errors.Is(err, ErrInvalidValidStatus) {
		t.Fatalf("row-201 replacement error = %v", err)
	}
	afterFailure, err := repos.RadarrTitles.Page(ctx, RadarrTitleFilter{})
	if err != nil {
		t.Fatal(err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	defer cancel()
	firstRowWritten := false
	cancelErr := store.InTransaction(ctx, func(tx DatasetTransaction) error {
		if err := tx.ReplaceRadarrTitles(ctx, RadarrTitleBatch{Rows: []RadarrTitle{{ID: 999, MainTitle: "new", Title: "new", CleanTitle: "new", Monitored: Monitored, ValidStatus: Valid}}}); err != nil {
			return err
		}
		firstRowWritten = true
		cancel()
		return tx.UpsertRadarrTitles(cancelled, RadarrTitleBatch{Rows: []RadarrTitle{{ID: 1000, MainTitle: "next", Title: "next", CleanTitle: "next", Monitored: Monitored, ValidStatus: Valid}}})
	})
	afterCancel, err := repos.RadarrTitles.Page(ctx, RadarrTitleFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if err := repos.SystemConfigs.Upsert(ctx, SystemConfig{ID: 1, Key: "config", Value: stringPointer("next"), ValidStatus: Valid}); err != nil {
		t.Fatal(err)
	}
	updated := mustConfig(t, repos, 1)
	if err := repos.SonarrRules.Upsert(ctx, SonarrRule{ID: "remote-author", Token: "author", Regex: "x", Example: "x", ValidStatus: Invalid}); err != nil {
		t.Fatal(err)
	}
	remoteInputs := remoteSonarrRuleInputs(batchLimit + 1)
	remoteInputs[0].Rule.ID = "remote-author"
	if err := store.InTransaction(ctx, func(tx DatasetTransaction) error {
		return tx.UpsertRemoteSonarrRules(ctx, remoteInputs)
	}); err != nil {
		t.Fatal(err)
	}
	remoteAuthor := mustSonarrRule(t, repos, "remote-author")
	remoteNew := mustSonarrRule(t, repos, "sonarr-2")
	rollbackInputs := remoteSonarrRuleInputs(batchLimit + 1)
	rollbackInputs[batchLimit].Rule.Offset = 2147483648
	rollbackErr := store.InTransaction(ctx, func(tx DatasetTransaction) error {
		return tx.UpsertRemoteSonarrRules(ctx, rollbackInputs)
	})
	remoteAfterRollback := mustSonarrRule(t, repos, "remote-author")
	evidence := repositoryEvidence{
		OperationsExecuted:                   len(operations),
		RoundTrips:                           roundTrips,
		InjectionLiteralMatches:              injection.Total,
		WildcardMatches:                      wildcard.Total,
		ReplacementRowsPersisted:             replaced.Total,
		Row201FailureBaselineTotal:           baseline.Total,
		Row201FailureAfterRollbackTotal:      afterFailure.Total,
		CancellationBaselineTotal:            afterFailure.Total,
		CancellationAfterRollbackTotal:       afterCancel.Total,
		CancellationReturnedContextError:     errors.Is(cancelErr, context.Canceled),
		CancellationWroteFirstReplacementRow: firstRowWritten,
		TimestampCreatePreserved:             updated.CreateTime != nil && *updated.CreateTime == stamp,
		TimestampUpdateRefreshed:             updated.UpdateTime != nil && *updated.UpdateTime != stamp,
		RemoteBatchPreservesDisabledStatus:   remoteAuthor.ValidStatus == Invalid,
		RemoteBatchDefaultsNewStatus:         remoteNew.ValidStatus == Valid,
		RemoteBatchRollbackPreservesAuthor:   errors.Is(rollbackErr, ErrJavaIntegerRange) && remoteAfterRollback.ValidStatus == Invalid,
	}
	data, err := json.MarshalIndent(evidence, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
}
