package proxy

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"jproxy-go/internal/runtime"
	"jproxy-go/internal/store/sqlite"
)

func TestTransmissionRoute_proxiesBothPublicPrefixes_withoutInjectingRuntimeCredentials(t *testing.T) {
	// Given
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		if request.URL.Path != "/transmission/rpc" || request.URL.RawQuery != "tag=one&x=%2F" || request.Method != http.MethodPost || string(body) != "payload" || request.Header.Get("Authorization") != "Bearer caller" || request.Header.Get("X-Transmission-Session-Id") != "caller-session" || request.Header.Get("X-Custom") != "value" || request.Host == "client.example" {
			t.Fatal("request was not transparently forwarded")
		}
		if request.Header.Get("Connection") != "" || request.Header.Get("Proxy-Connection") != "" || request.Header.Get("X-Remove") != "" {
			t.Fatal("hop-by-hop request header was forwarded")
		}
		writer.Header().Add("Set-Cookie", "one=1")
		writer.Header().Add("Set-Cookie", "two=2")
		writer.Header().Set("X-Transmission-Session-Id", "upstream-session")
		writer.Header().Set("Connection", "X-Remove")
		writer.Header().Set("Proxy-Connection", "keep-alive")
		writer.Header().Set("X-Remove", "secret-header")
		writer.WriteHeader(http.StatusConflict)
		_, _ = writer.Write([]byte("upstream"))
	}))
	defer upstream.Close()
	provider := runtime.NewStaticProvider(sqlite.Snapshot{TransmissionURL: upstream.URL + "/transmission/rpc", TransmissionUsername: "runtime-user", TransmissionPassword: "runtime-password"})
	handler := NewServerWithRuntime(testConfig("http://127.0.0.1:1", "http://127.0.0.1:1"), RuntimeOptions{Provider: provider}).Routes()

	// When
	for _, path := range []string{"/sonarr/transmission/transmission/rpc?tag=one&x=%2F", "/radarr/transmission/transmission/rpc/?tag=one&x=%2F"} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString("payload"))
		request.Host = "client.example"
		request.Header.Set("Authorization", "Bearer caller")
		request.Header.Set("X-Transmission-Session-Id", "caller-session")
		request.Header.Set("X-Custom", "value")
		request.Header.Set("Connection", "X-Remove")
		request.Header.Set("Proxy-Connection", "keep-alive")
		request.Header.Set("X-Remove", "secret-header")
		handler.ServeHTTP(recorder, request)
		// Then
		if recorder.Code != http.StatusConflict || recorder.Body.String() != "upstream" || recorder.Header().Get("X-Transmission-Session-Id") != "upstream-session" || len(recorder.Result().Cookies()) != 2 || recorder.Header().Get("Connection") != "" || recorder.Header().Get("Proxy-Connection") != "" || recorder.Header().Get("X-Remove") != "" {
			t.Fatalf("path=%s status=%d cookies=%d", path, recorder.Code, len(recorder.Result().Cookies()))
		}
	}
}

func TestTransmissionRoute_preserves409WithoutRetry_andDoesNotInjectHeaders_whenCallerOmitsThem(t *testing.T) {
	// Given
	calls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		calls++
		if request.Header.Get("Authorization") != "" || request.Header.Get("X-Transmission-Session-Id") != "" {
			t.Fatal("proxy injected authentication or session header")
		}
		writer.Header().Set("X-Transmission-Session-Id", "challenge")
		writer.WriteHeader(http.StatusConflict)
		_, _ = writer.Write([]byte("retry yourself"))
	}))
	defer upstream.Close()
	handler := NewServerWithRuntime(testConfig("http://127.0.0.1:1", "http://127.0.0.1:1"), RuntimeOptions{Provider: runtime.NewStaticProvider(sqlite.Snapshot{TransmissionURL: upstream.URL + "/transmission/rpc", TransmissionUsername: "runtime-user", TransmissionPassword: "runtime-password"})}).Routes()

	// When
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/sonarr/transmission/transmission/rpc", nil))

	// Then
	if recorder.Code != http.StatusConflict || recorder.Body.String() != "retry yourself" || recorder.Header().Get("X-Transmission-Session-Id") != "challenge" || calls != 1 {
		t.Fatalf("status=%d calls=%d", recorder.Code, calls)
	}
}

func TestTransmissionRoute_passesAllUpstreamStatusesAndRejectsInvalidRuntimeURLs(t *testing.T) {
	// Given / When / Then
	for _, value := range []string{"ftp://example.test/transmission/rpc", "http://user:password@example.test/transmission/rpc", "http://example.test/transmission/rpc?", "http://example.test/transmission/rpc?q=x", "http://example.test/transmission/rpc#x", "http://example.test/other", "http://bad\n.test/transmission/rpc"} {
		handler := NewServerWithRuntime(testConfig("http://127.0.0.1:1", "http://127.0.0.1:1"), RuntimeOptions{Provider: runtime.NewStaticProvider(sqlite.Snapshot{TransmissionURL: value})}).Routes()
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/sonarr/transmission/transmission/rpc", nil))
		if recorder.Code != http.StatusServiceUnavailable {
			t.Fatalf("invalid runtime URL status=%d", recorder.Code)
		}
	}
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusInternalServerError} {
		upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			writer.Header().Set("X-Status", "preserved")
			writer.WriteHeader(status)
			_, _ = writer.Write([]byte("upstream status body"))
		}))
		handler := NewServerWithRuntime(testConfig("http://127.0.0.1:1", "http://127.0.0.1:1"), RuntimeOptions{Provider: runtime.NewStaticProvider(sqlite.Snapshot{TransmissionURL: upstream.URL + "/transmission/rpc"})}).Routes()
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/radarr/transmission/transmission/rpc", nil))
		upstream.Close()
		if recorder.Code != status || recorder.Body.String() != "upstream status body" || recorder.Header().Get("X-Status") != "preserved" {
			t.Fatalf("expected status=%d got=%d", status, recorder.Code)
		}
	}
}

