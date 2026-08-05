// Package runtime owns immutable proxy data and exact cache invalidation.
package runtime

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"jproxy-go/internal/format"
	"jproxy-go/internal/store/sqlite"
)

var ErrSnapshotRefresh = errors.New("runtime formatter snapshot refresh failed")

type snapshotRefreshError struct{ cause error }

func (e snapshotRefreshError) Error() string { return ErrSnapshotRefresh.Error() }

func (e snapshotRefreshError) Is(target error) bool {
	return target == ErrSnapshotRefresh || errors.Is(e.cause, target)
}

func (e snapshotRefreshError) Unwrap() error { return e.cause }

type Scope uint8

const (
	ScopeSystemConfig Scope = 1 << iota
	ScopeSonarrRules
	ScopeSonarrTitles
	ScopeRadarrRules
	ScopeRadarrTitles
)

const allScopes = ScopeSystemConfig | ScopeSonarrRules | ScopeSonarrTitles | ScopeRadarrRules | ScopeRadarrTitles

type Snapshot struct {
	Radarr               format.Config
	Sonarr               format.SonarrConfig
	RadarrRevision       uint64
	SonarrRevision       uint64
	RadarrSearchRevision uint64
	SonarrSearchRevision uint64
}

type Loader interface {
	LoadFormatterSnapshot(context.Context) (sqlite.Snapshot, error)
}

type Provider interface {
	Snapshot() Snapshot
	Refresh(context.Context, Scope) error
}

type provider struct {
	loader        Loader
	mu            sync.Mutex
	value         atomic.Pointer[Snapshot]
	beforePublish func()
}

func NewProvider(initial sqlite.Snapshot, loader Loader) Provider {
	p := &provider{loader: loader}
	value := snapshotFromSQLite(initial, Snapshot{}, allScopes)
	p.value.Store(&value)
	return p
}

func NewStaticProvider(initial sqlite.Snapshot) Provider { return NewProvider(initial, nil) }

func (p *provider) Snapshot() Snapshot {
	value := p.value.Load()
	return Snapshot{
		Radarr:               cloneRadarr(value.Radarr),
		Sonarr:               cloneSonarr(value.Sonarr),
		RadarrRevision:       value.RadarrRevision,
		SonarrRevision:       value.SonarrRevision,
		RadarrSearchRevision: value.RadarrSearchRevision,
		SonarrSearchRevision: value.SonarrSearchRevision,
	}
}

func (p *provider) Refresh(ctx context.Context, scopes Scope) error {
	if scopes == 0 || p.loader == nil {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("refresh runtime formatter snapshot: %w", err)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("refresh runtime formatter snapshot: %w", err)
	}
	loaded, err := p.loader.LoadFormatterSnapshot(ctx)
	if err != nil {
		return snapshotRefreshError{cause: err}
	}
	current := p.Snapshot()
	next := snapshotFromSQLite(loaded, current, scopes)
	if p.beforePublish != nil {
		p.beforePublish()
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("publish runtime formatter snapshot: %w", err)
	}
	p.value.Store(&next)
	return nil
}

func snapshotFromSQLite(source sqlite.Snapshot, previous Snapshot, scopes Scope) Snapshot {
	next := Snapshot{Radarr: cloneRadarr(previous.Radarr), Sonarr: cloneSonarr(previous.Sonarr), RadarrRevision: previous.RadarrRevision, SonarrRevision: previous.SonarrRevision, RadarrSearchRevision: previous.RadarrSearchRevision, SonarrSearchRevision: previous.SonarrSearchRevision}
	if scopes == allScopes {
		next.Radarr = cloneRadarr(source.Radarr)
		next.Sonarr = cloneSonarr(source.Sonarr)
	}
	if scopes&ScopeSystemConfig != 0 {
		next.Radarr.Format = source.Radarr.Format
		next.Radarr.CleanTitleRegex = source.Radarr.CleanTitleRegex
		next.Sonarr.Format = source.Sonarr.Format
		next.Sonarr.CleanTitleRegex = source.Sonarr.CleanTitleRegex
		next.RadarrRevision++
		next.SonarrRevision++
	}
	if scopes&(ScopeRadarrRules|ScopeRadarrTitles) != 0 {
		next.RadarrRevision++
	}
	if scopes&ScopeRadarrRules != 0 {
		next.Radarr.Rules = cloneRadarr(source.Radarr).Rules
	}
	if scopes&ScopeRadarrTitles != 0 {
		next.Radarr.Titles = cloneRadarr(source.Radarr).Titles
	}
	if scopes&(ScopeSonarrRules|ScopeSonarrTitles) != 0 {
		next.SonarrRevision++
	}
	if scopes&ScopeSonarrRules != 0 {
		next.Sonarr.Rules = cloneSonarr(source.Sonarr).Rules
	}
	if scopes&ScopeSonarrTitles != 0 {
		next.Sonarr.Titles = cloneSonarr(source.Sonarr).Titles
	}
	if scopes&ScopeRadarrTitles != 0 {
		next.RadarrSearchRevision++
	}
	if scopes&ScopeSonarrTitles != 0 {
		next.SonarrSearchRevision++
	}
	return next
}

func cloneRadarr(value format.Config) format.Config {
	value.Rules = append([]format.Rule(nil), value.Rules...)
	for index := range value.Rules {
		if value.Rules[index].ValidStatus != nil {
			status := *value.Rules[index].ValidStatus
			value.Rules[index].ValidStatus = &status
		}
	}
	value.Titles = append([]format.Title(nil), value.Titles...)
	return value
}

func cloneSonarr(value format.SonarrConfig) format.SonarrConfig {
	value.Rules = append([]format.Rule(nil), value.Rules...)
	for index := range value.Rules {
		if value.Rules[index].ValidStatus != nil {
			status := *value.Rules[index].ValidStatus
			value.Rules[index].ValidStatus = &status
		}
	}
	value.Titles = append([]format.SonarrTitle(nil), value.Titles...)
	for index := range value.Titles {
		if value.Titles[index].SeasonNumber != nil {
			season := *value.Titles[index].SeasonNumber
			value.Titles[index].SeasonNumber = &season
		}
	}
	return value
}
