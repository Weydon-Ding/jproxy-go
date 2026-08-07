package proxy

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

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

func TestTransmissionRoute_passesAllUpstreamStatuses(t *testing.T) {
	// Given / When / Then
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

func TestTransmissionRoute_preservesRawGzipResponse_withoutAcceptEncodingNegotiation(t *testing.T) {
	// Given
	defaultTransport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		t.Fatal("default transport is not *http.Transport")
	}
	if defaultTransport.DisableCompression {
		t.Fatal("default transport already disables compression")
	}
	var compressed bytes.Buffer
	gzipWriter := gzip.NewWriter(&compressed)
	_, _ = gzipWriter.Write([]byte("gzip-body"))
	_ = gzipWriter.Close()
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Accept-Encoding") != "" {
			t.Fatalf("unexpected accept-encoding: %q", request.Header.Get("Accept-Encoding"))
		}
		writer.Header().Set("Content-Encoding", "gzip")
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write(compressed.Bytes())
	}))
	defer upstream.Close()
	handler := NewServerWithRuntime(testConfig("http://127.0.0.1:1", "http://127.0.0.1:1"), RuntimeOptions{Provider: runtime.NewStaticProvider(sqlite.Snapshot{TransmissionURL: upstream.URL + "/transmission/rpc"})}).Routes()

	// When
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/sonarr/transmission/transmission/rpc", nil))

	// Then
	if recorder.Code != http.StatusOK || recorder.Header().Get("Content-Encoding") != "gzip" || !bytes.Equal(recorder.Body.Bytes(), compressed.Bytes()) {
		t.Fatalf("status=%d content-encoding=%q body=%q", recorder.Code, recorder.Header().Get("Content-Encoding"), recorder.Body.Bytes())
	}
	if defaultTransport.DisableCompression {
		t.Fatal("default transport was mutated")
	}
}

func TestTransmissionRoute_reusesConnection_whenRequestsAreSequential(t *testing.T) {
	// Given
	var connections atomic.Int32
	upstream := httptest.NewUnstartedServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte("upstream"))
	}))
	upstream.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateNew {
			connections.Add(1)
		}
	}
	upstream.Start()
	defer upstream.Close()
	server := NewServerWithRuntime(testConfig("http://127.0.0.1:1", "http://127.0.0.1:1"), RuntimeOptions{Provider: runtime.NewStaticProvider(sqlite.Snapshot{TransmissionURL: upstream.URL + "/transmission/rpc"})})
	handler := server.Routes()

	// When
	for range 2 {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/sonarr/transmission/transmission/rpc", nil))
		if recorder.Code != http.StatusOK || recorder.Body.String() != "upstream" {
			t.Fatalf("status=%d body=%q", recorder.Code, recorder.Body.String())
		}
	}

	// Then
	if connections.Load() != 1 {
		t.Fatalf("connections=%d", connections.Load())
	}
}

func TestServer_transmissionClient_returnsOneClient_whenFirstAccessIsConcurrent(t *testing.T) {
	// Given
	server := NewServer(testConfig("http://127.0.0.1:1", "http://127.0.0.1:1"))
	clients := make(chan *http.Client, 32)
	var group sync.WaitGroup

	// When
	for range cap(clients) {
		group.Add(1)
		go func() {
			defer group.Done()
			clients <- server.transmissionClient()
		}()
	}
	group.Wait()
	close(clients)

	// Then
	var first *http.Client
	for client := range clients {
		if first == nil {
			first = client
			continue
		}
		if client != first {
			t.Fatal("transmission client was initialized more than once")
		}
	}
}

func TestServer_transmissionClient_preservesCustomTransport_whenServerIsConstructedDirectly(t *testing.T) {
	// Given
	transport := &transmissionTestTransport{}
	server := &Server{
		cfg:    testConfig("http://127.0.0.1:1", "http://127.0.0.1:1"),
		client: &http.Client{Transport: transport},
	}

	// When
	client := server.transmissionClient()

	// Then
	if client.Transport != transport {
		t.Fatal("custom transport was replaced")
	}
}

type transmissionTestTransport struct{}

func (*transmissionTestTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("not called")
}

type transmissionTestProvider struct{ snapshot runtime.Snapshot }

func (p *transmissionTestProvider) Snapshot() runtime.Snapshot { return p.snapshot }

func (*transmissionTestProvider) Refresh(context.Context, runtime.Scope) error {
	return errors.New("not implemented")
}