func TestTransmissionRoute_usesCurrentRuntimeEndpoint_whenConfigurationChanges(t *testing.T) {
	// Given
	first := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) { _, _ = writer.Write([]byte("first")) }))
	defer first.Close()
	second := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) { _, _ = writer.Write([]byte("second")) }))
	defer second.Close()
	provider := &transmissionTestProvider{snapshot: runtime.Snapshot{TransmissionURL: first.URL + "/transmission/rpc"}}
	handler := NewServerWithRuntime(testConfig("http://127.0.0.1:1", "http://127.0.0.1:1"), RuntimeOptions{Provider: provider}).Routes()

	// When
	firstResponse := httptest.NewRecorder()
	handler.ServeHTTP(firstResponse, httptest.NewRequest(http.MethodPost, "/sonarr/transmission/transmission/rpc", nil))
	provider.snapshot.TransmissionURL = second.URL + "/transmission/rpc"
	secondResponse := httptest.NewRecorder()
	handler.ServeHTTP(secondResponse, httptest.NewRequest(http.MethodPost, "/sonarr/transmission/transmission/rpc", nil))

	// Then
	if firstResponse.Code != http.StatusOK || firstResponse.Body.String() != "first" || secondResponse.Code != http.StatusOK || secondResponse.Body.String() != "second" {
		t.Fatalf("first=%d/%q second=%d/%q", firstResponse.Code, firstResponse.Body.String(), secondResponse.Code, secondResponse.Body.String())
	}
}

func TestTransmissionRoute_returnsControlledErrors_whenInvalidOrUnsafe(t *testing.T) {
	// Given
	empty := NewServerWithRuntime(testConfig("http://127.0.0.1:1", "http://127.0.0.1:1"), RuntimeOptions{Provider: runtime.NewStaticProvider(sqlite.Snapshot{})}).Routes()
	configured := NewServerWithRuntime(testConfig("http://127.0.0.1:1", "http://127.0.0.1:1"), RuntimeOptions{Provider: runtime.NewStaticProvider(sqlite.Snapshot{TransmissionURL: "http://127.0.0.1:1/transmission/rpc"})}).Routes()

	// When / Then
	for _, test := range []struct {
		method, path string
		handler      http.Handler
		want         int
	}{
		{http.MethodGet, "/sonarr/transmission/transmission/rpc", empty, http.StatusServiceUnavailable},
		{http.MethodPut, "/sonarr/transmission/transmission/rpc", configured, http.StatusMethodNotAllowed},
		{http.MethodGet, "/sonarr/transmission/other", configured, http.StatusBadRequest},
		{http.MethodGet, "/sonarr/transmission/%2e%2e/transmission/rpc", configured, http.StatusBadRequest},
		{http.MethodGet, "/radarr/transmission/transmission/rpc%2f..", configured, http.StatusBadRequest},
		{http.MethodGet, "/radarr/transmission/transmission\\rpc", configured, http.StatusBadRequest},
	} {
		recorder := httptest.NewRecorder()
		test.handler.ServeHTTP(recorder, httptest.NewRequest(test.method, test.path, nil))
		if recorder.Code != test.want || recorder.Body.String() != `{"error":"request failed"}` || (test.want == http.StatusMethodNotAllowed && recorder.Header().Get("Allow") != "GET, POST") {
			t.Fatalf("%s %s status=%d", test.method, test.path, recorder.Code)
		}
	}
}

func TestTransmissionRoute_limitsBodiesAndMapsRedirectAndTimeout(t *testing.T) {
	// Given
	redirect := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Location", "/other")
		writer.WriteHeader(http.StatusFound)
	}))
	defer redirect.Close()
	timeout := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) { <-request.Context().Done() }))
	defer timeout.Close()
	oversized := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(strings.Repeat("x", transmissionProxyLimit+1)))
	}))
	defer oversized.Close()

	// When / Then
	for _, test := range []struct {
		name, url, body string
		want            int
	}{
		{"request limit", "http://127.0.0.1:1/transmission/rpc", strings.Repeat("x", (8<<20)+1), http.StatusRequestEntityTooLarge},
		{"redirect", redirect.URL + "/transmission/rpc", "", http.StatusBadGateway},
		{"timeout", timeout.URL + "/transmission/rpc", "", http.StatusGatewayTimeout},
		{"response limit", oversized.URL + "/transmission/rpc", "", http.StatusBadGateway},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg := testConfig("http://127.0.0.1:1", "http://127.0.0.1:1")
			cfg.HTTPTimeout = 20 * time.Millisecond
			handler := NewServerWithRuntime(cfg, RuntimeOptions{Provider: runtime.NewStaticProvider(sqlite.Snapshot{TransmissionURL: test.url})}).Routes()
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/sonarr/transmission/transmission/rpc", strings.NewReader(test.body)))
			if recorder.Code != test.want || recorder.Body.String() != `{"error":"request failed"}` {
				t.Fatalf("status=%d body=%q", recorder.Code, recorder.Body.String())
			}
		})
	}
}

type transmissionTestProvider struct{ snapshot runtime.Snapshot }

func (p *transmissionTestProvider) Snapshot() runtime.Snapshot { return p.snapshot }

func (*transmissionTestProvider) Refresh(context.Context, runtime.Scope) error {
	return errors.New("not implemented")
}
