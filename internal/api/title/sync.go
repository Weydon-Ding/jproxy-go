package title

import (
	"errors"
	"net/http"

	"jproxy-go/internal/runtime"
)

var (
	sonarrInvalidation = []string{runtime.SonarrSearchTitle, runtime.IndexerSearchOffset, runtime.SonarrResultTitle}
	radarrInvalidation = []string{runtime.RadarrSearchTitle, runtime.IndexerSearchOffset, runtime.RadarrResultTitle}
)

type syncSpec struct {
	syncer       Syncer
	marker       string
	invalidation []string
}

func (h *Handler) sync(writer http.ResponseWriter, request *http.Request, spec syncSpec) {
	if spec.syncer == nil {
		writeError(writer, http.StatusServiceUnavailable)
		return
	}
	var attempt runtime.TitleSyncAttempt
	if h.options.Admission != nil {
		var err error
		attempt, err = h.options.Admission.BeginTitleSync(spec.marker)
		if errors.Is(err, runtime.ErrTitleSyncTooFrequent) {
			writeError(writer, http.StatusBadRequest)
			return
		}
		if err != nil {
			writeError(writer, http.StatusInternalServerError)
			return
		}
	}
	result, err := spec.syncer.Sync(request.Context())
	if errors.Is(err, ErrSyncUnavailable) {
		h.finishSyncAttempt(attempt, false)
		writeError(writer, http.StatusServiceUnavailable)
		return
	}
	if err != nil {
		h.finishSyncAttempt(attempt, false)
		h.deleteMarker(writer, spec.marker)
		return
	}
	if result == SyncTooFrequent {
		h.finishSyncAttempt(attempt, false)
		writeError(writer, http.StatusBadRequest)
		return
	}
	h.finishSyncAttempt(attempt, true)
	h.completeMutation(writer, request, spec.invalidation...)
}

func (h *Handler) finishSyncAttempt(attempt runtime.TitleSyncAttempt, succeeded bool) {
	if h.options.Admission != nil {
		h.options.Admission.FinishTitleSync(attempt, succeeded)
	}
}

func (h *Handler) deleteMarker(writer http.ResponseWriter, marker string) {
	if h.options.DeleteMarker != nil {
		if err := h.options.DeleteMarker(marker); err != nil {
			writeError(writer, http.StatusInternalServerError)
			return
		}
	}
	writeError(writer, http.StatusInternalServerError)
}
