package example

import (
	"errors"
	"jproxy-go/internal/format"
	"jproxy-go/internal/runtime"
	"jproxy-go/internal/store/sqlite"
	"net/http"
	"strconv"
	"strings"
)

func (h *Handler) query(writer http.ResponseWriter, request *http.Request) {
	filter, status, err := parseQuery(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest)
		return
	}
	snapshot := h.options.Provider.Snapshot()
	if status == nil {
		page, err := h.page(request, filter)
		if err != nil {
			writeError(writer, http.StatusInternalServerError)
			return
		}
		writeJSON(writer, http.StatusOK, pageDTO{Current: page.Current, PageSize: page.Size, Total: page.Total, List: h.project(page.List, snapshot)})
		return
	}
	rows, err := h.list(request, filter)
	if err != nil {
		writeError(writer, http.StatusInternalServerError)
		return
	}
	projected := h.project(rows, snapshot)
	matched := make([]exampleDTO, 0, len(projected))
	for _, row := range projected {
		if row.ValidStatus == *status {
			matched = append(matched, row)
		}
	}
	start := (filter.Page.Current - 1) * filter.Page.Size
	if start > int64(len(matched)) {
		start = int64(len(matched))
	}
	end := start + filter.Page.Size
	if end > int64(len(matched)) {
		end = int64(len(matched))
	}
	writeJSON(writer, http.StatusOK, pageDTO{Current: filter.Page.Current, PageSize: filter.Page.Size, Total: int64(len(matched)), List: matched[start:end]})
}

func (h *Handler) page(request *http.Request, filter sqlite.ExampleFilter) (sqlite.PageResult[sqlite.SonarrExample], error) {
	if h.options.Domain == "sonarr" {
		return h.options.Store.Repositories().SonarrExamples.Page(request.Context(), filter)
	}
	page, err := h.options.Store.Repositories().RadarrExamples.Page(request.Context(), filter)
	return convertPage(page), err
}

func (h *Handler) list(request *http.Request, filter sqlite.ExampleFilter) ([]sqlite.SonarrExample, error) {
	if h.options.Domain == "sonarr" {
		return h.options.Store.Repositories().SonarrExamples.List(request.Context(), filter)
	}
	rows, err := h.options.Store.Repositories().RadarrExamples.List(request.Context(), filter)
	result := make([]sqlite.SonarrExample, len(rows))
	for index, row := range rows {
		result[index] = sqlite.SonarrExample(row)
	}
	return result, err
}

func convertPage(page sqlite.PageResult[sqlite.RadarrExample]) sqlite.PageResult[sqlite.SonarrExample] {
	rows := make([]sqlite.SonarrExample, len(page.List))
	for index, row := range page.List {
		rows[index] = sqlite.SonarrExample(row)
	}
	return sqlite.PageResult[sqlite.SonarrExample]{Current: page.Current, Size: page.Size, Total: page.Total, List: rows}
}

func (h *Handler) project(rows []sqlite.SonarrExample, snapshot runtime.Snapshot) []exampleDTO {
	result := make([]exampleDTO, len(rows))
	for index, row := range rows {
		formatted := format.RadarrExample(row.OriginalText, snapshot.Radarr)
		if h.options.Domain == "sonarr" {
			formatted = format.SonarrExample(row.OriginalText, snapshot.Sonarr)
		}
		status := int64(0)
		if formatted != row.OriginalText {
			status = 1
		}
		result[index] = exampleDTO{Hash: row.Hash, OriginalText: row.OriginalText, FormatText: formatted, ValidStatus: status, CreateTime: text(row.CreateTime), UpdateTime: text(row.UpdateTime)}
	}
	return result
}

func parseQuery(request *http.Request) (sqlite.ExampleFilter, *int64, error) {
	current, size := int64(1), int64(10)
	var err error
	if raw := request.URL.Query().Get("current"); raw != "" {
		current, err = strconv.ParseInt(raw, 10, 64)
		if err != nil || current < 1 {
			return sqlite.ExampleFilter{}, nil, errors.New("page")
		}
	}
	if raw := request.URL.Query().Get("pageSize"); raw != "" {
		size, err = strconv.ParseInt(raw, 10, 64)
		if err != nil || size < 1 || size > maxBatch {
			return sqlite.ExampleFilter{}, nil, errors.New("page")
		}
	}
	filter := sqlite.ExampleFilter{Page: sqlite.PageInput{Current: current, Size: size}}
	if original := strings.TrimSpace(request.URL.Query().Get("originalText")); original != "" {
		filter.OriginalText = &original
	}
	if raw := request.URL.Query().Get("validStatus"); raw != "" {
		status, parseErr := strconv.ParseInt(raw, 10, 64)
		if parseErr != nil || (status != 0 && status != 1) {
			return filter, nil, errors.New("status")
		}
		return filter, &status, nil
	}
	return filter, nil, nil
}

func text(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
