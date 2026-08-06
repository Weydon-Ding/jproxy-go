package tmdb

import (
	"context"
	"errors"
	"fmt"

	"jproxy-go/internal/store/sqlite"
)

var errInvalidSyncConfig = errors.New("invalid TMDB sync configuration")

type configLoader interface {
	Load(context.Context) (SyncConfig, error)
}

type finder interface {
	Find(context.Context, Config, int64, string) (Alias, bool, error)
}

type syncSource interface {
	NeedTMDBSync(context.Context) ([]int64, error)
}

type batchRepository interface {
	UpsertBatch(context.Context, sqlite.TMDBTitleBatch) error
}

type SyncConfig struct {
	Client    Config
	Languages [2]string
}

type ConfigSource struct{ repository sqlite.SystemConfigRepository }

func NewConfigSource(repository sqlite.SystemConfigRepository) *ConfigSource {
	return &ConfigSource{repository: repository}
}

func (source *ConfigSource) Load(ctx context.Context) (SyncConfig, error) {
	rows, err := source.repository.List(ctx)
	if err != nil {
		return SyncConfig{}, fmt.Errorf("list TMDB sync config: %w", err)
	}
	values := map[string]string{}
	for _, row := range rows {
		if row.ValidStatus != sqlite.Valid || row.Value == nil {
			continue
		}
		if row.Key != "tmdbUrl" && row.Key != "tmdbApikey" && row.Key != "sonarrLanguage1" && row.Key != "sonarrLanguage2" {
			continue
		}
		if _, exists := values[row.Key]; exists {
			return SyncConfig{}, errInvalidSyncConfig
		}
		values[row.Key] = *row.Value
	}
	config := SyncConfig{Client: Config{BaseURL: values["tmdbUrl"], APIKey: values["tmdbApikey"]}, Languages: [2]string{values["sonarrLanguage1"], values["sonarrLanguage2"]}}
	if _, err := findEndpoint(config.Client, 1, config.Languages[0]); err != nil || config.Languages[1] == "" || containsControl(config.Languages[1]) {
		return SyncConfig{}, errInvalidSyncConfig
	}
	return config, nil
}

type Dependencies struct {
	Config     configLoader
	Client     finder
	Source     syncSource
	Repository batchRepository
}

type Service struct{ dependencies Dependencies }

func NewService(dependencies Dependencies) Service { return Service{dependencies: dependencies} }

func (service Service) Sync(ctx context.Context) error {
	config, err := service.dependencies.Config.Load(ctx)
	if err != nil {
		return fmt.Errorf("load TMDB sync config: %w", err)
	}
	ids, err := service.dependencies.Source.NeedTMDBSync(ctx)
	if err != nil {
		return fmt.Errorf("find TMDB sync IDs: %w", err)
	}
	rows := make([]sqlite.TMDBTitle, 0, len(ids)*len(config.Languages))
	for _, id := range ids {
		for _, language := range config.Languages {
			alias, found, err := service.dependencies.Client.Find(ctx, config.Client, id, language)
			if err != nil {
				return fmt.Errorf("find TMDB alias: %w", err)
			}
			if found {
				rows = append(rows, sqlite.TMDBTitle{TVDBID: alias.TVDBID, TMDBID: alias.TMDBID, Language: alias.Language, Title: alias.Title, ValidStatus: sqlite.Valid})
			}
		}
	}
	if err := service.dependencies.Repository.UpsertBatch(ctx, sqlite.TMDBTitleBatch{Rows: rows}); err != nil {
		return fmt.Errorf("upsert TMDB aliases: %w", err)
	}
	return nil
}
