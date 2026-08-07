package app

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"jproxy-go/internal/api/rule"
	"jproxy-go/internal/api/title"
	"jproxy-go/internal/runtime"
	"jproxy-go/internal/store/sqlite"
)

func TestSonarrTitleSyncTask_runsTMDBOnlyAfterSonarrSucceeds(t *testing.T) {
	// Given
	fixture := newTaskSyncFixture()
	sonarr := &taskTitleSyncer{outcomes: []taskTitleOutcome{{}}}
	tmdb := &taskTitleSyncer{outcomes: []taskTitleOutcome{{}}}
	task := sonarrTitleSyncTask(titleSyncDependencies{sonarr: sonarr, tmdb: tmdb, admission: fixture.registry}, fixture.invalidate)

	// When
	err := task(context.Background())

	// Then
	if err != nil || sonarr.calls != 1 || tmdb.calls != 1 || !reflect.DeepEqual(fixture.invalidations, [][]string{sonarrTitleInvalidation, sonarrTitleInvalidation}) {
		t.Fatalf("err=%v calls=%d/%d invalidations=%v", err, sonarr.calls, tmdb.calls, fixture.invalidations)
	}
}

func TestNewSyncTasks_exposesAllGroupCAdapters(t *testing.T) {
	// Given
	fixture := newTaskSyncFixture()
	titles := titleSyncDependencies{sonarr: &taskTitleSyncer{}, radarr: &taskTitleSyncer{}, tmdb: &taskTitleSyncer{}, admission: fixture.registry}
	rules := ruleSyncDependencies{sonarr: &taskRuleSyncer{}, radarr: &taskRuleSyncer{}}

	// When
	tasks := newSyncTasks(titles, rules, fixture.invalidate)

	// Then
	if tasks.sonarrTitle == nil || tasks.radarrTitle == nil || tasks.sonarrRule == nil || tasks.radarrRule == nil {
		t.Fatal("missing Group C task adapter")
	}
}

func TestSonarrTitleSyncTask_releasesAdmissionAndDoesNotRunTMDB_whenSonarrFails(t *testing.T) {
	// Given
	fixture := newTaskSyncFixture()
	sonarr := &taskTitleSyncer{respectContext: true}
	tmdb := &taskTitleSyncer{}
	task := sonarrTitleSyncTask(titleSyncDependencies{sonarr: sonarr, tmdb: tmdb, admission: fixture.registry}, fixture.invalidate)
	canceled, cancel := context.WithCancel(context.Background())
	cancel()

	// When
	firstErr := task(canceled)
	secondErr := task(context.Background())

	// Then
	if !errors.Is(firstErr, context.Canceled) || secondErr != nil || sonarr.calls != 2 || tmdb.calls != 1 || !reflect.DeepEqual(fixture.invalidations, [][]string{sonarrTitleInvalidation, sonarrTitleInvalidation}) {
		t.Fatalf("errors=%v/%v calls=%d/%d invalidations=%v", firstErr, secondErr, sonarr.calls, tmdb.calls, fixture.invalidations)
	}
}

func TestRadarrTitleSyncTask_retainsSuccessfulAdmission_whenInvalidationFails(t *testing.T) {
	// Given
	fixture := newTaskSyncFixture()
	syncer := &taskTitleSyncer{}
	task := radarrTitleSyncTask(titleSyncDependencies{radarr: syncer, admission: fixture.registry}, func(context.Context, ...string) error { return errors.New("refresh failed") })

	// When
	firstErr := task(context.Background())
	secondErr := task(context.Background())

	// Then
	if firstErr == nil || secondErr != nil || syncer.calls != 1 {
		t.Fatalf("errors=%v/%v calls=%d", firstErr, secondErr, syncer.calls)
	}
}

func TestSonarrTitleSyncTask_skipsTMDB_whenSonarrIsTooFrequent(t *testing.T) {
	// Given
	fixture := newTaskSyncFixture()
	attempt, err := fixture.registry.BeginTitleSync(runtime.SonarrTitleSyncInterval)
	if err != nil {
		t.Fatal(err)
	}
	fixture.registry.FinishTitleSync(attempt, true)
	sonarr := &taskTitleSyncer{}
	tmdb := &taskTitleSyncer{}
	task := sonarrTitleSyncTask(titleSyncDependencies{sonarr: sonarr, tmdb: tmdb, admission: fixture.registry}, fixture.invalidate)

	// When
	err = task(context.Background())

	// Then
	if err != nil || sonarr.calls != 0 || tmdb.calls != 0 || len(fixture.invalidations) != 0 {
		t.Fatalf("err=%v calls=%d/%d invalidations=%v", err, sonarr.calls, tmdb.calls, fixture.invalidations)
	}
}

