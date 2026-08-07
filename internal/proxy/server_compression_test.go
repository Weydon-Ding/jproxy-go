package proxy

import (
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIndexerRoute_usesDefaultCompressionBehavior_whenUpstreamSendsGzip(t *testing.T) {
	// Given
	var compressed bytes.Buffer
	gzipWriter := gzip.NewWriter(&compressed)
	_, _ = gzipWriter.Write([]byte(rssWithItems(item("gzip"))))
	_ = gzipWriter.Close()
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Accept-Encoding") != "gzip" {
			t.Fatalf("accept-encoding=%q, want gzip", request.Header.Get("Accept-Encoding"))
		}
		writer.Header().Set("Content-Encoding", "gzip")
		_, _ = writer.Write(compressed.Bytes())
	}))
	defer upstream.Close()
	handler := NewServer(testConfig(upstream.URL, upstream.URL)).Routes()

	// When
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/sonarr/jackett/api?t=search", nil))

	// Then
	if recorder.Code != http.StatusOK || recorder.Header().Get("Content-Encoding") != "" || recorder.Body.String() != rssWithItems(item("gzip")) {
		t.Fatalf("status=%d content-encoding=%q body=%q", recorder.Code, recorder.Header().Get("Content-Encoding"), recorder.Body.String())
	}
}
