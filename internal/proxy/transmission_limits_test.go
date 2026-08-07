package proxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"jproxy-go/internal/runtime"
	"jproxy-go/internal/store/sqlite"
)

func TestTransmissionRoute_returns413_whenRequestExceedsLimit(t *testing.T) {
	// Given
	handler := transmissionHandler(t, "http://127.0.0.1:1/transmission/rpc")

	// When
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/sonarr/transmission/transmission/rpc", strings.NewReader(strings.Repeat("x", transmissionProxyLimit+1))))

	// Then
	assertTransmissionError(t, recorder, http.StatusRequestEntityTooLarge)
}

func TestTransmissionRoute_returns502_whenUpstreamRedirects(t *testing.T) {
	// Given
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Location", "/other")
		writer.WriteHeader(http.StatusFound)
	}))
	defer upstream.Close()
	handler := transmissionHandler(t, upstream.URL+"/transmission/rpc")

	// When
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/sonarr/transmission/transmission/rpc", nil))

	// Then
	assertTransmissionError(t, recorder, http.StatusBadGateway)
}

func TestTransmissionRoute_returns504_whenUpstreamContextExpires(t *testing.T) {
	// Given
	cfg := testConfig("http://127.0.0.1:1", "http://127.0.0.1:1")
	cfg.HTTPTimeout = 20 * time.Millisecond
	server := NewServerWithRuntime(cfg, RuntimeOptions{Provider: runtime.NewStaticProvider(sqlite.Snapshot{TransmissionURL: "http://upstream.test/transmission/rpc"})})
	server.client = &http.Client{Transport: roundTripperFunc(func(request *http.Request) (*http.Response, error) {
		<-request.Context().Done()
		return nil, request.Context().Err()
	})}

	// When
	recorder := httptest.NewRecorder()
	server.Routes().ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/sonarr/transmission/transmission/rpc", nil))

	// Then
	assertTransmissionError(t, recorder, http.StatusGatewayTimeout)
}

func TestTransmissionRoute_returns502_whenResponseExceedsLimit(t *testing.T) {
	// Given
	server := NewServerWithRuntime(testConfig("http://127.0.0.1:1", "http://127.0.0.1:1"), RuntimeOptions{Provider: runtime.NewStaticProvider(sqlite.Snapshot{TransmissionURL: "http://upstream.test/transmission/rpc"})})
	server.client = &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(strings.Repeat("x", transmissionProxyLimit+1))), Header: make(http.Header)}, nil
	})}

	// When
	recorder := httptest.NewRecorder()
	server.Routes().ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/sonarr/transmission/transmission/rpc", nil))

	// Then
	assertTransmissionError(t, recorder, http.StatusBadGateway)
}

func transmissionHandler(t *testing.T, endpoint string) http.Handler {
	t.Helper()
	return NewServerWithRuntime(testConfig("http://127.0.0.1:1", "http://127.0.0.1:1"), RuntimeOptions{Provider: runtime.NewStaticProvider(sqlite.Snapshot{TransmissionURL: endpoint})}).Routes()
}

func assertTransmissionError(t *testing.T, recorder *httptest.ResponseRecorder, status int) {
	t.Helper()
	if recorder.Code != status || recorder.Body.String() != `{"error":"request failed"}` {
		t.Fatalf("status=%d body=%q", recorder.Code, recorder.Body.String())
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (roundTrip roundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return roundTrip(request)
}

var _ http.RoundTripper = roundTripperFunc(nil)