func TestRadarrTitleSyncTask_skipsTooFrequentWithoutInvalidation(t *testing.T) {
	// Given
	fixture := newTaskSyncFixture()
	attempt, err := fixture.registry.BeginTitleSync(runtime.RadarrTitleSyncInterval)
	if err != nil {
		t.Fatal(err)
	}
	fixture.registry.FinishTitleSync(attempt, true)
	syncer := &taskTitleSyncer{}
	task := radarrTitleSyncTask(titleSyncDependencies{radarr: syncer, admission: fixture.registry}, fixture.invalidate)

	// When
	err = task(context.Background())

	// Then
	if err != nil || syncer.calls != 0 || len(fixture.invalidations) != 0 {
		t.Fatalf("err=%v calls=%d invalidations=%v", err, syncer.calls, fixture.invalidations)
	}
}

func TestRadarrTitleSyncTask_invalidatesOnlyCommittedSync(t *testing.T) {
	// Given
	fixture := newTaskSyncFixture()
	syncer := &taskTitleSyncer{outcomes: []taskTitleOutcome{{result: title.SyncTooFrequent}, {}}}
	task := radarrTitleSyncTask(titleSyncDependencies{radarr: syncer, admission: fixture.registry}, fixture.invalidate)

	// When
	firstErr := task(context.Background())
	secondErr := task(context.Background())

	// Then
	if firstErr != nil || secondErr != nil || !reflect.DeepEqual(fixture.invalidations, [][]string{radarrTitleInvalidation}) {
		t.Fatalf("errors=%v/%v invalidations=%v", firstErr, secondErr, fixture.invalidations)
	}
}

func TestRuleSyncTasks_invalidateSuccessfulLiveAdaptersAndSkipTooFrequent(t *testing.T) {
	// Given
	var invalidations [][]string
	invalidate := func(_ context.Context, names ...string) error {
		invalidations = append(invalidations, append([]string(nil), names...))
		return nil
	}
	sonarr := &taskRuleSyncer{}
	radarr := &taskRuleSyncer{outcomes: []taskRuleOutcome{{result: rule.SyncTooFrequent}}}

	// When
	sonarrErr := sonarrRuleSyncTask(ruleSyncDependencies{sonarr: sonarr}, invalidate)(context.Background())
	radarrErr := radarrRuleSyncTask(ruleSyncDependencies{radarr: radarr}, invalidate)(context.Background())

	// Then
	if sonarrErr != nil || radarrErr != nil || !reflect.DeepEqual(invalidations, [][]string{{runtime.SonarrRule}}) {
		t.Fatalf("errors=%v/%v invalidations=%v", sonarrErr, radarrErr, invalidations)
	}
}

type taskSyncFixture struct {
	registry      *runtime.Registry
	invalidations [][]string
	invalidate    func(context.Context, ...string) error
}

func newTaskSyncFixture() *taskSyncFixture {
	fixture := &taskSyncFixture{registry: runtime.NewRegistry(runtime.NewStaticProvider(sqlite.Snapshot{}), taskSyncCache{}, taskSyncCache{}, taskSyncCache{})}
	fixture.invalidate = func(_ context.Context, names ...string) error {
		fixture.invalidations = append(fixture.invalidations, append([]string(nil), names...))
		return nil
	}
	return fixture
}

type taskSyncCache struct{}

func (taskSyncCache) Clear()        {}
func (taskSyncCache) Delete(string) {}

type taskTitleOutcome struct {
	result title.SyncResult
	err    error
}

type taskTitleSyncer struct {
	outcomes       []taskTitleOutcome
	calls          int
	respectContext bool
}

func (syncer *taskTitleSyncer) Sync(ctx context.Context) (title.SyncResult, error) {
	if syncer.respectContext && ctx.Err() != nil {
		syncer.calls++
		return title.SyncSucceeded, ctx.Err()
	}
	if syncer.calls >= len(syncer.outcomes) {
		syncer.calls++
		return title.SyncSucceeded, nil
	}
	outcome := syncer.outcomes[syncer.calls]
	syncer.calls++
	return outcome.result, outcome.err
}

type taskRuleOutcome struct {
	result rule.SyncResult
	err    error
}

type taskRuleSyncer struct {
	outcomes []taskRuleOutcome
	calls    int
}

func (syncer *taskRuleSyncer) Sync(context.Context) (rule.SyncResult, error) {
	if syncer.calls >= len(syncer.outcomes) {
		syncer.calls++
		return rule.SyncSucceeded, nil
	}
	outcome := syncer.outcomes[syncer.calls]
	syncer.calls++
	return outcome.result, outcome.err
}
