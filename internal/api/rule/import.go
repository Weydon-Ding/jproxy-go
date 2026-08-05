package rule

import (
	"context"
	"encoding/json"
	"io"
	"jproxy-go/internal/store/sqlite"
	"mime"
	"net/http"
	"strings"
)

func (h *Handler) importRules(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	media, _, e := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if e != nil || media != "multipart/form-data" {
		writeError(w, 400)
		return
	}
	mr, e := r.MultipartReader()
	if e != nil {
		writeError(w, 400)
		return
	}
	p, e := mr.NextPart()
	if e != nil || p.FormName() != "file" || badName(p.FileName()) || badContentDisposition(p.Header.Get("Content-Disposition")) {
		writeError(w, 400)
		return
	}
	if h.options.MultipartObserver != nil {
		h.options.MultipartObserver.ObservePartRead()
	}
	var rows []ruleDTO
	d := json.NewDecoder(io.LimitReader(p, maxBodyBytes+1))
	d.DisallowUnknownFields()
	if d.Decode(&rows) != nil || d.Decode(&struct{}{}) != io.EOF || len(rows) == 0 || len(rows) > maxBatch {
		writeError(w, 400)
		return
	}
	if _, e = mr.NextPart(); e != io.EOF {
		writeError(w, 400)
		return
	}
	for i := range rows {
		if rows[i].ID == primaryID || validate(&rows[i]) != nil {
			writeError(w, 400)
			return
		}
		if rows[i].ID == "" {
			rows[i].ID, _ = id()
		}
	}
	if e = h.importRows(r.Context(), rows); e != nil {
		writeError(w, 500)
		return
	}
	h.done(w, r)
}
func badName(v string) bool {
	if len(v) == 0 || len(v) > 255 || strings.ContainsAny(v, "/\\") || strings.HasPrefix(v, ".") {
		return true
	}
	for _, r := range v {
		if r < 32 || r == 127 {
			return true
		}
	}
	return false
}

func badContentDisposition(value string) bool {
	return strings.Contains(value, "..") || strings.Contains(value, `\`) || strings.Contains(value, ":")
}
func (h *Handler) importRows(c context.Context, rows []ruleDTO) error {
	if h.options.Domain == "sonarr" {
		out := make([]sqlite.SonarrRule, len(rows))
		for i, v := range rows {
			out[i] = toRule(v)
		}
		return h.options.Store.Repositories().SonarrRules.Import(c, sqlite.SonarrRuleBatch{Rows: out})
	}
	out := make([]sqlite.RadarrRule, len(rows))
	for i, v := range rows {
		out[i] = sqlite.RadarrRule(toRule(v))
	}
	return h.options.Store.Repositories().RadarrRules.Import(c, sqlite.RadarrRuleBatch{Rows: out})
}
