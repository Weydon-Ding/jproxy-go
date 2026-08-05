package title

import (
	"net/http"

	"jproxy-go/internal/runtime"
	"jproxy-go/internal/store/sqlite"
)

func (h *Handler) removeSonarr(writer http.ResponseWriter, request *http.Request) {
	ids, err := decodeIDs(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest)
		return
	}
	values := make([]sqlite.SonarrTitleID, len(ids))
	for index, id := range ids {
		values[index] = sqlite.SonarrTitleID(id)
	}
	if err := h.options.Store.Repositories().SonarrTitles.DeleteBatch(request.Context(), sqlite.SonarrTitleIDs{IDs: values}); err != nil {
		writeError(writer, http.StatusInternalServerError)
		return
	}
	h.completeMutation(writer, request, runtime.SonarrSearchTitle, runtime.IndexerSearchOffset, runtime.SonarrResultTitle)
}

func (h *Handler) removeRadarr(writer http.ResponseWriter, request *http.Request) {
	ids, err := decodeIDs(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest)
		return
	}
	values := make([]sqlite.RadarrTitleID, len(ids))
	for index, id := range ids {
		values[index] = sqlite.RadarrTitleID(id)
	}
	if err := h.options.Store.Repositories().RadarrTitles.DeleteBatch(request.Context(), sqlite.RadarrTitleIDs{IDs: values}); err != nil {
		writeError(writer, http.StatusInternalServerError)
		return
	}
	h.completeMutation(writer, request, runtime.RadarrSearchTitle, runtime.IndexerSearchOffset, runtime.RadarrResultTitle)
}

func (h *Handler) removeTMDB(writer http.ResponseWriter, request *http.Request) {
	ids, err := decodeIDs(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest)
		return
	}
	values := make([]sqlite.TMDBTitleID, len(ids))
	for index, id := range ids {
		values[index] = sqlite.TMDBTitleID(id)
	}
	if err := h.options.Store.Repositories().TMDBTitles.DeleteBatch(request.Context(), sqlite.TMDBTitleIDs{IDs: values}); err != nil {
		writeError(writer, http.StatusInternalServerError)
		return
	}
	h.completeMutation(writer, request, runtime.SonarrSearchTitle, runtime.IndexerSearchOffset, runtime.SonarrResultTitle)
}

func (h *Handler) saveTMDB(writer http.ResponseWriter, request *http.Request) {
	var input tmdbTitleDTO
	if err := decodeJSON(request, &input); err != nil || !validTMDBInput(input) {
		writeError(writer, http.StatusBadRequest)
		return
	}
	title := sqlite.TMDBTitle{TVDBID: input.TVDBID, TMDBID: input.TMDBID, Language: input.Language, Title: input.Title, ValidStatus: sqlite.ValidStatus(input.ValidStatus), CreateTime: input.CreateTime, UpdateTime: input.UpdateTime}
	if input.ID != nil {
		title.ID = sqlite.TMDBTitleID(*input.ID)
	}
	if _, err := h.options.Store.Repositories().TMDBTitles.Save(request.Context(), sqlite.TMDBTitleSaveInput{Title: title, SuppliedID: input.ID != nil}); err != nil {
		writeError(writer, http.StatusInternalServerError)
		return
	}
	h.completeMutation(writer, request, runtime.SonarrSearchTitle, runtime.IndexerSearchOffset, runtime.SonarrResultTitle)
}

func (h *Handler) completeMutation(writer http.ResponseWriter, request *http.Request, names ...string) {
	if err := h.invalidate(request.Context(), names...); err != nil {
		writeError(writer, http.StatusInternalServerError)
		return
	}
	writer.WriteHeader(http.StatusOK)
}

func decodeIDs(request *http.Request) ([]int64, error) {
	var ids []int64
	if err := decodeJSON(request, &ids); err != nil {
		return nil, err
	}
	if len(ids) > maxBatch {
		return nil, errBatchTooLarge
	}
	for _, id := range ids {
		if _, err := parseIntegerInt64(id); err != nil {
			return nil, err
		}
	}
	return ids, nil
}
