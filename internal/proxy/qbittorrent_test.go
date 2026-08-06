package proxy

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"jproxy-go/internal/runtime"
	"jproxy-go/internal/store/sqlite"
)

func TestQBittorrentRoute_proxiesBothPublicPrefixes_whenRequestIsSafe(t *testing.T) {
	// Given
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if r.URL.Path != "/api/v2/torrents/info" || r.URL.RawQuery != "filter=all&x=%2F" || r.Method != http.MethodPost || string(body) != "payload" || r.Header.Get("X-Custom") != "value" || r.Host == "client.example" {
			t.Fatalf("path=%q query=%q method=%q body=%q header=%q host=%q", r.URL.Path, r.URL.RawQuery, r.Method, body, r.Header.Get("X-Custom"), r.Host)
		}
		w.Header().Add("Set-Cookie", "one=1")
		w.Header().Add("Set-Cookie", "two=2")
		w.Header().Set("X-Upstream", "yes")
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte("upstream"))
	}))
	defer upstream.Close()
	provider := runtime.NewStaticProvider(sqlite.Snapshot{QBittorrentURL: upstream.URL})
	handler := NewServerWithRuntime(testConfig("http://127.0.0.1:1", "http://127.0.0.1:1"), RuntimeOptions{Provider: provider}).Routes()

	// When
	for _, path := range []string{"/sonarr/qbittorrent/api/v2/torrents/info?filter=all&x=%2F", "/radarr/qbittorrent/api/v2/torrents/info?filter=all&x=%2F"} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString("payload"))
		request.Host = "client.example"
		request.Header.Set("X-Custom", "value")
		handler.ServeHTTP(recorder, request)
		// Then
		if recorder.Code != http.StatusConflict || recorder.Body.String() != "upstream" || recorder.Header().Get("X-Upstream") != "yes" || len(recorder.Result().Cookies()) != 2 {
			t.Fatalf("path=%s status=%d body=%q headers=%v", path, recorder.Code, recorder.Body.String(), recorder.Header())
		}
	}
}

func TestQBittorrentRoute_returnsControlledErrors_whenUnsafeOrUnavailable(t *testing.T) {
	// Given
	empty := NewServerWithRuntime(testConfig("http://127.0.0.1:1", "http://127.0.0.1:1"), RuntimeOptions{Provider: runtime.NewStaticProvider(sqlite.Snapshot{})}).Routes()
	configured := NewServerWithRuntime(testConfig("http://127.0.0.1:1", "http://127.0.0.1:1"), RuntimeOptions{Provider: runtime.NewStaticProvider(sqlite.Snapshot{QBittorrentURL: "http://127.0.0.1:1"})}).Routes()

	// When / Then
	for _, test := range []struct {
		method, path string
		handler      http.Handler
		want         int
	}{
		{http.MethodGet, "/sonarr/qbittorrent/api/v2/torrents/info", empty, http.StatusServiceUnavailable}, {http.MethodPut, "/sonarr/qbittorrent/api/v2/torrents/info", configured, http.StatusMethodNotAllowed}, {http.MethodGet, "/sonarr/qbittorrent/%2e%2e/api/v2/torrents/info", configured, http.StatusBadRequest}, {http.MethodGet, "/sonarr/qbittorrent/api/v2/../torrents/info", configured, http.StatusBadRequest},
	} {
		recorder := httptest.NewRecorder()
		test.handler.ServeHTTP(recorder, httptest.NewRequest(test.method, test.path, nil))
		if recorder.Code != test.want || bytes.Contains(recorder.Body.Bytes(), []byte("127.0.0.1")) {
			t.Fatalf("%s %s status=%d body=%q", test.method, test.path, recorder.Code, recorder.Body.String())
		}
	}
}

func TestQBittorrentRoute_rejectsInvalidRuntimeURLsAndPassesAllUpstreamStatuses(t *testing.T) {
	// Given / When / Then
	for _, value := range []string{"ftp://example.test", "http://user:password@example.test", "http://example.test?q=x", "http://example.test#x", "http://bad\n.test"} {
		handler := NewServerWithRuntime(testConfig("http://127.0.0.1:1", "http://127.0.0.1:1"), RuntimeOptions{Provider: runtime.NewStaticProvider(sqlite.Snapshot{QBittorrentURL: value})}).Routes()
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/sonarr/qbittorrent/api/v2/torrents/info", nil))
		if recorder.Code != http.StatusServiceUnavailable {
			t.Fatalf("url=%q status=%d", value, recorder.Code)
		}
	}
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusConflict, http.StatusInternalServerError} {
		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(status); _, _ = w.Write([]byte("canary")) }))
		handler := NewServerWithRuntime(testConfig("http://127.0.0.1:1", "http://127.0.0.1:1"), RuntimeOptions{Provider: runtime.NewStaticProvider(sqlite.Snapshot{QBittorrentURL: upstream.URL})}).Routes()
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/radarr/qbittorrent/api/v2/torrents/info", nil))
		upstream.Close()
		if recorder.Code != status || recorder.Body.String() != "canary" {
			t.Fatalf("status=%d got=%d body=%q", status, recorder.Code, recorder.Body.String())
		}
	}
}
