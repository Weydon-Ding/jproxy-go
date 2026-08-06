// Package runtime owns immutable proxy data and exact cache invalidation.
package runtime

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"jproxy-go/internal/format"
	"jproxy-go/internal/search"
	"jproxy-go/internal/store/sqlite"
)

var ErrSnapshotRefresh = errors.New("runtime formatter snapshot refresh failed")

type snapshotRefreshError struct{}

func (e snapshotRefreshError) Error() string { return ErrSnapshotRefresh.Error() }

func (e snapshotRefreshError) Is(target error) bool {
	return target == ErrSnapshotRefresh
}

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
	RadarrCandidates     []search.Candidate
	SonarrCandidates     []search.Candidate
	JackettURL           string
	ProwlarrURL          string
	QBittorrentURL       string
	QBittorrentUsername  string
	QBittorrentPassword  string
	QBittorrentRevision  uint64
	TransmissionURL      string
	TransmissionUsername string
	TransmissionPassword string
	TransmissionRevision uint64
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
		RadarrCandidates:     cloneCandidates(value.RadarrCandidates),
		SonarrCandidates:     cloneCandidates(value.SonarrCandidates),
		JackettURL:           value.JackettURL,
		ProwlarrURL:          value.ProwlarrURL,
		QBittorrentURL:       value.QBittorrentURL,
		QBittorrentUsername:  value.QBittorrentUsername,
		QBittorrentPassword:  value.QBittorrentPassword,
		QBittorrentRevision:  value.QBittorrentRevision,
		TransmissionURL:      value.TransmissionURL,
		TransmissionUsername: value.TransmissionUsername,
		TransmissionPassword: value.TransmissionPassword,
		TransmissionRevision: value.TransmissionRevision,
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
		return snapshotRefreshError{}
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

func (p *provider) publishPrepared(source sqlite.Snapshot) {
	p.mu.Lock()
	defer p.mu.Unlock()
	current := p.Snapshot()
	next := snapshotFromSQLite(source, current, ScopeSystemConfig)
	p.value.Store(&next)
}

func snapshotFromSQLite(source sqlite.Snapshot, previous Snapshot, scopes Scope) Snapshot {
	next := Snapshot{Radarr: cloneRadarr(previous.Radarr), Sonarr: cloneSonarr(previous.Sonarr), RadarrCandidates: cloneCandidates(previous.RadarrCandidates), SonarrCandidates: cloneCandidates(previous.SonarrCandidates), JackettURL: previous.JackettURL, ProwlarrURL: previous.ProwlarrURL, QBittorrentURL: previous.QBittorrentURL, QBittorrentUsername: previous.QBittorrentUsername, QBittorrentPassword: previous.QBittorrentPassword, QBittorrentRevision: previous.QBittorrentRevision, TransmissionURL: previous.TransmissionURL, TransmissionUsername: previous.TransmissionUsername, TransmissionPassword: previous.TransmissionPassword, TransmissionRevision: previous.TransmissionRevision, RadarrRevision: previous.RadarrRevision, SonarrRevision: previous.SonarrRevision, RadarrSearchRevision: previous.RadarrSearchRevision, SonarrSearchRevision: previous.SonarrSearchRevision}
	if scopes == allScopes {
		next.Radarr = cloneRadarr(source.Radarr)
		next.Sonarr = cloneSonarr(source.Sonarr)
		next.RadarrCandidates = cloneCandidates(source.RadarrCandidates)
		next.SonarrCandidates = cloneCandidates(source.SonarrCandidates)
	}
	if scopes&ScopeSystemConfig != 0 {
		next.Radarr.Format = source.Radarr.Format
		next.Radarr.CleanTitleRegex = source.Radarr.CleanTitleRegex
		next.Sonarr.Format = source.Sonarr.Format
		next.Sonarr.CleanTitleRegex = source.Sonarr.CleanTitleRegex
		next.JackettURL = source.JackettURL
		next.ProwlarrURL = source.ProwlarrURL
		next.QBittorrentURL = source.QBittorrentURL
		next.QBittorrentUsername = source.QBittorrentUsername
		next.QBittorrentPassword = source.QBittorrentPassword
		next.QBittorrentRevision++
		next.TransmissionURL = source.TransmissionURL
		next.TransmissionUsername = source.TransmissionUsername
		next.TransmissionPassword = source.TransmissionPassword
		next.TransmissionRevision++
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
		next.RadarrCandidates = cloneCandidates(source.RadarrCandidates)
	}
	if scopes&(ScopeSonarrRules|ScopeSonarrTitles) != 0 {
		next.SonarrRevision++
	}
	if scopes&ScopeSonarrRules != 0 {
		next.Sonarr.Rules = cloneSonarr(source.Sonarr).Rules
	}
	if scopes&ScopeSonarrTitles != 0 {
		next.Sonarr.Titles = cloneSonarr(source.Sonarr).Titles
		next.SonarrCandidates = cloneCandidates(source.SonarrCandidates)
	}
	if scopes&ScopeRadarrTitles != 0 {
		next.RadarrSearchRevision++
	}
	if scopes&ScopeSonarrTitles != 0 {
		next.SonarrSearchRevision++
	}
	return next
}

func cloneCandidates(value []search.Candidate) []search.Candidate {
	return append([]search.Candidate(nil), value...)
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
