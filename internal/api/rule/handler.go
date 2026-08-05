package rule

import (
	"context"
	"errors"
	"jproxy-go/internal/store/sqlite"
	"net/http"
	"strings"
)

var ErrSyncUnavailable = errors.New("rule sync unavailable")

type SyncResult uint8

const (
	SyncSucceeded SyncResult = iota
	SyncTooFrequent
)

type Syncer interface {
	Sync(context.Context) (SyncResult, error)
}
type Store interface{ Repositories() sqlite.Repositories }
type Options struct {
	Store      Store
	Domain     string
	Invalidate func(context.Context, string) error
	Syncer     Syncer
}
type Handler struct{ options Options }

func NewHandler(o Options) *Handler { return &Handler{options: o} }
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/"+h.options.Domain+"/rule")
	switch r.Method + " " + path {
	case "GET /query":
		h.query(w, r)
	case "POST /save":
		h.save(w, r)
	case "POST /remove":
		h.ids(w, r, "remove")
	case "POST /enable":
		h.ids(w, r, "enable")
	case "POST /disable":
		h.ids(w, r, "disable")
	case "POST /export":
		h.export(w, r)
	case "POST /import":
		h.importRules(w, r)
	case "GET /token/list":
		h.tokens(w, r)
	case "POST /sync":
		h.sync(w, r)
	default:
		allow := ""
		if path == "/query" || path == "/token/list" {
			allow = "GET"
		}
		if strings.Contains("/sync /save /remove /enable /disable /export /import", path) {
			allow = "POST"
		}
		if allow != "" {
			w.Header().Set("Allow", allow)
			writeError(w, 405)
			return
		}
		http.NotFound(w, r)
	}
}
