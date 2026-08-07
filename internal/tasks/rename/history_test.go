package rename

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestHistoryClient_Fetch_usesExactQueryAndParsesEvents(t *testing.T) {
	// Given
	since := time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/v3/history/since" || request.URL.RawQuery != "apikey=secret&date=2026-08-07T12%3A00%3A00.000Z&eventType=1" {
			t.Fatalf("request = %s", request.URL.String())
		}
		_, _ = writer.Write([]byte(`[{"sourceTitle":"Release","downloadId":"ABC","data":{"downloadClient":"Transmission"}}]`))
	}))
	defer server.Close()
	client, err := NewHistoryClient(HistoryClientOptions{HTTPClient: server.Client(), Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}

	// When
	events, err := client.Fetch(context.Background(), server.URL, "secret", since)

	// Then
	if err != nil || !reflect.DeepEqual(events, []Event{{SourceTitle: "Release", Hash: "abc", Downloader: DownloaderTransmission}}) {
		t.Fatalf("Fetch() = %#v, %v", events, err)
	}
}

func TestHistoryClient_Fetch_defaultsUnknownDownloaderToQBittorrent(t *testing.T) {
	// Given
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`[{"sourceTitle":"Release","downloadId":"ABC","data":{"downloadClient":"unsupported"}}]`))
	}))
	defer server.Close()
	client, err := NewHistoryClient(HistoryClientOptions{HTTPClient: server.Client(), Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}

	// When
	events, err := client.Fetch(context.Background(), server.URL, "secret", time.Unix(0, 0))

	// Then
	if err != nil || !reflect.DeepEqual(events, []Event{{SourceTitle: "Release", Hash: "abc", Downloader: DownloaderQBittorrent}}) {
		t.Fatalf("Fetch() = %#v, %v", events, err)
	}
}

func TestHistoryClient_Fetch_rejectsUnsafeUpstreamsWithoutLeakingAPIKey(t *testing.T) {
	// Given
	client, err := NewHistoryClient(HistoryClientOptions{Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	cases := []string{"https://sonarr.test?other=value", "https://sonarr.test#fragment", "https://user:pass@sonarr.test"}

	for _, endpoint := range cases {
		// When
		_, fetchErr := client.Fetch(context.Background(), endpoint, "api-secret", time.Unix(0, 0))

		// Then
		if !errors.Is(fetchErr, ErrInvalidEndpoint) || strings.Contains(fetchErr.Error(), "api-secret") {
			t.Fatalf("Fetch() error = %v", fetchErr)
		}
	}
}

func TestHistoryClient_Fetch_rejectsRedirectStatusAndMalformedBody(t *testing.T) {
	// Given
	cases := []struct {
		status  int
		body    string
		wantErr error
	}{{http.StatusFound, "", ErrHistoryRedirect}, {http.StatusBadGateway, "", ErrHistoryStatus}, {http.StatusOK, "{", ErrHistoryBody}}
	for _, test := range cases {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			writer.WriteHeader(test.status)
			_, _ = writer.Write([]byte(test.body))
		}))
		client, err := NewHistoryClient(HistoryClientOptions{HTTPClient: server.Client(), Timeout: time.Second})
		if err != nil {
			t.Fatal(err)
		}

		// When
		_, fetchErr := client.Fetch(context.Background(), server.URL, "key", time.Unix(0, 0))
		server.Close()

		// Then
		if !errors.Is(fetchErr, test.wantErr) {
			t.Fatalf("Fetch() error = %v", fetchErr)
		}
	}
}
