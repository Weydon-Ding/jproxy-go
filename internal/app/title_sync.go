package app

import (
	"context"
	"net/http"

	"jproxy-go/internal/api/title"
	"jproxy-go/internal/config"
	"jproxy-go/internal/runtime"
	titlesync "jproxy-go/internal/sync/title"
	tmdbsync "jproxy-go/internal/sync/tmdb"
)

type titleService interface{ Sync(context.Context) error }

type titleServiceAdapter struct{ service titleService }

func (adapter titleServiceAdapter) Sync(ctx context.Context) (title.SyncResult, error) {
	if err := adapter.service.Sync(ctx); err != nil {
		return title.SyncSucceeded, err
	}
	return title.SyncSucceeded, nil
}

func liveTitleSyncDependencies(cfg config.Config, store managementStore, registry *runtime.Registry) titleSyncDependencies {
	client := &http.Client{Timeout: cfg.HTTPTimeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	configSource := titlesync.NewConfigSource(store.Repositories().SystemConfigs)
	sonarr := titlesync.NewSonarrService(titlesync.SonarrServiceDependencies{
		Config: configSource, Client: titlesync.NewSonarrClient(titlesync.NewRequestClient(client, cfg.HTTPTimeout)), Repository: store.Repositories().SonarrTitles,
	})
	radarr := titlesync.NewRadarrService(titlesync.RadarrServiceDependencies{
		Config: configSource, Client: titlesync.NewRadarrClient(titlesync.NewRequestClient(client, cfg.HTTPTimeout)), Repository: store.Repositories().RadarrTitles,
	})
	tmdb := tmdbsync.NewService(tmdbsync.Dependencies{
		Config: tmdbsync.NewConfigSource(store.Repositories().SystemConfigs), Client: tmdbsync.NewClient(client, cfg.HTTPTimeout),
		Source: store.Repositories().SonarrTitles, Repository: store.Repositories().TMDBTitles,
	})
	return titleSyncDependencies{sonarr: titleServiceAdapter{service: sonarr}, radarr: titleServiceAdapter{service: radarr}, tmdb: titleServiceAdapter{service: tmdb}, admission: registry}
}
