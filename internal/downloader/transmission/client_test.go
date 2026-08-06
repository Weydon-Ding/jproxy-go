package transmission

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"jproxy-go/internal/runtime"
)

type mutableProvider struct {
	mu       sync.RWMutex
	snapshot runtime.Snapshot
}

func (p *mutableProvider) Snapshot() runtime.Snapshot {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.snapshot
}

func (*mutableProvider) Refresh(context.Context, runtime.Scope) error { return nil }

func (p *mutableProvider) set(snapshot runtime.Snapshot) {
	p.mu.Lock()
	p.snapshot = snapshot
	p.mu.Unlock()
}

func TestClient_replaysOldEnvelopeOnceAfterSessionChallenge(t *testing.T) {
	// Given
	var bodies []string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		bodies = append(bodies, string(body))
		user, password, ok := r.BasicAuth()
		if !ok || user != "user" || password != "password" || r.Header.Get("Content-Type") != "application/json" {
			t.Fatal("request authentication or content type is invalid")
		}
		if len(bodies) == 1 {
			w.Header().Set("X-Transmission-Session-Id", "session")
			w.WriteHeader(http.StatusConflict)
			return
		}
		if r.Header.Get("X-Transmission-Session-Id") != "session" {
			t.Fatal("request session is invalid")
		}
		_, _ = io.WriteString(w, `{"result":"success","arguments":{"torrents":[{"name":"root","files":[{"name":"one"},{"name":"two"}]}]}}`)
	}))
	defer upstream.Close()
	client := testClient(t, &mutableProvider{snapshot: runtime.Snapshot{TransmissionURL: upstream.URL, TransmissionUsername: "user", TransmissionPassword: "password", TransmissionRevision: 1}})

	// When
	files, err := client.Files(context.Background(), "hash")

	// Then
	if err != nil || strings.Join(files, ",") != "one,two" || len(bodies) != 2 || bodies[0] != bodies[1] || bodies[0] != `{"method":"torrent-get","arguments":{"ids":["hash"],"fields":["name","files"]}}` {
		t.Fatalf("files=%v err=%v bodies=%v", files, err, bodies)
	}
}

func TestClient_classifiesProtocolAndTransportFailures_whenResponsesAreInvalid(t *testing.T) {
	for _, test := range []struct {
		name, body string
		status     int
		want       error
	}{
		{name: "authentication", status: http.StatusUnauthorized, want: ErrAuthentication},
		{name: "upstream status", status: http.StatusInternalServerError, want: ErrUpstreamStatus},
		{name: "missing session", status: http.StatusConflict, want: ErrSessionChallenge},
		{name: "whitespace session", status: http.StatusConflict, want: ErrSessionChallenge},
		{name: "second session", status: http.StatusConflict, want: ErrSessionChallenge},
		{name: "result", status: http.StatusOK, body: `{"result":"failure","arguments":{}}`, want: ErrUpstreamResult},
		{name: "malformed", status: http.StatusOK, body: `{`, want: ErrMalformedResponse},
		{name: "trailing", status: http.StatusOK, body: `{} {}`, want: ErrMalformedResponse},
		{name: "large", status: http.StatusOK, body: strings.Repeat("x", maxResponseBytes+1), want: ErrResponseTooLarge},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Given
			calls := 0
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls++
				if test.name == "second session" {
					w.Header().Set("X-Transmission-Session-Id", "one")
				}
				if test.name == "whitespace session" {
					w.Header().Set("X-Transmission-Session-Id", " ")
				}
				w.WriteHeader(test.status)
				_, _ = io.WriteString(w, test.body)
			}))
			defer upstream.Close()

			// When
			_, err := testClient(t, &mutableProvider{snapshot: runtime.Snapshot{TransmissionURL: upstream.URL, TransmissionUsername: "user", TransmissionPassword: "password", TransmissionRevision: 1}}).Files(context.Background(), "hash")

			// Then
			if !errors.Is(err, test.want) || (test.name == "second session" && calls != 2) {
				t.Fatalf("err=%v calls=%d", err, calls)
			}
		})
	}
}

