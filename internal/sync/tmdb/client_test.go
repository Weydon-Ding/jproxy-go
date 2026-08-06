package tmdb

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClient_Find_returnsFirstTVResultWithTMDBRequestContract(t *testing.T) {
	// Given
	secret := "client-secret"
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Path != "/3/find/101" || request.URL.Query().Get("api_key") != secret || request.URL.Query().Get("language") != "zh-CN" || request.URL.Query().Get("external_source") != "tvdb_id" {
			t.Fatalf("unexpected request: %s", request.URL.String())
		}
		_, _ = writer.Write([]byte(`{"tv_results":[{"id":12,"name":"第一项"},{"id":13,"name":"忽略"}]}`))
	}))
	t.Cleanup(server.Close)
	client := NewClient(server.Client(), time.Second)

	// When
	alias, found, err := client.Find(context.Background(), Config{BaseURL: server.URL, APIKey: secret}, 101, "zh-CN")

	// Then
	if err != nil || !found || alias.TVDBID != 101 || alias.TMDBID == nil || *alias.TMDBID != 12 || alias.Language != "zh-CN" || alias.Title != "第一项" {
		t.Fatalf("alias=%#v found=%t error=%v", alias, found, err)
	}
}

func TestClient_Find_rejectsUnsafeOrMalformedResponsesWithoutSecretLeakage(t *testing.T) {
	secret := "never-return-this-secret"
	oversized := `{"tv_results":[]}` + strings.Repeat(" ", maxResponseBytes)
	tests := []struct {
		name     string
		response func(http.ResponseWriter, *http.Request)
		want     string
	}{
		{name: "429", response: func(writer http.ResponseWriter, _ *http.Request) { writer.WriteHeader(http.StatusTooManyRequests) }, want: "status 429"},
		{name: "malformed", response: func(writer http.ResponseWriter, _ *http.Request) { _, _ = writer.Write([]byte(`{"tv_results":`)) }, want: "field json"},
		{name: "multiple JSON", response: func(writer http.ResponseWriter, _ *http.Request) { _, _ = writer.Write([]byte(`{"tv_results":[]} {}`)) }, want: "field json"},
		{name: "oversized", response: func(writer http.ResponseWriter, _ *http.Request) { _, _ = writer.Write([]byte(oversized)) }, want: "field body size"},
		{name: "invalid schema", response: func(writer http.ResponseWriter, _ *http.Request) {
			_, _ = writer.Write([]byte(`{"tv_results":[{"id":2147483648,"name":"bad"}]}`))
		}, want: "field tv_results"},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			// Given
			server := httptest.NewServer(http.HandlerFunc(testCase.response))
			t.Cleanup(server.Close)

			// When
			_, _, err := NewClient(server.Client(), time.Second).Find(context.Background(), Config{BaseURL: server.URL, APIKey: secret}, 1, "en")

			// Then
			if err == nil || !strings.Contains(err.Error(), testCase.want) || strings.Contains(err.Error(), secret) || strings.Contains(err.Error(), server.URL) {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

func TestClient_Find_preservesCancellationTimeoutAndRedirectRejection(t *testing.T) {
	// Given
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) { <-request.Context().Done() }))
	t.Cleanup(server.Close)
	client := NewClient(server.Client(), time.Nanosecond)
	redirect := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, "/other", http.StatusFound)
	}))
	t.Cleanup(redirect.Close)

	// When
	_, _, timeoutErr := client.Find(context.Background(), Config{BaseURL: server.URL, APIKey: "secret"}, 1, "en")
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, cancelErr := client.Find(cancelled, Config{BaseURL: server.URL, APIKey: "secret"}, 1, "en")
	_, _, redirectErr := NewClient(redirect.Client(), time.Second).Find(context.Background(), Config{BaseURL: redirect.URL, APIKey: "secret"}, 1, "en")

	// Then
	if !errors.Is(timeoutErr, context.DeadlineExceeded) || !errors.Is(cancelErr, context.Canceled) || !errors.Is(redirectErr, errRedirect) || strings.Contains(redirectErr.Error(), "secret") {
		t.Fatalf("timeout=%v cancellation=%v redirect=%v", timeoutErr, cancelErr, redirectErr)
	}
}
