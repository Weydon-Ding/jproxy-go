package title

import (
	"errors"
	"net/http"
	"strconv"

	"jproxy-go/internal/format"
	"jproxy-go/internal/store/sqlite"
)

type queryInput struct {
	page  sqlite.PageInput
	title *string
	id    *int64
}

func (h *Handler) querySonarr(writer http.ResponseWriter, request *http.Request) {
	input, err := parseQuery(request, "tvdbId")
	if err != nil {
		writeError(writer, http.StatusBadRequest)
		return
	}
	page, err := h.options.Store.Repositories().SonarrTitles.Page(request.Context(), sqlite.SonarrTitleFilter{Page: input.page, Title: input.title, TVDBID: input.id})
	if err != nil {
		writeError(writer, http.StatusInternalServerError)
		return
	}
	rows := make([]sonarrTitleDTO, len(page.List))
	for index, value := range page.List {
		rows[index] = fromSonarr(value)
	}
	writeJSON(writer, http.StatusOK, pageDTO[sonarrTitleDTO]{Current: page.Current, PageSize: page.Size, Total: page.Total, List: rows})
}

func (h *Handler) queryRadarr(writer http.ResponseWriter, request *http.Request) {
	input, err := parseQuery(request, "tmdbId")
	if err != nil {
		writeError(writer, http.StatusBadRequest)
		return
	}
	page, err := h.options.Store.Repositories().RadarrTitles.Page(request.Context(), sqlite.RadarrTitleFilter{Page: input.page, Title: input.title, TMDBID: input.id})
	if err != nil {
		writeError(writer, http.StatusInternalServerError)
		return
	}
	rows := make([]radarrTitleDTO, len(page.List))
	for index, value := range page.List {
		rows[index] = fromRadarr(value)
	}
	writeJSON(writer, http.StatusOK, pageDTO[radarrTitleDTO]{Current: page.Current, PageSize: page.Size, Total: page.Total, List: rows})
}

func (h *Handler) queryTMDB(writer http.ResponseWriter, request *http.Request) {
	input, err := parseQuery(request, "tvdbId")
	if err != nil {
		writeError(writer, http.StatusBadRequest)
		return
	}
	page, err := h.options.Store.Repositories().TMDBTitles.Page(request.Context(), sqlite.TMDBTitleFilter{Page: input.page, Title: input.title, TVDBID: input.id})
	if err != nil {
		writeError(writer, http.StatusInternalServerError)
		return
	}
	regex := ""
	if h.options.Provider != nil {
		regex = h.options.Provider.Snapshot().Sonarr.CleanTitleRegex
	}
	rows := make([]tmdbTitleDTO, len(page.List))
	for index, value := range page.List {
		rows[index] = fromTMDB(value, format.CleanTitle(value.Title, regex))
	}
	writeJSON(writer, http.StatusOK, pageDTO[tmdbTitleDTO]{Current: page.Current, PageSize: page.Size, Total: page.Total, List: rows})
}

func parseQuery(request *http.Request, idKey string) (queryInput, error) {
	values := request.URL.Query()
	for key, values := range values {
		if len(values) != 1 || (key != "current" && key != "pageSize" && key != "title" && key != idKey) {
			return queryInput{}, errors.New("invalid query parameter")
		}
	}
	current, err := parsePageValue(values.Get("current"), 1, 1, 2147483647)
	if err != nil {
		return queryInput{}, err
	}
	size, err := parsePageValue(values.Get("pageSize"), 10, 1, maxBatch)
	if err != nil {
		return queryInput{}, err
	}
	input := queryInput{page: sqlite.PageInput{Current: current, Size: size}}
	if title, ok := values["title"]; ok {
		input.title = &title[0]
	}
	if raw, ok := values[idKey]; ok {
		id, err := parseInteger(raw[0])
		if err != nil {
			return queryInput{}, err
		}
		input.id = &id
	}
	return input, nil
}

func parsePageValue(raw string, fallback, minimum, maximum int64) (int64, error) {
	if raw == "" {
		return fallback, nil
	}
	value, err := parseInteger(raw)
	if err != nil || value < minimum || value > maximum {
		return 0, errors.New("invalid page")
	}
	return value, nil
}

func parseInteger(raw string) (int64, error) {
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value < -2147483648 || value > 2147483647 {
		return 0, errors.New("invalid integer")
	}
	return value, nil
}
