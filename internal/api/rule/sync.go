package rule

import (
	"errors"
	"net/http"
)

func (h *Handler) sync(w http.ResponseWriter, r *http.Request) {
	if h.options.Syncer == nil {
		writeError(w, 503)
		return
	}
	result, err := h.options.Syncer.Sync(r.Context())
	if errors.Is(err, ErrSyncUnavailable) {
		writeError(w, 503)
		return
	}
	if err != nil {
		writeError(w, 500)
		return
	}
	if result == SyncTooFrequent {
		writeError(w, 400)
		return
	}
	h.done(w, r)
}
