package system_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"jproxy-go/internal/api/system"
	"jproxy-go/internal/cache"
	"jproxy-go/internal/runtime"
	"jproxy-go/internal/store/sqlite"
)

func TestHandler_rejectsManagementURLQueriesWithoutPersistingOrPublishingCanaries(t *testing.T) {
	for _, test := range []struct{ name, value string }{
		{name: "raw query", value: "https://jackett.test/prefix?url-query-canary"},
		{name: "force query", value: "https://jackett.test/prefix?"},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := openSeededStore(t)
			provider := runtime.NewProvider(snapshot(t, store), store)
			registry := runtime.NewRegistry(provider, cache.NewTTLCache[string](0, 1), cache.NewTTLCache[[]int](0, 1), cache.NewTTLCache[struct{}](0, 3))
			server := httptest.NewServer(system.NewHandler(system.Options{Store: store, Provider: provider, Registry: registry}))
			t.Cleanup(server.Close)
			before := provider.Snapshot()
			payload := completePayload(t, store)
			for _, row := range payload {
				if row["key"] == "jackettUrl" {
					row["value"] = test.value
				}
			}
			body, err := json.Marshal(payload)
			if err != nil {
				t.Fatal(err)
			}

			response, err := http.Post(server.URL+"/api/system/config/update", "application/json", bytes.NewReader(body))
			if err != nil {
				t.Fatal(err)
			}
			responseBody, err := io.ReadAll(response.Body)
			if err != nil {
				t.Fatal(err)
			}
			if err := response.Body.Close(); err != nil {
				t.Fatal(err)
			}
			queryResponse, err := http.Get(server.URL + "/api/system/config/query")
			if err != nil {
				t.Fatal(err)
			}
			queryBody, err := io.ReadAll(queryResponse.Body)
			if err != nil {
				t.Fatal(err)
			}
			if err := queryResponse.Body.Close(); err != nil {
				t.Fatal(err)
			}

			after := provider.Snapshot()
			rows, err := store.Repositories().SystemConfigs.List(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			persisted := systemConfigValue(t, rows, "jackettUrl")
			if response.StatusCode != http.StatusBadRequest || len(responseBody) == 0 || bytes.Contains(responseBody, []byte("url-query-canary")) || queryResponse.StatusCode != http.StatusOK || bytes.Contains(queryBody, []byte("url-query-canary")) || after.JackettURL != before.JackettURL || after.RadarrRevision != before.RadarrRevision || persisted != before.JackettURL {
				t.Fatalf("update=%d/%q query=%d/%q before=%q after=%q persisted=%q", response.StatusCode, responseBody, queryResponse.StatusCode, queryBody, before.JackettURL, after.JackettURL, persisted)
			}
		})
	}
}

func TestHandler_restoresMaskedTransmissionPasswordBeforeValidationWithoutPersistingOrPublishingSentinel(t *testing.T) {
	store := openSeededStore(t)
	values := completeSystemConfigs(t, store)
	for index := range values {
		switch values[index].Key {
		case "transmissionUrl":
			value := "https://transmission.test"
			values[index].Value = &value
		case "transmissionUsername":
			value := "transmission-user"
			values[index].Value = &value
		case "transmissionPassword":
			value := "transmission-password"
			values[index].Value = &value
		}
	}
	initial, err := store.UpdateSystemConfigs(context.Background(), values)
	if err != nil {
		t.Fatal(err)
	}
	provider := runtime.NewProvider(initial, store)
	registry := runtime.NewRegistry(provider, cache.NewTTLCache[string](0, 1), cache.NewTTLCache[[]int](0, 1), cache.NewTTLCache[struct{}](0, 3))
	handler := system.NewHandler(system.Options{Store: store, Provider: provider, Registry: registry})
	payload := completePayload(t, store)
	for _, row := range payload {
		if row["key"] == "transmissionPassword" {
			row["value"] = "******"
		}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/system/config/update", bytes.NewReader(body)))

	rows, err := store.Repositories().SystemConfigs.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	persisted := systemConfigValue(t, rows, "transmissionPassword")
	after := provider.Snapshot()
	if response.Code != http.StatusOK || persisted != "transmission-password" || after.TransmissionPassword != "transmission-password" || persisted == "******" || after.TransmissionPassword == "******" {
		t.Fatalf("status=%d persisted=%q published=%q", response.Code, persisted, after.TransmissionPassword)
	}
}

func completeSystemConfigs(t *testing.T, store *sqlite.Store) []sqlite.SystemConfig {
	t.Helper()
	rows, err := store.Repositories().SystemConfigs.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

func systemConfigValue(t *testing.T, rows []sqlite.SystemConfig, key string) string {
	t.Helper()
	for _, row := range rows {
		if row.Key == key && row.Value != nil {
			return *row.Value
		}
	}
	t.Fatalf("missing system config %q", key)
	return ""
}
