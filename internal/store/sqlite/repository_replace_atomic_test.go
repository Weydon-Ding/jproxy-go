package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"strconv"
	"testing"
)

func TestRepositories_replaceRestoresPreviousDataset_whenRow201IsInvalid(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name    string
		seed    func(context.Context, Repositories) error
		replace func(context.Context, Repositories) error
		count   func(context.Context, Repositories) (int64, error)
	}{
		{
			name: "sonarr rules", seed: func(ctx context.Context, repos Repositories) error {
				return repos.SonarrRules.Upsert(ctx, SonarrRule{ID: "old", Token: "old", Regex: "x", Example: "x", ValidStatus: Valid})
			}, replace: func(ctx context.Context, repos Repositories) error {
				rows := validSonarrRules(batchLimit + 1)
				rows[batchLimit].ValidStatus = ValidStatus(2)
				return repos.SonarrRules.Replace(ctx, SonarrRuleBatch{Rows: rows})
			}, count: func(ctx context.Context, repos Repositories) (int64, error) {
				page, err := repos.SonarrRules.Page(ctx, RuleFilter{})
				return page.Total, err
			},
		},
		{
			name: "radarr rules", seed: func(ctx context.Context, repos Repositories) error {
				return repos.RadarrRules.Upsert(ctx, RadarrRule{ID: "old", Token: "old", Regex: "x", Example: "x", ValidStatus: Valid})
			}, replace: func(ctx context.Context, repos Repositories) error {
				rows := validRadarrRules(batchLimit + 1)
				rows[batchLimit].ValidStatus = ValidStatus(2)
				return repos.RadarrRules.Replace(ctx, RadarrRuleBatch{Rows: rows})
			}, count: func(ctx context.Context, repos Repositories) (int64, error) {
				page, err := repos.RadarrRules.Page(ctx, RuleFilter{})
				return page.Total, err
			},
		},
		{
			name: "sonarr titles", seed: func(ctx context.Context, repos Repositories) error {
				return repos.SonarrTitles.Upsert(ctx, SonarrTitle{ID: 1, MainTitle: "old", Title: "old", CleanTitle: stringPointer("old"), Monitored: Monitored, ValidStatus: Valid})
			}, replace: func(ctx context.Context, repos Repositories) error {
				rows := validSonarrTitles(batchLimit + 1)
				rows[batchLimit].ValidStatus = ValidStatus(2)
				return repos.SonarrTitles.Replace(ctx, SonarrTitleBatch{Rows: rows})
			}, count: func(ctx context.Context, repos Repositories) (int64, error) {
				page, err := repos.SonarrTitles.Page(ctx, SonarrTitleFilter{})
				return page.Total, err
			},
		},
		{
			name: "radarr titles", seed: func(ctx context.Context, repos Repositories) error {
				return repos.RadarrTitles.Upsert(ctx, RadarrTitle{ID: 1, MainTitle: "old", Title: "old", CleanTitle: "old", Monitored: Monitored, ValidStatus: Valid})
			}, replace: func(ctx context.Context, repos Repositories) error {
				rows := validRadarrTitles(batchLimit + 1)
				rows[batchLimit].ValidStatus = ValidStatus(2)
				return repos.RadarrTitles.Replace(ctx, RadarrTitleBatch{Rows: rows})
			}, count: func(ctx context.Context, repos Repositories) (int64, error) {
				page, err := repos.RadarrTitles.Page(ctx, RadarrTitleFilter{})
				return page.Total, err
			},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			store, err := Open(ctx, filepath.Join(t.TempDir(), "replace.db"))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = store.Close() })
			repos := store.Repositories()
			if err := testCase.seed(ctx, repos); err != nil {
				t.Fatal(err)
			}
			if err := testCase.replace(ctx, repos); !errors.Is(err, ErrInvalidValidStatus) {
				t.Fatalf("Replace() error = %v, want ErrInvalidValidStatus", err)
			}
			total, err := testCase.count(ctx, repos)
			if err != nil || total != 1 {
				t.Fatalf("previous dataset total = %d, error = %v", total, err)
			}
		})
	}
}

func validSonarrRules(length int) []SonarrRule {
	rows := make([]SonarrRule, length)
	for index := range rows {
		rows[index] = SonarrRule{ID: RuleID("s" + strconv.Itoa(index+1)), Token: "title", Regex: "x", Example: "x", ValidStatus: Valid}
	}
	return rows
}

func validRadarrRules(length int) []RadarrRule {
	rows := make([]RadarrRule, length)
	for index := range rows {
		rows[index] = RadarrRule{ID: RuleID("r" + strconv.Itoa(index+1)), Token: "title", Regex: "x", Example: "x", ValidStatus: Valid}
	}
	return rows
}

func validSonarrTitles(length int) []SonarrTitle {
	rows := make([]SonarrTitle, length)
	for index := range rows {
		rows[index] = SonarrTitle{ID: SonarrTitleID(index + 10), MainTitle: "title", Title: "title", CleanTitle: stringPointer("title"), Monitored: Monitored, ValidStatus: Valid}
	}
	return rows
}

func validRadarrTitles(length int) []RadarrTitle {
	rows := make([]RadarrTitle, length)
	for index := range rows {
		rows[index] = RadarrTitle{ID: RadarrTitleID(index + 10), MainTitle: "title", Title: "title", CleanTitle: "title", Monitored: Monitored, ValidStatus: Valid}
	}
	return rows
}
