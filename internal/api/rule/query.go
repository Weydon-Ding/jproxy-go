package rule

import (
	"jproxy-go/internal/store/sqlite"
	"net/http"
)

func (h *Handler) query(w http.ResponseWriter, r *http.Request) {
	cur, size, err := page(r)
	if err != nil {
		writeError(w, 400)
		return
	}
	f := sqlite.RuleFilter{Page: sqlite.PageInput{Current: cur, Size: size}}
	if v := r.URL.Query().Get("token"); v != "" {
		f.Token = &v
	}
	if v := r.URL.Query().Get("remark"); v != "" {
		f.Remark = &v
	}
	if h.options.Domain == "sonarr" {
		p, e := h.options.Store.Repositories().SonarrRules.Page(r.Context(), f)
		if e != nil {
			writeError(w, 500)
			return
		}
		rows := make([]ruleDTO, len(p.List))
		for i, v := range p.List {
			rows[i] = fromRule(v)
		}
		writeJSON(w, 200, pageDTO{Current: p.Current, PageSize: p.Size, Total: p.Total, List: rows})
		return
	}
	p, e := h.options.Store.Repositories().RadarrRules.Page(r.Context(), f)
	if e != nil {
		writeError(w, 500)
		return
	}
	rows := make([]ruleDTO, len(p.List))
	for i, v := range p.List {
		rows[i] = fromRule(sqlite.SonarrRule(v))
	}
	writeJSON(w, 200, pageDTO{Current: p.Current, PageSize: p.Size, Total: p.Total, List: rows})
}
func (h *Handler) export(w http.ResponseWriter, r *http.Request) {
	var ids []string
	if decodeJSON(r, &ids) != nil || len(ids) > maxBatch {
		writeError(w, 400)
		return
	}
	if h.options.Domain == "sonarr" {
		rows, e := h.options.Store.Repositories().SonarrRules.Export(r.Context(), ruleIDs(ids))
		if e != nil {
			writeError(w, 500)
			return
		}
		out := make([]ruleDTO, len(rows))
		for i, v := range rows {
			out[i] = fromRule(v)
		}
		writeJSON(w, 200, out)
		return
	}
	rows, e := h.options.Store.Repositories().RadarrRules.Export(r.Context(), ruleIDs(ids))
	if e != nil {
		writeError(w, 500)
		return
	}
	out := make([]ruleDTO, len(rows))
	for i, v := range rows {
		out[i] = fromRule(sqlite.SonarrRule(v))
	}
	writeJSON(w, 200, out)
}
func (h *Handler) tokens(w http.ResponseWriter, r *http.Request) {
	if h.options.Domain == "sonarr" {
		v, e := h.options.Store.Repositories().SonarrRules.Tokens(r.Context())
		if e != nil {
			writeError(w, 500)
			return
		}
		writeJSON(w, 200, v)
		return
	}
	v, e := h.options.Store.Repositories().RadarrRules.Tokens(r.Context())
	if e != nil {
		writeError(w, 500)
		return
	}
	writeJSON(w, 200, v)
}
