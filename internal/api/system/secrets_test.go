package system_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"jproxy-go/internal/api/system"
	"jproxy-go/internal/cache"
	"jproxy-go/internal/runtime"
	"jproxy-go/internal/store/sqlite"
)

func TestHandler_queryMasksSensitiveConfigValues(t *testing.T) {
	// Given
	store := openSeededStore(t)
	secrets := secretCanaries()
	secrets["transmissionUsername"] = "transmission-user"
	setConfigValues(t, store, secrets)
	provider := runtime.NewProvider(snapshot(t, store), store)
	handler := system.NewHandler(system.Options{Store: store, Provider: provider, Registry: newRegistry(provider)})

	// When
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/system/config/query", nil))

	// Then
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%q", response.Code, response.Body.String())
	}
	for _, canary := range secretCanaries() {
		if bytes.Contains(response.Body.Bytes(), []byte(canary)) {
			t.Fatalf("query leaked canary %q", canary)
		}
	}
	var rows []map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &rows); err != nil {
		t.Fatalf("decode query: %v", err)
	}
	if len(rows) != 20 || rows[1]["id"] != float64(2) || rows[1]["key"] != "sonarrApikey" || rows[1]["value"] != "******" {
		t.Fatalf("query rows=%v", rows)
	}
	if configValues(t, store)["sonarrApikey"] != secrets["sonarrApikey"] {
		t.Fatalf("query changed persisted secret")
	}
}

func TestHandler_updatePreservesMaskedSensitiveValuesAndPublishesRealSnapshot(t *testing.T) {
	// Given
	store := openSeededStore(t)
	secrets := secretCanaries()
	secrets["transmissionUsername"] = "transmission-user"
	setConfigValues(t, store, secrets)
	provider := runtime.NewProvider(snapshot(t, store), store)
	handler := system.NewHandler(system.Options{Store: store, Provider: provider, Registry: newRegistry(provider)})
	queryResponse := httptest.NewRecorder()
	handler.ServeHTTP(queryResponse, httptest.NewRequest(http.MethodGet, "/api/system/config/query", nil))
	var payload []map[string]any
	if err := json.Unmarshal(queryResponse.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode query: %v", err)
	}
	for _, row := range payload {
		if row["key"] == "ruleSyncAuthors" {
			row["value"] = "ALL,team"
		}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal update payload: %v", err)
	}

	// When
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/system/config/update", bytes.NewReader(body)))

	// Then
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%q", response.Code, response.Body.String())
	}
	values := configValues(t, store)
	for key, want := range secrets {
		if values[key] != want {
			t.Fatalf("stored %s=%q want %q", key, values[key], want)
		}
	}
	if values["ruleSyncAuthors"] != "ALL,team" || provider.Snapshot().QBittorrentPassword != secrets["qbittorrentPassword"] || provider.Snapshot().TransmissionPassword != secrets["transmissionPassword"] {
		t.Fatalf("stored authors=%q runtime passwords=%q/%q", values["ruleSyncAuthors"], provider.Snapshot().QBittorrentPassword, provider.Snapshot().TransmissionPassword)
	}
}

func TestHandler_updateReplacesExplicitSensitiveValueAndKeepsQueryMasked(t *testing.T) {
	// Given
	store := openSeededStore(t)
	setConfigValues(t, store, map[string]string{"qbittorrentPassword": "old-password-canary"})
	provider := runtime.NewProvider(snapshot(t, store), store)
	handler := system.NewHandler(system.Options{Store: store, Provider: provider, Registry: newRegistry(provider)})
	payload := completePayload(t, store)
	for _, row := range payload {
		if row["key"] == "qbittorrentPassword" {
			row["value"] = "new-password-canary"
		}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal update payload: %v", err)
	}

	// When
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/system/config/update", bytes.NewReader(body)))
	queryResponse := httptest.NewRecorder()
	handler.ServeHTTP(queryResponse, httptest.NewRequest(http.MethodGet, "/api/system/config/query", nil))

	// Then
	if response.Code != http.StatusOK || configValues(t, store)["qbittorrentPassword"] != "new-password-canary" || provider.Snapshot().QBittorrentPassword != "new-password-canary" {
		t.Fatalf("status=%d stored=%q runtime=%q", response.Code, configValues(t, store)["qbittorrentPassword"], provider.Snapshot().QBittorrentPassword)
	}
	if bytes.Contains(queryResponse.Body.Bytes(), []byte("new-password-canary")) || !bytes.Contains(queryResponse.Body.Bytes(), []byte(`"value":"******"`)) {
		t.Fatalf("query body=%q", queryResponse.Body.String())
	}
}

