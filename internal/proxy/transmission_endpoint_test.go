package proxy

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"jproxy-go/internal/runtime"
	"jproxy-go/internal/store/sqlite"
)

func TestTransmissionRoute_normalizesRuntimeEndpointForms(t *testing.T) {
	tests := []struct {
		name, suffix, wantPath string
	}{
		{name: "base", wantPath: "/transmission/rpc"},
		{name: "web", suffix: "/transmission/web", wantPath: "/transmission/rpc"},
		{name: "rpc", suffix: "/transmission/rpc", wantPath: "/transmission/rpc"},
		{name: "prefix", suffix: "/proxy", wantPath: "/proxy/transmission/rpc"},
		{name: "prefix web", suffix: "/proxy/transmission/web", wantPath: "/proxy/transmission/rpc"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given
			upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.URL.Path != test.wantPath || request.URL.RawQuery != "tag=one&x=%2F" {
					t.Fatalf("path=%q query=%q", request.URL.Path, request.URL.RawQuery)
				}
				_, _ = writer.Write([]byte("upstream"))
			}))
			defer upstream.Close()
			handler := NewServerWithRuntime(testConfig("http://127.0.0.1:1", "http://127.0.0.1:1"), RuntimeOptions{Provider: runtime.NewStaticProvider(sqlite.Snapshot{TransmissionURL: upstream.URL + test.suffix})}).Routes()

			// When
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/sonarr/transmission/transmission/rpc?tag=one&x=%2F", nil))

			// Then
			if recorder.Code != http.StatusOK || recorder.Body.String() != "upstream" {
				t.Fatalf("status=%d body=%q", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestTransmissionRoute_rejectsUnsafeRuntimeEndpointForms(t *testing.T) {
	unsafeEndpoints := []string{
		"ftp://example.test/transmission/rpc",
		"http://user:password@example.test/transmission/rpc",
		"http://example.test/transmission/rpc?",
		"http://example.test/transmission/rpc?query=value",
		"http://example.test/transmission/rpc#fragment",
		"http://example.test/proxy/../transmission/rpc",
		"http://example.test/proxy%2Fchild",
		"http://bad\n.test/transmission/rpc",
	}

	for _, endpoint := range unsafeEndpoints {
		t.Run(endpoint, func(t *testing.T) {
			// Given
			handler := NewServerWithRuntime(testConfig("http://127.0.0.1:1", "http://127.0.0.1:1"), RuntimeOptions{Provider: runtime.NewStaticProvider(sqlite.Snapshot{TransmissionURL: endpoint})}).Routes()

			// When
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/sonarr/transmission/transmission/rpc", nil))

			// Then
			if recorder.Code != http.StatusServiceUnavailable {
				t.Fatalf("status=%d", recorder.Code)
			}
		})
	}
}
