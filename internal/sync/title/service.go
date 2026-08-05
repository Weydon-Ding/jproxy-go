package titlesync

import (
	"context"
	"fmt"

	"jproxy-go/internal/store/sqlite"
)

type sonarrConfigLoader interface {
	loadSonarr(context.Context) (providerConfig, error)
}

type radarrConfigLoader interface {
	loadRadarr(context.Context) (providerConfig, error)
}

type sonarrFetcher interface {
	Fetch(context.Context, providerConfig) ([]SonarrSeries, error)
}

type radarrFetcher interface {
	Fetch(context.Context, providerConfig) ([]RadarrMovie, error)
}

type sonarrReplacer interface {
	Replace(context.Context, sqlite.SonarrTitleBatch) error
}

type radarrReplacer interface {
	Replace(context.Context, sqlite.RadarrTitleBatch) error
}

type SonarrServiceDependencies struct {
	Config     sonarrConfigLoader
	Client     sonarrFetcher
	Repository sonarrReplacer
}

type RadarrServiceDependencies struct {
	Config     radarrConfigLoader
	Client     radarrFetcher
	Repository radarrReplacer
}

type SonarrService struct{ dependencies SonarrServiceDependencies }
type RadarrService struct{ dependencies RadarrServiceDependencies }

func NewSonarrService(dependencies SonarrServiceDependencies) SonarrService {
	return SonarrService{dependencies: dependencies}
}

func NewRadarrService(dependencies RadarrServiceDependencies) RadarrService {
	return RadarrService{dependencies: dependencies}
}

func (s SonarrService) Sync(ctx context.Context) error {
	config, err := s.dependencies.Config.loadSonarr(ctx)
	if err != nil {
		return fmt.Errorf("sonarr sync config: %w", err)
	}
	values, err := s.dependencies.Client.Fetch(ctx, config)
	if err != nil {
		return fmt.Errorf("sonarr sync fetch: %w", err)
	}
	rows, err := MapSonarr(values, config.cleanRE)
	if err != nil {
		return fmt.Errorf("sonarr sync map: %w", err)
	}
	if err := s.dependencies.Repository.Replace(ctx, sqlite.SonarrTitleBatch{Rows: rows}); err != nil {
		return fmt.Errorf("sonarr sync replace: %w", err)
	}
	return nil
}

func (s RadarrService) Sync(ctx context.Context) error {
	config, err := s.dependencies.Config.loadRadarr(ctx)
	if err != nil {
		return fmt.Errorf("radarr sync config: %w", err)
	}
	values, err := s.dependencies.Client.Fetch(ctx, config)
	if err != nil {
		return fmt.Errorf("radarr sync fetch: %w", err)
	}
	rows, err := MapRadarr(values, config.cleanRE)
	if err != nil {
		return fmt.Errorf("radarr sync map: %w", err)
	}
	if err := s.dependencies.Repository.Replace(ctx, sqlite.RadarrTitleBatch{Rows: rows}); err != nil {
		return fmt.Errorf("radarr sync replace: %w", err)
	}
	return nil
}
