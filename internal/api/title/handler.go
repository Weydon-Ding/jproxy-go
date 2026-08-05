package title

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"jproxy-go/internal/runtime"
	"jproxy-go/internal/store/sqlite"
)

type Store interface{ Repositories() sqlite.Repositories }

var ErrSyncUnavailable = errors.New("title sync unavailable")

type SyncResult uint8

const (
	SyncSucceeded SyncResult = iota
	SyncTooFrequent
)

type Syncer interface {
	Sync(context.Context) (SyncResult, error)
}

type SyncAdmission interface {
	BeginTitleSync(string) (runtime.TitleSyncAttempt, error)
	FinishTitleSync(runtime.TitleSyncAttempt, bool)
}

type Options struct {
	Store        Store
	Provider     runtime.Provider
	Invalidate   func(context.Context, ...string) error
	DeleteMarker func(string) error
	Admission    SyncAdmission
	SonarrSyncer Syncer
	RadarrSyncer Syncer
	TMDBSyncer   Syncer
}

type Handler struct{ options Options }

func NewHandler(options Options) *Handler { return &Handler{options: options} }

func (h *Handler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	path := strings.TrimPrefix(request.URL.Path, "/api/")
	switch request.Method + " " + path {
	case http.MethodGet + " sonarr/title/query":
		h.querySonarr(writer, request)
	case http.MethodPost + " sonarr/title/remove":
		h.removeSonarr(writer, request)
	case http.MethodPost + " sonarr/title/sync":
		h.sync(writer, request, syncSpec{syncer: h.options.SonarrSyncer, marker: runtime.SonarrTitleSyncInterval, invalidation: sonarrInvalidation})
	case http.MethodGet + " radarr/title/query":
		h.queryRadarr(writer, request)
	case http.MethodPost + " radarr/title/remove":
		h.removeRadarr(writer, request)
	case http.MethodPost + " radarr/title/sync":
		h.sync(writer, request, syncSpec{syncer: h.options.RadarrSyncer, marker: runtime.RadarrTitleSyncInterval, invalidation: radarrInvalidation})
	case http.MethodGet + " tmdb/title/query":
		h.queryTMDB(writer, request)
	case http.MethodPost + " tmdb/title/remove":
		h.removeTMDB(writer, request)
	case http.MethodPost + " tmdb/title/save":
		h.saveTMDB(writer, request)
	case http.MethodPost + " tmdb/title/sync":
		h.sync(writer, request, syncSpec{syncer: h.options.TMDBSyncer, marker: runtime.TMDBTitleSyncInterval, invalidation: sonarrInvalidation})
	default:
		if method := allowedMethod(path); method != "" {
			writer.Header().Set("Allow", method)
			writeError(writer, http.StatusMethodNotAllowed)
			return
		}
		http.NotFound(writer, request)
	}
}

func allowedMethod(path string) string {
	switch path {
	case "sonarr/title/query", "radarr/title/query", "tmdb/title/query":
		return http.MethodGet
	case "sonarr/title/remove", "sonarr/title/sync", "radarr/title/remove", "radarr/title/sync", "tmdb/title/remove", "tmdb/title/save", "tmdb/title/sync":
		return http.MethodPost
	default:
		return ""
	}
}

func (h *Handler) invalidate(ctx context.Context, names ...string) error {
	if h.options.Invalidate == nil {
		return nil
	}
	return h.options.Invalidate(ctx, names...)
}
