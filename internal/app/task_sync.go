package app

import (
	"context"
	"errors"
	"fmt"

	"jproxy-go/internal/api/rule"
	"jproxy-go/internal/api/title"
	"jproxy-go/internal/runtime"
)

type scheduledTask func(context.Context) error

type syncTasks struct {
	sonarrTitle scheduledTask
	radarrTitle scheduledTask
	sonarrRule  scheduledTask
	radarrRule  scheduledTask
}

var (
	sonarrTitleInvalidation = []string{runtime.SonarrSearchTitle, runtime.IndexerSearchOffset, runtime.SonarrResultTitle}
	radarrTitleInvalidation = []string{runtime.RadarrSearchTitle, runtime.IndexerSearchOffset, runtime.RadarrResultTitle}
)

func newSyncTasks(titles titleSyncDependencies, rules ruleSyncDependencies, invalidate func(context.Context, ...string) error) syncTasks {
	return syncTasks{
		sonarrTitle: sonarrTitleSyncTask(titles, invalidate),
		radarrTitle: radarrTitleSyncTask(titles, invalidate),
		sonarrRule:  sonarrRuleSyncTask(rules, invalidate),
		radarrRule:  radarrRuleSyncTask(rules, invalidate),
	}
}

func sonarrTitleSyncTask(dependencies titleSyncDependencies, invalidate func(context.Context, ...string) error) scheduledTask {
	return func(ctx context.Context) error {
		succeeded, err := runTitleSync(ctx, dependencies.sonarr, dependencies.admission, runtime.SonarrTitleSyncInterval, sonarrTitleInvalidation, invalidate)
		if err != nil {
			return fmt.Errorf("sync Sonarr titles: %w", err)
		}
		if !succeeded {
			return nil
		}
		if _, err := runTitleSync(ctx, dependencies.tmdb, dependencies.admission, runtime.TMDBTitleSyncInterval, sonarrTitleInvalidation, invalidate); err != nil {
			return fmt.Errorf("sync TMDB titles: %w", err)
		}
		return nil
	}
}

func radarrTitleSyncTask(dependencies titleSyncDependencies, invalidate func(context.Context, ...string) error) scheduledTask {
	return func(ctx context.Context) error {
		if _, err := runTitleSync(ctx, dependencies.radarr, dependencies.admission, runtime.RadarrTitleSyncInterval, radarrTitleInvalidation, invalidate); err != nil {
			return fmt.Errorf("sync Radarr titles: %w", err)
		}
		return nil
	}
}

func sonarrRuleSyncTask(dependencies ruleSyncDependencies, invalidate func(context.Context, ...string) error) scheduledTask {
	return ruleSyncTask(dependencies.sonarr, runtime.SonarrRule, invalidate)
}

func radarrRuleSyncTask(dependencies ruleSyncDependencies, invalidate func(context.Context, ...string) error) scheduledTask {
	return ruleSyncTask(dependencies.radarr, runtime.RadarrRule, invalidate)
}

func runTitleSync(ctx context.Context, syncer title.Syncer, admission title.SyncAdmission, marker string, invalidation []string, invalidate func(context.Context, ...string) error) (bool, error) {
	if syncer == nil {
		return false, title.ErrSyncUnavailable
	}
	var attempt runtime.TitleSyncAttempt
	if admission != nil {
		var err error
		attempt, err = admission.BeginTitleSync(marker)
		if errors.Is(err, runtime.ErrTitleSyncTooFrequent) {
			return false, nil
		}
		if err != nil {
			return false, fmt.Errorf("admit title sync: %w", err)
		}
	}
	result, err := syncer.Sync(ctx)
	if err != nil {
		finishTitleSync(admission, attempt, false)
		return false, err
	}
	if result == title.SyncTooFrequent {
		finishTitleSync(admission, attempt, false)
		return false, nil
	}
	finishTitleSync(admission, attempt, true)
	if invalidate == nil {
		return true, nil
	}
	if err := invalidate(ctx, invalidation...); err != nil {
		return false, fmt.Errorf("refresh synced titles: %w", err)
	}
	return true, nil
}

func finishTitleSync(admission title.SyncAdmission, attempt runtime.TitleSyncAttempt, succeeded bool) {
	if admission != nil {
		admission.FinishTitleSync(attempt, succeeded)
	}
}

func ruleSyncTask(syncer rule.Syncer, scope string, invalidate func(context.Context, ...string) error) scheduledTask {
	return func(ctx context.Context) error {
		if syncer == nil {
			return rule.ErrSyncUnavailable
		}
		result, err := syncer.Sync(ctx)
		if err != nil {
			return err
		}
		if result == rule.SyncTooFrequent || invalidate == nil {
			return nil
		}
		if err := invalidate(ctx, scope); err != nil {
			return fmt.Errorf("refresh synced rules: %w", err)
		}
		return nil
	}
}
