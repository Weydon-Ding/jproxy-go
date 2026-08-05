package proxy

import (
	"jproxy-go/internal/cache"
	"jproxy-go/internal/config"
	"jproxy-go/internal/runtime"
	"jproxy-go/internal/store/sqlite"
)

type RuntimeOptions struct {
	Provider runtime.Provider
	Markers  *cache.TTLCache[struct{}]
}

func NewServerWithRuntime(cfg config.Config, options RuntimeOptions) *Server {
	provider := options.Provider
	if provider == nil {
		provider = runtime.NewStaticProvider(sqlite.Snapshot{Radarr: cfg.RadarrFormatting.Config, Sonarr: cfg.SonarrFormatting.Config, JackettURL: cfg.JackettURL, ProwlarrURL: cfg.ProwlarrURL})
	}
	results := cache.NewTTLCache[string](cfg.IndexerResultCacheTTL, cfg.ResultCacheMaxEntries)
	offsets := cache.NewTTLCache[[]int](cfg.OffsetCacheTTL, cfg.OffsetCacheMaxEntries)
	markers := options.Markers
	if markers == nil {
		markers = cache.NewTTLCache[struct{}](cfg.OffsetCacheTTL, 3)
	}
	return &Server{
		cfg:         cfg,
		client:      newHTTPClient(cfg),
		provider:    provider,
		resultCache: results,
		offsetCache: offsets,
		registry:    runtime.NewRegistry(provider, results, offsets, markers),
	}
}

func (s *Server) CacheRegistry() *runtime.Registry { return s.registry }
