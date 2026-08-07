package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"jproxy-go/internal/config"
	"jproxy-go/internal/tasks"
)

var errTaskDependencyUnavailable = errors.New("task dependency unavailable")

type taskScheduler interface {
	Add(tasks.Job) error
	Start(context.Context)
	Wait()
}

type TaskRuntime struct{ scheduler taskScheduler }

// NewTaskRuntime creates the task scheduler with application-wide execution limits.
func NewTaskRuntime(cfg config.Config, clock tasks.Clock, logger *slog.Logger) TaskRuntime {
	return TaskRuntime{scheduler: tasks.NewScheduler(tasks.SchedulerOptions{
		Clock: clock, Logger: logger, MaxConcurrency: 1, Timeout: cfg.HTTPTimeout,
	})}
}

func (runtime TaskRuntime) Start(ctx context.Context) { runtime.scheduler.Start(ctx) }
func (runtime TaskRuntime) Wait()                     { runtime.scheduler.Wait() }

type taskDependencies struct {
	sync         syncTasks
	sonarrRename scheduledTask
	radarrRename scheduledTask
	loginStartup scheduledTask
	loginRefresh scheduledTask
}

func composeTasks(runtime TaskRuntime, dependencies taskDependencies) error {
	if runtime.scheduler == nil {
		return fmt.Errorf("task runtime: %w", errTaskDependencyUnavailable)
	}
	if err := validateTaskDependencies(dependencies); err != nil {
		return err
	}
	jobs, err := configuredJobs(dependencies)
	if err != nil {
		return err
	}
	for _, job := range jobs {
		if err := runtime.scheduler.Add(job); err != nil {
			return fmt.Errorf("register task %q: %w", job.Name, err)
		}
	}
	return nil
}

func validateTaskDependencies(dependencies taskDependencies) error {
	for name, callback := range map[string]scheduledTask{
		"sonarr-title-sync":  dependencies.sync.sonarrTitle,
		"sonarr-rule-sync":   dependencies.sync.sonarrRule,
		"sonarr-rename":      dependencies.sonarrRename,
		"radarr-title-sync":  dependencies.sync.radarrTitle,
		"radarr-rule-sync":   dependencies.sync.radarrRule,
		"radarr-rename":      dependencies.radarrRename,
		"downloader-startup": dependencies.loginStartup,
		"downloader-refresh": dependencies.loginRefresh,
	} {
		if callback == nil {
			return fmt.Errorf("task %q: %w", name, errTaskDependencyUnavailable)
		}
	}
	return nil
}

func configuredJobs(dependencies taskDependencies) ([]tasks.Job, error) {
	sonarrTitle, err := tasks.Hourly(0, 0)
	if err != nil {
		return nil, err
	}
	sonarrRule, err := tasks.Daily(0, 15, 0)
	if err != nil {
		return nil, err
	}
	sonarrRename, err := tasks.EverySeconds(30)
	if err != nil {
		return nil, err
	}
	radarrTitle, err := tasks.Hourly(30, 0)
	if err != nil {
		return nil, err
	}
	radarrRule, err := tasks.Daily(1, 45, 0)
	if err != nil {
		return nil, err
	}
	radarrRename, err := tasks.EveryMinutes(1, 0)
	if err != nil {
		return nil, err
	}
	downloaderLogin, err := tasks.EveryMinutes(30, 0)
	if err != nil {
		return nil, err
	}
	return []tasks.Job{
		{Name: "sonarr-title-sync", Schedule: sonarrTitle, Run: dependencies.sync.sonarrTitle},
		{Name: "sonarr-rule-sync", Schedule: sonarrRule, Run: dependencies.sync.sonarrRule},
		{Name: "sonarr-rename", Schedule: sonarrRename, Run: dependencies.sonarrRename},
		{Name: "radarr-title-sync", Schedule: radarrTitle, Run: dependencies.sync.radarrTitle},
		{Name: "radarr-rule-sync", Schedule: radarrRule, Run: dependencies.sync.radarrRule},
		{Name: "radarr-rename", Schedule: radarrRename, Run: dependencies.radarrRename},
		{Name: "downloader-login", Schedule: downloaderLogin, ImmediateRun: dependencies.loginStartup, Run: dependencies.loginRefresh},
	}, nil
}