func TestClient_usesCurrentEndpointWithoutLeakingStaleSession_whenRevisionChangesDuringChallenge(t *testing.T) {
	// Given
	challenged := make(chan struct{}, 1)
	release := make(chan struct{})
	old := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		challenged <- struct{}{}
		<-release
		w.Header().Set("X-Transmission-Session-Id", "old-session")
		w.WriteHeader(http.StatusConflict)
	}))
	defer old.Close()
	newRequests := 0
	replacement := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		newRequests++
		if r.Header.Get("X-Transmission-Session-Id") != "" {
			t.Fatal("received stale session")
		}
		user, password, _ := r.BasicAuth()
		if user != "new" || password != "new-password" {
			t.Fatal("received incorrect credentials")
		}
		_, _ = io.WriteString(w, `{"result":"success","arguments":{"torrents":[{"name":"fresh","files":[{"name":"file"}]}]}}`)
	}))
	defer replacement.Close()
	provider := &mutableProvider{snapshot: runtime.Snapshot{TransmissionURL: old.URL, TransmissionUsername: "old", TransmissionPassword: "old-password", TransmissionRevision: 1}}
	client := testClient(t, provider)
	result := make(chan error, 1)
	go func() { _, err := client.Files(context.Background(), "hash"); result <- err }()
	<-challenged
	provider.set(runtime.Snapshot{TransmissionURL: replacement.URL, TransmissionUsername: "new", TransmissionPassword: "new-password", TransmissionRevision: 2})

	// When
	close(release)
	err := <-result

	// Then
	if err != nil || newRequests != 1 {
		t.Fatalf("err=%v new_requests=%d", err, newRequests)
	}
}

func TestClient_rejectsInvalidConfigurationAndRedirects(t *testing.T) {
	// Given / When / Then
	client, err := New(Options{Provider: &mutableProvider{snapshot: runtime.Snapshot{TransmissionURL: "http://user:password@example.test", TransmissionUsername: "user", TransmissionPassword: "password"}}, Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Files(context.Background(), "hash"); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("invalid config err=%v", err)
	}
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/other", http.StatusFound) }))
	defer redirect.Close()
	if _, err := testClient(t, &mutableProvider{snapshot: runtime.Snapshot{TransmissionURL: redirect.URL, TransmissionUsername: "user", TransmissionPassword: "password"}}).Files(context.Background(), "hash"); !errors.Is(err, ErrRedirect) {
		t.Fatalf("redirect err=%v", err)
	}
}

func TestClient_classifiesCallerCancellationAndTimeout_whenRequestCannotComplete(t *testing.T) {
	// Given
	upstream := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(100 * time.Millisecond):
		}
	}))
	defer upstream.Close()
	provider := &mutableProvider{snapshot: runtime.Snapshot{TransmissionURL: upstream.URL, TransmissionUsername: "user", TransmissionPassword: "password"}}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	client := testClient(t, provider)
	timedOut, err := New(Options{Provider: provider, Timeout: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}

	// When
	_, cancelErr := client.Files(cancelled, "hash")
	_, timeoutErr := timedOut.Files(context.Background(), "hash")

	// Then
	if !errors.Is(cancelErr, context.Canceled) || !errors.Is(timeoutErr, ErrTimeout) {
		t.Fatalf("cancel=%v timeout=%v", cancelErr, timeoutErr)
	}
}

func TestClient_rejectsInvalidOperationInputsAndOversizedPayload_beforeTransport(t *testing.T) {
	// Given
	client := testClient(t, &mutableProvider{snapshot: runtime.Snapshot{TransmissionURL: "http://example.test", TransmissionUsername: "user", TransmissionPassword: "password"}})
	largeName := strings.Repeat("a", maxRequestBytes)

	// When / Then
	for _, hash := range []string{"", "bad\x00hash"} {
		if _, err := client.Files(context.Background(), hash); !errors.Is(err, ErrInvalidConfig) {
			t.Fatalf("invalid hash category=%v", err)
		}
	}
	for _, name := range []string{"", ".", "..", "dir/name", `dir\name`, "bad\x00name"} {
		if err := client.Rename(context.Background(), "hash", name); !errors.Is(err, ErrInvalidPath) {
			t.Fatalf("invalid name category=%v", err)
		}
	}
	if _, err := marshalRequest("torrent-rename-path", renamePathArguments{IDs: []string{"hash"}, Path: "old", Name: largeName}); !errors.Is(err, ErrRequestTooLarge) {
		t.Fatalf("oversized request category=%v", err)
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

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (transport roundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}

func testClient(t *testing.T, provider runtime.Provider) *Client {
	t.Helper()
	client, err := New(Options{Provider: provider, Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	return client
}
