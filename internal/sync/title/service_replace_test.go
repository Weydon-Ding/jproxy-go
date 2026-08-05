package titlesync

import (
	"context"
	"testing"

	"jproxy-go/internal/store/sqlite"
)

type fixedSonarrConfig struct{ value providerConfig }

func (c fixedSonarrConfig) loadSonarr(context.Context) (providerConfig, error) { return c.value, nil }

type fixedSonarrFetcher struct{ values []SonarrSeries }

func (f fixedSonarrFetcher) Fetch(context.Context, providerConfig) ([]SonarrSeries, error) {
	return f.values, nil
}

type countingSonarrReplacer struct {
	calls int
	batch sqlite.SonarrTitleBatch
}

func (r *countingSonarrReplacer) Replace(_ context.Context, batch sqlite.SonarrTitleBatch) error {
	r.calls++
	r.batch = batch
	return nil
}

func TestSonarrService_callsReplaceOnce_whenSyncSucceeds(t *testing.T) {
	// Given
	repository := &countingSonarrReplacer{}
	service := NewSonarrService(SonarrServiceDependencies{
		Config: fixedSonarrConfig{},
		Client: fixedSonarrFetcher{values: []SonarrSeries{
			{ID: 1, TVDBID: 2, Title: "Main", TitleSlug: "main", Monitored: true},
		}},
		Repository: repository,
	})

	// When
	err := service.Sync(context.Background())

	// Then
	if err != nil {
		t.Fatal(err)
	}
	if repository.calls != 1 {
		t.Fatalf("replace calls = %d", repository.calls)
	}
	if len(repository.batch.Rows) != 2 {
		t.Fatalf("replace rows = %d", len(repository.batch.Rows))
	}
}
