package system_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"jproxy-go/internal/api/system"
	"jproxy-go/internal/cache"
	"jproxy-go/internal/runtime"
)

func TestHandler_rejectsIncompleteTransmissionCredentialsWithoutPublishing(t *testing.T) {
	// Given
	store := openSeededStore(t)
	provider := runtime.NewProvider(snapshot(t, store), store)
	registry := runtime.NewRegistry(provider, cache.NewTTLCache[string](0, 1), cache.NewTTLCache[[]int](0, 1), cache.NewTTLCache[struct{}](0, 3))
	handler := system.NewHandler(system.Options{Store: store, Provider: provider, Registry: registry})
	before := provider.Snapshot()
	payload := completePayload(t, store)
	for _, row := range payload {
		if row["key"] == "transmissionUsername" {
			row["value"] = "user-canary"
		}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}

	// When
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/system/config/update", bytes.NewReader(body)))

	// Then
	after := provider.Snapshot()
	if response.Code != http.StatusBadRequest || after.TransmissionRevision != before.TransmissionRevision || after.TransmissionUsername != before.TransmissionUsername || after.TransmissionPassword != before.TransmissionPassword {
		t.Fatalf("status=%d transmission=%q/%q revision=%d", response.Code, after.TransmissionUsername, after.TransmissionPassword, after.TransmissionRevision)
	}
}
