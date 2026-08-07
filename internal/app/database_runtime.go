package app

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"jproxy-go/internal/config"
	"jproxy-go/internal/downloader/qbittorrent"
	"jproxy-go/internal/downloader/transmission"
	"jproxy-go/internal/proxy"
	"jproxy-go/internal/runtime"
	"jproxy-go/internal/tasks/rename"
)

const renameFallback = 30 * time.Minute

type taskRuntime interface {
	Start(context.Context)
	Wait()
}

type databaseRuntime struct {
	handler http.Handler
	tasks   taskRuntime
}

func composeDatabaseRuntime(cfg config.Config, provider runtime.Provider, store managementStore, logger *slog.Logger) (databaseRuntime, error) {
	proxyServer := proxy.NewServerWithRuntime(cfg, proxy.RuntimeOptions{Provider: provider})
	registry := proxyServer.CacheRegistry()
	titles := liveTitleSyncDependencies(cfg, store, registry)
	rules := liveRuleSyncDependencies(cfg, store, func(ctx context.Context, name string) error {
		return registry.Invalidate(ctx, name)
	})
	handler := rootHandlerWithProxyServer(cfg, provider, store, proxyServer, managementSyncDependencies{title: titles, rule: rules})
	if rules.sonarr == nil || rules.radarr == nil {
		return databaseRuntime{}, errors.New("database rule sync dependencies unavailable")
	}
	qb, err := qbittorrent.New(qbittorrent.Options{Provider: provider, Timeout: cfg.HTTPTimeout})
	if err != nil {
		return databaseRuntime{}, err
	}
	transmission, err := transmission.New(transmission.Options{Provider: provider, Timeout: cfg.HTTPTimeout})
	if err != nil {
		return databaseRuntime{}, err
	}
	history, err := rename.NewHistoryClient(rename.HistoryClientOptions{Timeout: cfg.HTTPTimeout})
	if err != nil {
		return databaseRuntime{}, err
	}
	repositories := store.Repositories()
	runtime := NewTaskRuntime(cfg, nil, logger)
	dependencies := taskDependencies{
		sync:         newSyncTasks(titles, rules, registry.Invalidate),
		sonarrRename: liveRenameTask(cfg, provider, repositories.SystemConfigs, history, qb, transmission, true),
		radarrRename: liveRenameTask(cfg, provider, repositories.SystemConfigs, history, qb, transmission, false),
		loginStartup: startupLoginTask(qb, transmission),
		loginRefresh: qb.Login,
	}
	if err := composeTasks(runtime, dependencies); err != nil {
		return databaseRuntime{}, err
	}
	return databaseRuntime{handler: handler, tasks: runtime}, nil
}

func liveRenameTask(cfg config.Config, provider runtime.Provider, source rename.Config, history rename.History, qb rename.QBittorrent, transmission rename.TorrentDownloader, sonarr bool) scheduledTask {
	return func(ctx context.Context) error {
		snapshot := provider.Snapshot()
		files := rename.FileOptions{Enabled: cfg.Task.RenameFiles}
		if sonarr {
			files.SonarrFormat = snapshot.Sonarr.Format
			files.SonarrRules = snapshot.Sonarr.Rules
		}
		service, err := rename.NewService(rename.ServiceOptions{History: history, Config: source, QBittorrent: qb, Transmission: transmission, Files: files, Now: time.Now})
		if err != nil {
			return err
		}
		if sonarr {
			return service.RunConfiguredSonarr(ctx, renameFallback)
		}
		return service.RunConfiguredRadarr(ctx, renameFallback)
	}
}

func startupLoginTask(qb rename.LoginDownloader, transmission rename.LoginDownloader) scheduledTask {
	return func(ctx context.Context) error {
		return errors.Join(qb.Login(ctx), transmission.Login(ctx))
	}
}
