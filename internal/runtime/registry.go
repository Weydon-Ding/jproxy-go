package runtime

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

const (
	SystemConfig            = "system_config"
	SonarrSearchTitle       = "sonarr_search_title"
	IndexerSearchOffset     = "indexer_search_offset"
	SonarrRule              = "sonarr_rule"
	SonarrResultTitle       = "sonarr_result_title"
	RadarrSearchTitle       = "radarr_search_title"
	RadarrRule              = "radarr_rule"
	RadarrResultTitle       = "radarr_result_title"
	SonarrTitleSyncInterval = "sonarr_title_sync_interval"
	TMDBTitleSyncInterval   = "tmdb_title_sync_interval"
	RadarrTitleSyncInterval = "radarr_title_sync_interval"
	IndexerResult           = "indexer_result"
)

var ErrUnknownCacheName = errors.New("unknown cache name")

type Cache interface {
	Clear()
	Delete(string)
}

type Registry struct {
	provider Provider
	results  Cache
	offsets  Cache
	markers  Cache
}

func NewRegistry(provider Provider, results, offsets, markers Cache) *Registry {
	return &Registry{provider: provider, results: results, offsets: offsets, markers: markers}
}

func (r *Registry) Invalidate(ctx context.Context, names ...string) error {
	plan, err := invalidationPlan(names)
	if err != nil {
		return err
	}
	if err := r.provider.Refresh(ctx, plan.scopes); err != nil {
		return fmt.Errorf("refresh named runtime cache: %w", err)
	}
	r.apply(plan)
	return nil
}

func (r *Registry) InvalidateAll(ctx context.Context) error {
	if err := r.provider.Refresh(ctx, allScopes); err != nil {
		return fmt.Errorf("refresh all runtime caches: %w", err)
	}
	r.results.Clear()
	r.offsets.Clear()
	r.markers.Clear()
	return nil
}

// DeleteMarker removes one title-sync retry marker without refreshing runtime
// snapshots or changing result and offset caches.
func (r *Registry) DeleteMarker(name string) error {
	switch name {
	case SonarrTitleSyncInterval, TMDBTitleSyncInterval, RadarrTitleSyncInterval:
		r.markers.Delete(name)
		return nil
	default:
		return fmt.Errorf("%q: %w", name, ErrUnknownCacheName)
	}
}

// DeleteSystemConfigSyncMarkers is the post-commit half of a prepared system
// configuration publication. The snapshot has already been atomically swapped.
func (r *Registry) DeleteSystemConfigSyncMarkers() {
	r.markers.Delete(SonarrTitleSyncInterval)
	r.markers.Delete(TMDBTitleSyncInterval)
	r.markers.Delete(RadarrTitleSyncInterval)
}

type plan struct {
	scopes       Scope
	clearResults bool
	clearOffsets bool
	markers      []string
}

func invalidationPlan(names []string) (plan, error) {
	seen := map[string]bool{}
	var result plan
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			return plan{}, fmt.Errorf("blank cache name: %w", ErrUnknownCacheName)
		}
		if seen[name] {
			continue
		}
		seen[name] = true
		switch name {
		case SystemConfig:
			result.scopes |= ScopeSystemConfig
		case SonarrSearchTitle, SonarrResultTitle:
			result.scopes |= ScopeSonarrTitles
		case RadarrSearchTitle, RadarrResultTitle:
			result.scopes |= ScopeRadarrTitles
		case SonarrRule:
			result.scopes |= ScopeSonarrRules
		case RadarrRule:
			result.scopes |= ScopeRadarrRules
		case IndexerResult:
			result.clearResults = true
		case IndexerSearchOffset:
			result.clearOffsets = true
		case SonarrTitleSyncInterval, TMDBTitleSyncInterval, RadarrTitleSyncInterval:
			result.markers = append(result.markers, name)
		default:
			return plan{}, fmt.Errorf("%q: %w", name, ErrUnknownCacheName)
		}
	}
	return result, nil
}

func (r *Registry) apply(plan plan) {
	if plan.clearResults {
		r.results.Clear()
	}
	if plan.clearOffsets {
		r.offsets.Clear()
	}
	for _, marker := range plan.markers {
		r.markers.Delete(marker)
	}
}
