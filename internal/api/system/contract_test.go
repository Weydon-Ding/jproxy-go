package system_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"jproxy-go/internal/api/system"
	"jproxy-go/internal/cache"
	"jproxy-go/internal/runtime"
)

func TestHandler_rejectsStrictJSONBoundaries(t *testing.T) {
	store := openSeededStore(t)
	provider := runtime.NewProvider(snapshot(t, store), store)
	handler := system.NewHandler(system.Options{Store: store, Provider: provider, Registry: runtime.NewRegistry(provider, cache.NewTTLCache[string](0, 1), cache.NewTTLCache[[]int](0, 1), cache.NewTTLCache[struct{}](0, 3))})
	payload, err := json.Marshal(completePayload(t, store))
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct{ name, body string }{
		{"empty", ""}, {"unknown_field", `[{}]`}, {"second_value", string(payload) + ` []`},
		{"non_string", strings.Replace(string(payload), `"value":"`, `"value":1,"ignored":"`, 1)},
		{"invalid_status", strings.Replace(string(payload), `"validStatus":1`, `"validStatus":2`, 1)},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/system/config/update", strings.NewReader(testCase.body))
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusBadRequest || response.Body.String() != `{"error":"request failed"}` || strings.Contains(response.Body.String(), "ignored") {
				t.Fatalf("status=%d body=%q", response.Code, response.Body.String())
			}
		})
	}
	oversized := bytes.Repeat([]byte("x"), 256*1024+1)
	request := httptest.NewRequest(http.MethodPost, "/api/system/config/update", bytes.NewReader(oversized))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("oversized status=%d", response.Code)
	}
}

func TestHandler_reportsMethodsWithAllowAndRedactedErrors(t *testing.T) {
	store := openSeededStore(t)
	provider := runtime.NewProvider(snapshot(t, store), store)
	handler := system.NewHandler(system.Options{Store: store, Provider: provider, Registry: runtime.NewRegistry(provider, cache.NewTTLCache[string](0, 1), cache.NewTTLCache[[]int](0, 1), cache.NewTTLCache[struct{}](0, 3))})
	request := httptest.NewRequest(http.MethodPost, "/api/system/config/version", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusMethodNotAllowed || response.Header().Get("Allow") != http.MethodGet || response.Body.String() != `{"error":"request failed"}` {
		t.Fatalf("status=%d allow=%q body=%q", response.Code, response.Header().Get("Allow"), response.Body.String())
	}
}

func TestHandler_returnsNotFoundForUnknownSystemPath(t *testing.T) {
	store := openSeededStore(t)
	provider := runtime.NewProvider(snapshot(t, store), store)
	handler := system.NewHandler(system.Options{Store: store, Provider: provider, Registry: runtime.NewRegistry(provider, cache.NewTTLCache[string](0, 1), cache.NewTTLCache[[]int](0, 1), cache.NewTTLCache[struct{}](0, 3))})
	request := httptest.NewRequest(http.MethodGet, "/api/system/config/unknown", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound || response.Header().Get("Allow") != "" {
		t.Fatalf("status=%d allow=%q", response.Code, response.Header().Get("Allow"))
	}
}
