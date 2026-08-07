// Package system exposes the DB-only Java-compatible system management surface.
package system

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"

	"jproxy-go/internal/runtime"
	"jproxy-go/internal/store/sqlite"
	"jproxy-go/internal/transmissionconfig"
)

const maxBodyBytes = 256 * 1024

type Options struct {
	Store           Store
	Provider        runtime.Provider
	Registry        *runtime.Registry
	Version         string
	VersionURL      string
	AuthorURL       string
	AuthorBackupURL string
}

type Store interface {
	Repositories() sqlite.Repositories
	UpdateSystemConfigs(context.Context, []sqlite.SystemConfig) (sqlite.Snapshot, error)
}

type Handler struct {
	options Options
	mu      sync.Mutex
}

func NewHandler(options Options) *Handler { return &Handler{options: options} }

func (h *Handler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	switch request.Method + " " + request.URL.Path {
	case http.MethodGet + " /api/system/config/version":
		h.version(request.Context(), writer)
	case http.MethodGet + " /api/system/config/query":
		h.query(writer, request)
	case http.MethodPost + " /api/system/config/update":
		h.update(writer, request)
	case http.MethodGet + " /api/system/config/author/list":
		h.authors(request.Context(), writer)
	case http.MethodPost + " /api/system/cache/clearAll":
		h.clearAll(writer, request)
	case http.MethodPost + " /api/system/cache/clear":
		h.clear(writer, request)
	default:
		if method := allowedMethod(request.URL.Path); method != "" {
			writer.Header().Set("Allow", method)
			writeError(writer, http.StatusMethodNotAllowed)
			return
		}
		http.NotFound(writer, request)
	}
}

func allowedMethod(path string) string {
	if path == "/api/system/config/version" || path == "/api/system/config/query" || path == "/api/system/config/author/list" {
		return http.MethodGet
	}
	if path == "/api/system/config/update" || path == "/api/system/cache/clearAll" || path == "/api/system/cache/clear" {
		return http.MethodPost
	}
	return ""
}

func (h *Handler) version(ctx context.Context, writer http.ResponseWriter) {
	version := h.options.Version
	if version == "" {
		version = "dev"
	}
	source := h.options.VersionURL
	if source == "" {
		source = defaultVersionURL
	}
	if latest, ok := fetchVersion(ctx, source); ok && latest != version {
		version += " 🚨"
	}
	writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = writer.Write([]byte(version))
}

func (h *Handler) query(writer http.ResponseWriter, request *http.Request) {
	rows, err := h.options.Store.Repositories().SystemConfigs.List(request.Context())
	if err != nil {
		writeError(writer, http.StatusInternalServerError)
		return
	}
	sort.Slice(rows, func(left, right int) bool { return rows[left].ID < rows[right].ID })
	writeJSON(writer, http.StatusOK, maskedRows(rows))
}

func (h *Handler) update(writer http.ResponseWriter, request *http.Request) {
	rows, err := decodeRows(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest)
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	current, err := h.options.Store.Repositories().SystemConfigs.List(request.Context())
	if err == nil {
		err = restoreMaskedSecrets(rows, current)
	}
	if err == nil {
		err = validateRows(rows)
	}
	if err != nil {
		writeError(writer, http.StatusBadRequest)
		return
	}
	snapshot, err := h.options.Store.UpdateSystemConfigs(request.Context(), rows)
	if err != nil {
		writeError(writer, statusFor(err))
		return
	}
	if !runtime.PublishPrepared(h.options.Provider, snapshot) {
		writeError(writer, http.StatusInternalServerError)
		return
	}
	h.options.Registry.DeleteSystemConfigSyncMarkers()
	writer.WriteHeader(http.StatusOK)
}

func (h *Handler) clearAll(writer http.ResponseWriter, request *http.Request) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if err := h.options.Registry.InvalidateAll(request.Context()); err != nil {
		writeError(writer, http.StatusInternalServerError)
		return
	}
	writer.WriteHeader(http.StatusOK)
}

func (h *Handler) clear(writer http.ResponseWriter, request *http.Request) {
	var input struct {
		CacheName string `json:"cacheName"`
	}
	if err := decodeJSON(request, &input); err != nil || strings.TrimSpace(input.CacheName) == "" {
		writeError(writer, http.StatusBadRequest)
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if err := h.options.Registry.Invalidate(request.Context(), strings.TrimSpace(input.CacheName)); err != nil {
		writeError(writer, http.StatusBadRequest)
		return
	}
	writer.WriteHeader(http.StatusOK)
}

func (h *Handler) authors(ctx context.Context, writer http.ResponseWriter) {
	writeJSON(writer, http.StatusOK, h.authorList(ctx))
}

func decodeRows(request *http.Request) ([]sqlite.SystemConfig, error) {
	var inputs []struct {
		ID          int64   `json:"id"`
		Key         string  `json:"key"`
		Value       *string `json:"value"`
		ValidStatus *int64  `json:"validStatus"`
		CreateTime  *string `json:"createTime"`
		UpdateTime  *string `json:"updateTime"`
	}
	if err := decodeJSON(request, &inputs); err != nil {
		return nil, err
	}
	rows := make([]sqlite.SystemConfig, 0, len(inputs))
	for _, input := range inputs {
		if input.ValidStatus != nil && *input.ValidStatus != 0 && *input.ValidStatus != 1 {
			return nil, errors.New("invalid validStatus")
		}
		rows = append(rows, sqlite.SystemConfig{ID: sqlite.SystemConfigID(input.ID), Key: input.Key, Value: input.Value})
	}
	return rows, nil
}

func validateRows(rows []sqlite.SystemConfig) error {
	if len(rows) != len(configs) {
		return errors.New("config set incomplete")
	}
	seen := make(map[int64]bool, len(rows))
	for index := range rows {
		id := int64(rows[index].ID)
		expected, ok := configByID(id)
		if !ok || seen[id] || rows[index].Key != expected.Key || rows[index].Value == nil {
			return errors.New("invalid config identity")
		}
		value, err := validateValue(rows[index].Key, *rows[index].Value)
		if err != nil {
			return err
		}
		rows[index].Value = &value
		rows[index].ValidStatus = sqlite.Valid
		seen[id] = true
	}
	if err := transmissionconfig.ValidateCredentials(configValue(rows, "transmissionUsername"), configValue(rows, "transmissionPassword")); err != nil {
		return fmt.Errorf("invalid Transmission credentials: %w", err)
	}
	return nil
}

func configValue(rows []sqlite.SystemConfig, key string) string {
	for _, row := range rows {
		if row.Key == key && row.Value != nil {
			return *row.Value
		}
	}
	return ""
}

func decodeJSON(request *http.Request, target any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(nil, request.Body, maxBodyBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errors.New("expected one JSON value")
	}
	return nil
}

func statusFor(err error) int {
	if errors.Is(err, context.Canceled) {
		return http.StatusBadRequest
	}
	return http.StatusInternalServerError
}
func writeError(writer http.ResponseWriter, status int) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_, _ = writer.Write([]byte(`{"error":"request failed"}`))
}
func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}
