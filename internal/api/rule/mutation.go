package rule

import (
	"context"
	"jproxy-go/internal/store/sqlite"
	"net/http"
)

func (h *Handler) save(w http.ResponseWriter, r *http.Request) {
	var v ruleDTO
	if decodeJSON(r, &v) != nil || validate(&v) != nil || v.ID == primaryID {
		writeError(w, 400)
		return
	}
	if v.ID == "" {
		x, e := id()
		if e != nil {
			writeError(w, 500)
			return
		}
		v.ID = x
	}
	if e := h.upsert(r.Context(), v); e != nil {
		writeError(w, 500)
		return
	}
	h.done(w, r)
}
func (h *Handler) ids(w http.ResponseWriter, r *http.Request, action string) {
	var ids []string
	if decodeJSON(r, &ids) != nil || len(ids) > maxBatch || (action != "enable" && primary(ids)) {
		writeError(w, 400)
		return
	}
	var e error
	if action == "remove" {
		e = h.remove(r.Context(), ids)
	} else {
		s := sqlite.Valid
		if action == "disable" {
			s = sqlite.Invalid
		}
		e = h.switchStatus(r.Context(), ids, s)
	}
	if e != nil {
		writeError(w, 500)
		return
	}
	h.done(w, r)
}
func (h *Handler) upsert(c context.Context, v ruleDTO) error {
	row := toRule(v)
	if h.options.Domain == "sonarr" {
		return h.options.Store.Repositories().SonarrRules.Upsert(c, row)
	}
	return h.options.Store.Repositories().RadarrRules.Upsert(c, sqlite.RadarrRule(row))
}
func (h *Handler) remove(c context.Context, ids []string) error {
	if h.options.Domain == "sonarr" {
		return h.options.Store.Repositories().SonarrRules.DeleteBatch(c, ruleIDs(ids))
	}
	return h.options.Store.Repositories().RadarrRules.DeleteBatch(c, ruleIDs(ids))
}
func (h *Handler) switchStatus(c context.Context, ids []string, s sqlite.ValidStatus) error {
	if h.options.Domain == "sonarr" {
		return h.options.Store.Repositories().SonarrRules.SwitchValidStatus(c, ruleIDs(ids), s)
	}
	return h.options.Store.Repositories().RadarrRules.SwitchValidStatus(c, ruleIDs(ids), s)
}
func (h *Handler) done(w http.ResponseWriter, r *http.Request) {
	if h.options.Invalidate != nil {
		scope := "sonarr_rule"
		if h.options.Domain == "radarr" {
			scope = "radarr_rule"
		}
		if h.options.Invalidate(r.Context(), scope) != nil {
			writeError(w, 500)
			return
		}
	}
	w.WriteHeader(200)
}
