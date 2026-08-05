package example

import (
	"crypto/md5"
	"encoding/hex"
	"jproxy-go/internal/store/sqlite"
	"net/http"
	"strings"
	"unicode/utf8"
)

const maxBatch = 200
const maxTextBytes = 16 * 1024

func (h *Handler) save(writer http.ResponseWriter, request *http.Request) {
	var input saveDTO
	if decodeJSON(request, &input) != nil || !validText(input.OriginalText) {
		writeError(writer, http.StatusBadRequest)
		return
	}
	lines := javaSplitLines(input.OriginalText)
	if len(lines) == 0 || len(lines) > maxBatch || !validLines(lines) {
		writeError(writer, http.StatusBadRequest)
		return
	}
	if err := h.upsert(request, lines); err != nil {
		writeError(writer, http.StatusInternalServerError)
		return
	}
	writer.WriteHeader(http.StatusOK)
}

func (h *Handler) remove(writer http.ResponseWriter, request *http.Request) {
	var hashes []string
	if decodeJSON(request, &hashes) != nil || len(hashes) > maxBatch || !validHashes(hashes) {
		writeError(writer, http.StatusBadRequest)
		return
	}
	ids := sqlite.ExampleIDs{IDs: hashes}
	var err error
	if h.options.Domain == "sonarr" {
		err = h.options.Store.Repositories().SonarrExamples.DeleteBatch(request.Context(), ids)
	} else {
		err = h.options.Store.Repositories().RadarrExamples.DeleteBatch(request.Context(), ids)
	}
	if err != nil {
		writeError(writer, http.StatusInternalServerError)
		return
	}
	writer.WriteHeader(http.StatusOK)
}

func (h *Handler) upsert(request *http.Request, lines []string) error {
	if h.options.Domain == "sonarr" {
		rows := make([]sqlite.SonarrExample, len(lines))
		for index, line := range lines {
			rows[index] = sqlite.SonarrExample{Hash: exampleHash(line), OriginalText: line, ValidStatus: sqlite.Invalid}
		}
		return h.options.Store.Repositories().SonarrExamples.UpsertBatch(request.Context(), sqlite.SonarrExampleBatch{Rows: rows})
	}
	rows := make([]sqlite.RadarrExample, len(lines))
	for index, line := range lines {
		rows[index] = sqlite.RadarrExample{Hash: exampleHash(line), OriginalText: line, ValidStatus: sqlite.Invalid}
	}
	return h.options.Store.Repositories().RadarrExamples.UpsertBatch(request.Context(), sqlite.RadarrExampleBatch{Rows: rows})
}

func javaSplitLines(text string) []string {
	lines := strings.Split(text, "\n")
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func validText(text string) bool {
	return strings.TrimSpace(text) != "" && len(text) <= maxTextBytes && utf8.ValidString(text) && !hasControl(text)
}
func validLines(lines []string) bool {
	for _, line := range lines {
		if len(line) > maxTextBytes || !utf8.ValidString(line) || hasControl(line) {
			return false
		}
	}
	return true
}
func hasControl(text string) bool {
	for _, value := range text {
		if (value < 0x20 && value != '\n' && value != '\r') || value == 0x7f {
			return true
		}
	}
	return false
}
func validHashes(hashes []string) bool {
	for _, hash := range hashes {
		if len(hash) != 32 {
			return false
		}
		for _, value := range hash {
			if !strings.ContainsRune("0123456789abcdefABCDEF", value) {
				return false
			}
		}
	}
	return true
}
func exampleHash(text string) string {
	sum := md5.Sum([]byte(text))
	return strings.ToUpper(hex.EncodeToString(sum[:]))
}
