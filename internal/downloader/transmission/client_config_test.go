package transmission

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"jproxy-go/internal/runtime"
)

func TestClient_sendsJavaCompatibleIncompleteCredentials(t *testing.T) {
	for _, credentials := range [][2]string{{"user", ""}, {"", "password"}} {
		t.Run(credentials[0]+credentials[1], func(t *testing.T) {
			// Given
			client, err := New(Options{Provider: &mutableProvider{snapshot: runtime.Snapshot{TransmissionURL: "http://example.test", TransmissionUsername: credentials[0], TransmissionPassword: credentials[1]}}, HTTPClient: &http.Client{Transport: roundTripperFunc(func(request *http.Request) (*http.Response, error) {
				username, password, ok := request.BasicAuth()
				if !ok || username != credentials[0] || password != credentials[1] {
					t.Fatal("transport received unexpected credentials")
				}
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"result":"success","arguments":{"torrents":[{"id":7,"name":"root","files":[]}]}}`)), Header: make(http.Header)}, nil
			})}, Timeout: time.Second})
			if err != nil {
				t.Fatal(err)
			}

			// When
			_, err = client.Files(context.Background(), "hash")

			// Then
			if err != nil {
				t.Fatalf("err=%v", err)
			}
		})
	}
}

func TestClient_usesAnonymousRequests_whenCredentialsAreEmpty(t *testing.T) {
	// Given
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if _, _, ok := request.BasicAuth(); ok {
			t.Fatal("anonymous request included Authorization")
		}
		_, _ = io.WriteString(w, `{"result":"success","arguments":{"torrents":[{"id":7,"name":"root","files":[]}]}}`)
	}))
	defer upstream.Close()
	client := testClient(t, &mutableProvider{snapshot: runtime.Snapshot{TransmissionURL: upstream.URL}})

	// When
	_, err := client.Files(context.Background(), "hash")

	// Then
	if err != nil {
		t.Fatalf("Files() error=%v", err)
	}
}

func TestClient_rejectsInvalidConfigurationAndRedirects(t *testing.T) {
	for _, snapshot := range []runtime.Snapshot{
		{TransmissionURL: "http://user:password@example.test", TransmissionUsername: "user", TransmissionPassword: "password"},
		{TransmissionURL: "http://host/transmission/rpc?", TransmissionUsername: "user", TransmissionPassword: "password"},
	} {
		// Given
		client := testClient(t, &mutableProvider{snapshot: snapshot})

		// When
		_, err := client.Files(context.Background(), "hash")

		// Then
		if !errors.Is(err, ErrInvalidConfig) {
			t.Fatalf("err=%v", err)
		}
	}
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		http.Redirect(w, request, "/other", http.StatusFound)
	}))
	defer redirect.Close()
	if _, err := testClient(t, &mutableProvider{snapshot: runtime.Snapshot{TransmissionURL: redirect.URL}}).Files(context.Background(), "hash"); !errors.Is(err, ErrRedirect) {
		t.Fatalf("err=%v", err)
	}
}

func TestClient_classifiesCallerCancellationAndTimeout_whenRequestCannotComplete(t *testing.T) {
	// Given
	upstream := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		select {
		case <-request.Context().Done():
		case <-time.After(100 * time.Millisecond):
		}
	}))
	defer upstream.Close()
	provider := &mutableProvider{snapshot: runtime.Snapshot{TransmissionURL: upstream.URL}}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	timedOut, err := New(Options{Provider: provider, Timeout: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}

	// When
	_, cancelErr := testClient(t, provider).Files(cancelled, "hash")
	_, timeoutErr := timedOut.Files(context.Background(), "hash")

	// Then
	if !errors.Is(cancelErr, context.Canceled) || !errors.Is(timeoutErr, ErrTimeout) {
		t.Fatalf("cancel=%v timeout=%v", cancelErr, timeoutErr)
	}
}

func TestClient_redactsConfigurationSecrets_whenTransportFails(t *testing.T) {
	// Given
	const passwordCanary = "password-canary"
	const sessionCanary = "session-canary"
	const urlCanary = "host-canary.invalid"
	provider := &mutableProvider{snapshot: runtime.Snapshot{TransmissionURL: "http://" + urlCanary, TransmissionUsername: "user", TransmissionPassword: passwordCanary}}
	client, err := New(Options{Provider: provider, HTTPClient: &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusConflict, Header: http.Header{"X-Transmission-Session-Id": []string{sessionCanary}}, Body: io.NopCloser(strings.NewReader(""))}, nil
	})}, Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}

	// When
	_, err = client.Files(context.Background(), "hash")

	// Then
	if err == nil || strings.Contains(err.Error(), passwordCanary) || strings.Contains(err.Error(), sessionCanary) || strings.Contains(err.Error(), urlCanary) {
		t.Fatal("error exposed a configuration secret")
	}
}