func TestHandler_updateTreatsSentinelAsLiteralForNonSensitiveValue(t *testing.T) {
	// Given
	store := openSeededStore(t)
	provider := runtime.NewProvider(snapshot(t, store), store)
	handler := system.NewHandler(system.Options{Store: store, Provider: provider, Registry: newRegistry(provider)})
	payload := completePayload(t, store)
	for _, row := range payload {
		if row["key"] == "ruleSyncAuthors" {
			row["value"] = "******"
		}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal update payload: %v", err)
	}

	// When
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/system/config/update", bytes.NewReader(body)))

	// Then
	if response.Code != http.StatusOK || configValues(t, store)["ruleSyncAuthors"] != "******" {
		t.Fatalf("status=%d stored=%q", response.Code, configValues(t, store)["ruleSyncAuthors"])
	}
}

func TestHandler_updateAllowsEmptyExplicitSensitiveValue(t *testing.T) {
	// Given
	store := openSeededStore(t)
	setConfigValues(t, store, map[string]string{"sonarrApikey": "old-api-key-canary"})
	provider := runtime.NewProvider(snapshot(t, store), store)
	handler := system.NewHandler(system.Options{Store: store, Provider: provider, Registry: newRegistry(provider)})
	payload := completePayload(t, store)
	for _, row := range payload {
		if row["key"] == "sonarrApikey" {
			row["value"] = ""
		}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal update payload: %v", err)
	}

	// When
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/system/config/update", bytes.NewReader(body)))

	// Then
	if response.Code != http.StatusOK || configValues(t, store)["sonarrApikey"] != "" {
		t.Fatalf("status=%d stored=%q", response.Code, configValues(t, store)["sonarrApikey"])
	}
}

func TestHandler_updateFailureDoesNotExposeSensitiveValue(t *testing.T) {
	// Given
	store := openSeededStore(t)
	provider := runtime.NewProvider(snapshot(t, store), store)
	handler := system.NewHandler(system.Options{Store: store, Provider: provider, Registry: newRegistry(provider)})
	payload := completePayload(t, store)
	for _, row := range payload {
		switch row["key"] {
		case "sonarrApikey":
			row["value"] = "secret-error-canary"
		case "sonarrIndexerFormat":
			row["value"] = "invalid"
		}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal update payload: %v", err)
	}

	// When
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/system/config/update", bytes.NewReader(body)))

	// Then
	if response.Code != http.StatusBadRequest || bytes.Contains(response.Body.Bytes(), []byte("secret-error-canary")) {
		t.Fatalf("status=%d body=%q", response.Code, response.Body.String())
	}
}

func secretCanaries() map[string]string {
	return map[string]string{
		"sonarrApikey":         "sonarr-api-key-canary",
		"radarrApikey":         "radarr-api-key-canary",
		"tmdbApikey":           "tmdb-api-key-canary",
		"qbittorrentPassword":  "qbittorrent-password-canary",
		"transmissionPassword": "transmission-password-canary",
	}
}

func newRegistry(provider runtime.Provider) *runtime.Registry {
	return runtime.NewRegistry(provider, cache.NewTTLCache[string](0, 1), cache.NewTTLCache[[]int](0, 1), cache.NewTTLCache[struct{}](0, 3))
}

func setConfigValues(t *testing.T, store *sqlite.Store, replacements map[string]string) {
	t.Helper()
	rows, err := store.Repositories().SystemConfigs.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for index := range rows {
		if value, ok := replacements[rows[index].Key]; ok {
			rows[index].Value = &value
		}
	}
	if err := store.Repositories().SystemConfigs.UpsertBatch(context.Background(), rows); err != nil {
		t.Fatal(err)
	}
}

func configValues(t *testing.T, store *sqlite.Store) map[string]string {
	t.Helper()
	rows, err := store.Repositories().SystemConfigs.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	values := make(map[string]string, len(rows))
	for _, row := range rows {
		values[row.Key] = *row.Value
	}
	return values
}
