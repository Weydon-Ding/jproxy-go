package app

import (
	"context"
	"net/http"

	"jproxy-go/internal/config"
	"jproxy-go/internal/proxy"
	"jproxy-go/internal/runtime"
	"jproxy-go/internal/ui"
)

type managementSyncDependencies struct {
	title titleSyncDependencies
	rule  ruleSyncDependencies
}

func rootHandler(cfg config.Config, provider runtime.Provider, store managementStore) http.Handler {
	return rootHandlerWithSyncDependencies(cfg, provider, store, managementSyncDependencies{})
}

func rootHandlerWithSyncDependencies(cfg config.Config, provider runtime.Provider, store managementStore, dependencies managementSyncDependencies) http.Handler {
	proxyServer := proxy.NewServerWithRuntime(cfg, proxy.RuntimeOptions{Provider: provider})
	return rootHandlerWithProxyServer(cfg, provider, store, proxyServer, dependencies)
}

func rootHandlerWithProxyServer(cfg config.Config, provider runtime.Provider, store managementStore, proxyServer *proxy.Server, dependencies managementSyncDependencies) http.Handler {
	if !cfg.Database.Enabled {
		return proxyServer.Routes()
	}
	root := http.NewServeMux()
	registry := proxyServer.CacheRegistry()
	if dependencies.title.admission == nil {
		dependencies.title = liveTitleSyncDependencies(cfg, store, registry)
	}
	if dependencies.rule.sonarr == nil && dependencies.rule.radarr == nil {
		dependencies.rule = liveRuleSyncDependencies(cfg, store, func(ctx context.Context, name string) error {
			return registry.Invalidate(ctx, name)
		})
	}
	management, err := securedManagementRoutes(cfg, store, provider, registry, dependencies.title, dependencies.rule)
	if err != nil {
		return http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			http.Error(writer, "service unavailable", http.StatusServiceUnavailable)
		})
	}
	root.Handle("/", ui.NewHandler())
	root.Handle("/api/", management)
	proxyRoutes := proxyServer.Routes()
	root.Handle("/sonarr/", proxyRoutes)
	root.Handle("/radarr/", proxyRoutes)
	root.Handle("/health", proxyRoutes)
	return root
}
