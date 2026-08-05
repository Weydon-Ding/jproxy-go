package app

import (
	"context"
	"net/http"

	"jproxy-go/internal/api/example"
	"jproxy-go/internal/api/rule"
	"jproxy-go/internal/api/system"
	"jproxy-go/internal/api/title"
	"jproxy-go/internal/runtime"
)

type managementStore interface{ system.Store }

type unavailableRuleSyncer struct{}

func (unavailableRuleSyncer) Sync(context.Context) (rule.SyncResult, error) {
	return rule.SyncSucceeded, rule.ErrSyncUnavailable
}

type unavailableTitleSyncer struct{}

func (unavailableTitleSyncer) Sync(context.Context) (title.SyncResult, error) {
	return title.SyncSucceeded, title.ErrSyncUnavailable
}

type titleSyncDependencies struct {
	sonarr    title.Syncer
	radarr    title.Syncer
	admission title.SyncAdmission
}

func managementRoutes(store managementStore, provider runtime.Provider, registry *runtime.Registry, syncDependencies titleSyncDependencies) http.Handler {
	return managementRoutesWithMultipartAccess(store, provider, registry, nil, syncDependencies)
}

func managementRoutesWithMultipartAccess(store managementStore, provider runtime.Provider, registry *runtime.Registry, multipartAccess rule.MultipartAccess, syncDependencies titleSyncDependencies) http.Handler {
	root := http.NewServeMux()
	ruleOptions := func(domain string) rule.Options {
		return rule.Options{
			Store: store, Domain: domain, Syncer: unavailableRuleSyncer{}, MultipartAccess: multipartAccess,
			Invalidate: func(ctx context.Context, name string) error { return registry.Invalidate(ctx, name) },
		}
	}
	root.Handle("/api/sonarr/rule/", rule.NewHandler(ruleOptions("sonarr")))
	root.Handle("/api/radarr/rule/", rule.NewHandler(ruleOptions("radarr")))
	root.Handle("/api/rule/test", rule.NewTestHandler())
	root.Handle("/api/sonarr/example/", example.NewHandler(example.Options{Store: store, Provider: provider, Domain: "sonarr"}))
	root.Handle("/api/radarr/example/", example.NewHandler(example.Options{Store: store, Provider: provider, Domain: "radarr"}))
	titleHandler := managementTitleHandler(store, provider, registry, syncDependencies)
	root.Handle("/api/sonarr/title/", titleHandler)
	root.Handle("/api/radarr/title/", titleHandler)
	root.Handle("/api/tmdb/title/", titleHandler)
	root.Handle("/api/system/", system.NewHandler(system.Options{Store: store, Provider: provider, Registry: registry, Version: localBuildVersion()}))
	return root
}

func managementTitleHandler(store managementStore, provider runtime.Provider, registry *runtime.Registry, syncDependencies titleSyncDependencies) http.Handler {
	sonarr := syncDependencies.sonarr
	if sonarr == nil {
		sonarr = unavailableTitleSyncer{}
	}
	radarr := syncDependencies.radarr
	if radarr == nil {
		radarr = unavailableTitleSyncer{}
	}
	return title.NewHandler(title.Options{
		Store:        store,
		Provider:     provider,
		Invalidate:   registry.Invalidate,
		DeleteMarker: registry.DeleteMarker,
		Admission:    syncDependencies.admission,
		SonarrSyncer: sonarr,
		RadarrSyncer: radarr,
		TMDBSyncer:   unavailableTitleSyncer{},
	})
}
