package app

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"jproxy-go/internal/config"
	"jproxy-go/internal/tasks"
)

func TestComposeTasks_registersSevenJavaCompatibleJobsWithoutStarting(t *testing.T) {
	// Given
	registry := &taskRegistry{}
	calls := &taskCalls{}
	dependencies := newTaskDependencies(calls)
	start := time.Date(2026, time.August, 7, 10, 59, 59, 0, time.UTC)

	// When
	err := composeTasks(TaskRuntime{scheduler: registry}, dependencies)

	// Then
	if err != nil {
		t.Fatalf("composeTasks() error = %v", err)
	}
	wantNames := []string{"sonarr-title-sync", "sonarr-rule-sync", "sonarr-rename", "radarr-title-sync", "radarr-rule-sync", "radarr-rename", "downloader-login"}
	if got := taskNames(registry.jobs); !reflect.DeepEqual(got, wantNames) {
		t.Fatalf("job names = %#v, want %#v", got, wantNames)
	}
	wantNext := []time.Time{
		time.Date(2026, time.August, 7, 11, 0, 0, 0, time.UTC),
		time.Date(2026, time.August, 8, 0, 15, 0, 0, time.UTC),
		time.Date(2026, time.August, 7, 11, 0, 0, 0, time.UTC),
		time.Date(2026, time.August, 7, 11, 30, 0, 0, time.UTC),
		time.Date(2026, time.August, 8, 1, 45, 0, 0, time.UTC),
		time.Date(2026, time.August, 7, 11, 0, 0, 0, time.UTC),
		time.Date(2026, time.August, 7, 11, 0, 0, 0, time.UTC),
	}
	for index, job := range registry.jobs {
		if got := job.Schedule.Next(start); !got.Equal(wantNext[index]) {
			t.Fatalf("job %q next = %s, want %s", job.Name, got, wantNext[index])
		}
		if job.ImmediateRun != nil && job.Name != "downloader-login" {
			t.Fatalf("job %q has unexpected immediate callback", job.Name)
		}
	}
	if calls.any() {
		t.Fatal("callbacks ran during composition")
	}
}

func TestNewTaskRuntime_createsSchedulerWithoutStarting(t *testing.T) {
	// Given
	cfg := config.Config{HTTPTimeout: 17 * time.Second}

	// When
	runtime := NewTaskRuntime(cfg, nil, nil)

	// Then
	if runtime.scheduler == nil {
		t.Fatal("scheduler = nil")
	}
}

func TestComposeTasks_usesDistinctDownloaderLoginCallbacks(t *testing.T) {
	// Given
	registry := &taskRegistry{}
	calls := &taskCalls{}
	dependencies := newTaskDependencies(calls)

	// When
	err := composeTasks(TaskRuntime{scheduler: registry}, dependencies)
	if err != nil {
		t.Fatalf("composeTasks() error = %v", err)
	}
	job := registry.jobs[6]
	if err := job.ImmediateRun(context.Background()); err != nil {
		t.Fatalf("ImmediateRun() error = %v", err)
	}
	if err := job.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	// Then
	if calls.loginStartup != 1 || calls.loginRefresh != 1 {
		t.Fatalf("login calls = startup %d, refresh %d", calls.loginStartup, calls.loginRefresh)
	}
}

func TestComposeTasks_rejectsMissingCallbacksBeforeRegistration(t *testing.T) {
	// Given
	registry := &taskRegistry{}
	dependencies := newTaskDependencies(&taskCalls{})
	dependencies.sonarrRename = nil

	// When
	err := composeTasks(TaskRuntime{scheduler: registry}, dependencies)

	// Then
	if err == nil || len(registry.jobs) != 0 {
		t.Fatalf("composeTasks() error = %v, registered = %d", err, len(registry.jobs))
	}
}

func TestComposeTasks_propagatesSchedulerAddError(t *testing.T) {
	// Given
	registry := &taskRegistry{err: errors.New("add failed")}

	// When
	err := composeTasks(TaskRuntime{scheduler: registry}, newTaskDependencies(&taskCalls{}))

	// Then
	if !errors.Is(err, registry.err) {
		t.Fatalf("composeTasks() error = %v, want %v", err, registry.err)
	}
}

type taskRegistry struct {
	jobs []tasks.Job
	err  error
}

func (registry *taskRegistry) Add(job tasks.Job) error {
	if registry.err != nil {
		return registry.err
	}
	registry.jobs = append(registry.jobs, job)
	return nil
}

func (*taskRegistry) Start(context.Context) {}
func (*taskRegistry) Wait()                 {}

type taskCalls struct{ loginStartup, loginRefresh int }

func (calls *taskCalls) any() bool { return calls.loginStartup != 0 || calls.loginRefresh != 0 }

func newTaskDependencies(calls *taskCalls) taskDependencies {
	callback := func(context.Context) error { return nil }
	return taskDependencies{
		sync:         syncTasks{sonarrTitle: callback, sonarrRule: callback, radarrTitle: callback, radarrRule: callback},
		sonarrRename: callback,
		radarrRename: callback,
		loginStartup: func(context.Context) error { calls.loginStartup++; return nil },
		loginRefresh: func(context.Context) error { calls.loginRefresh++; return nil },
	}
}

func taskNames(jobs []tasks.Job) []string {
	names := make([]string, len(jobs))
	for index, job := range jobs {
		names[index] = job.Name
	}
	return names
}
