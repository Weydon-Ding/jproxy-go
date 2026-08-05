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
	result, err := spec.syncer.Sync(request.Context())
	if errors.Is(err, ErrSyncUnavailable) {
		writeError(writer, http.StatusServiceUnavailable)
		return
	}
	if err != nil {
		h.deleteMarker(writer, spec.marker)
		return
	}
	if result == SyncTooFrequent {
		writeError(writer, http.StatusBadRequest)
		return
	}
	h.completeMutation(writer, request, spec.invalidation...)
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
