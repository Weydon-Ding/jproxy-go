package title

import (
	"context"
	"net/http"
	"strings"

	"jproxy-go/internal/runtime"
	"jproxy-go/internal/store/sqlite"
)

type Store interface{ Repositories() sqlite.Repositories }

type Options struct {
	Store      Store
	Provider   runtime.Provider
	Invalidate func(context.Context, ...string) error
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
	case http.MethodGet + " radarr/title/query":
		h.queryRadarr(writer, request)
	case http.MethodPost + " radarr/title/remove":
		h.removeRadarr(writer, request)
	case http.MethodGet + " tmdb/title/query":
		h.queryTMDB(writer, request)
	case http.MethodPost + " tmdb/title/remove":
		h.removeTMDB(writer, request)
	case http.MethodPost + " tmdb/title/save":
		h.saveTMDB(writer, request)
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
	case "sonarr/title/remove", "radarr/title/remove", "tmdb/title/remove", "tmdb/title/save":
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
