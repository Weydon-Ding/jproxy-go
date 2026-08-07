package rename

import (
	"context"
	"errors"
	"strings"
	"time"
)

type ServiceOptions struct {
	History      History
	Config       Config
	QBittorrent  QBittorrent
	Transmission TorrentDownloader
	Files        FileOptions
	Now          func() time.Time
}

type Service struct{ options ServiceOptions }

func NewService(options ServiceOptions) (*Service, error) {
	if options.History == nil || options.Now == nil {
		return nil, errors.New("rename history and clock are required")
	}
	return &Service{options: options}, nil
}

func (s *Service) RunSonarr(ctx context.Context, endpoint Endpoint, fallback time.Duration) error {
	return s.run(ctx, endpoint, fallback, true)
}

func (s *Service) RunRadarr(ctx context.Context, endpoint Endpoint, fallback time.Duration) error {
	return s.run(ctx, endpoint, fallback, false)
}

func (s *Service) RunConfiguredSonarr(ctx context.Context, fallback time.Duration) error {
	endpoint, err := s.endpoint(ctx, sonarrURLKey, sonarrAPIKeyKey)
	if err != nil {
		return err
	}
	return s.RunSonarr(ctx, endpoint, fallback)
}

func (s *Service) RunConfiguredRadarr(ctx context.Context, fallback time.Duration) error {
	endpoint, err := s.endpoint(ctx, radarrURLKey, radarrAPIKeyKey)
	if err != nil {
		return err
	}
	return s.RunRadarr(ctx, endpoint, fallback)
}

func (s *Service) run(ctx context.Context, endpoint Endpoint, fallback time.Duration, deduplicate bool) error {
	if fallback < 0 {
		return errors.New("rename fallback must not be negative")
	}
	events, err := s.options.History.Fetch(ctx, endpoint.URL, endpoint.APIKey, s.options.Now().Add(-fallback))
	if err != nil {
		return err
	}
	seen := make(map[string]struct{}, len(events))
	var result error
	for _, event := range events {
		if err := ctx.Err(); err != nil {
			return errors.Join(result, err)
		}
		if strings.TrimSpace(event.Hash) == "" || !validName(event.SourceTitle) || hasControl(event.Hash) {
			continue
		}
		if deduplicate {
			if _, exists := seen[event.Hash]; exists {
				continue
			}
			seen[event.Hash] = struct{}{}
		}
		result = errors.Join(result, s.renameEvent(ctx, event, deduplicate))
	}
	return result
}

func (s *Service) renameEvent(ctx context.Context, event Event, sonarr bool) error {
	switch event.Downloader {
	case DownloaderTransmission:
		if s.options.Transmission != nil {
			return s.options.Transmission.Rename(ctx, event.Hash, event.SourceTitle)
		}
	case DownloaderQBittorrent:
		if s.options.QBittorrent == nil {
			return nil
		}
		var plan []FileRename
		if s.options.Files.Enabled {
			files, err := s.options.QBittorrent.Files(ctx, event.Hash)
			if err != nil {
				return err
			}
			plan, err = s.filePlan(event.SourceTitle, files, sonarr)
			if err != nil {
				return err
			}
		}
		if err := s.options.QBittorrent.Rename(ctx, event.Hash, event.SourceTitle); err != nil {
			return err
		}
		for _, rename := range plan {
			if err := ctx.Err(); err != nil {
				return err
			}
			if err := s.options.QBittorrent.RenameFile(ctx, event.Hash, rename.OldPath, rename.NewPath); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Service) filePlan(title string, files []string, sonarr bool) ([]FileRename, error) {
	if !sonarr || strings.TrimSpace(s.options.Files.SonarrFormat) == "" {
		return PlanRadarrFiles(title, files)
	}
	return PlanSonarrFiles(title, files, s.options.Files.SonarrFormat, s.options.Files.SonarrRules)
}

func (s *Service) endpoint(ctx context.Context, urlKey, apiKey string) (Endpoint, error) {
	if s.options.Config == nil {
		return Endpoint{}, errors.New("rename config is required")
	}
	url, err := s.options.Config.ValueByKey(ctx, urlKey)
	if err != nil {
		return Endpoint{}, err
	}
	key, err := s.options.Config.ValueByKey(ctx, apiKey)
	if err != nil {
		return Endpoint{}, err
	}
	return Endpoint{URL: url, APIKey: key}, nil
}
