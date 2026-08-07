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

	"jproxy-go/internal/runtime"
)

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
		_, _ = io.WriteString(w, `{"result":"success","arguments":{"torrents":[{"id":7,"name":"root","files":[{"name":"one"},{"name":"two"}]}]}}`)
	}))
	defer upstream.Close()
	client := testClient(t, &mutableProvider{snapshot: runtime.Snapshot{TransmissionURL: upstream.URL, TransmissionUsername: "user", TransmissionPassword: "password", TransmissionRevision: 1}})

	// When
	files, err := client.Files(context.Background(), "hash")

	// Then
	if err != nil || strings.Join(files, ",") != "one,two" || len(bodies) != 2 || bodies[0] != bodies[1] || bodies[0] != `{"method":"torrent-get","arguments":{"ids":["hash"],"fields":["id","name","files"]}}` {
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

func TestClient_rejectsReplay_whenRevisionChangesDuringChallenge(t *testing.T) {
	// Given
	challenged := make(chan struct{}, 1)
	release := make(chan struct{})
	old := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		challenged <- struct{}{}
		waitFor(t, release, "challenge release")
		w.Header().Set("X-Transmission-Session-Id", "old-session")
		w.WriteHeader(http.StatusConflict)
	}))
	defer old.Close()
	newRequests := 0
	replacement := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { newRequests++ }))
	defer replacement.Close()
	provider := &mutableProvider{snapshot: runtime.Snapshot{TransmissionURL: old.URL, TransmissionUsername: "old", TransmissionPassword: "old-password", TransmissionRevision: 1}}
	client := testClient(t, provider)
	result := make(chan error, 1)
	go func() { _, err := client.Files(context.Background(), "hash"); result <- err }()
	waitFor(t, challenged, "session challenge")
	provider.set(runtime.Snapshot{TransmissionURL: replacement.URL, TransmissionUsername: "new", TransmissionPassword: "new-password", TransmissionRevision: 2})

	// When
	close(release)
	err := waitFor(t, result, "files result")

	// Then
	if !errors.Is(err, ErrConfigChanged) || newRequests != 0 {
		t.Fatalf("err=%v new_requests=%d", err, newRequests)
	}
}

func TestClient_keepsNewerSession_whenOlderConcurrentChallengeArrivesLast(t *testing.T) {
	// Given
	initial := make(chan struct{}, 2)
	releaseOlder := make(chan struct{})
	var firstInitial sync.Once
	var requestsMu sync.Mutex
	requests := make(map[string]int)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Transmission-Session-Id")
		requestsMu.Lock()
		requests[id]++
		requestsMu.Unlock()
		switch id {
		case "":
			older := false
			firstInitial.Do(func() { older = true })
			initial <- struct{}{}
			if older {
				waitFor(t, releaseOlder, "older challenge release")
				w.Header().Set("X-Transmission-Session-Id", "older")
			} else {
				w.Header().Set("X-Transmission-Session-Id", "newer")
			}
			w.WriteHeader(http.StatusConflict)
		case "older", "newer":
			_, _ = io.WriteString(w, `{"result":"success","arguments":{"version":"4.0"}}`)
		default:
			t.Fatalf("unexpected session %q", id)
		}
	}))
	defer upstream.Close()
	client := testClient(t, &mutableProvider{snapshot: runtime.Snapshot{TransmissionURL: upstream.URL, TransmissionRevision: 1}})
	results := make(chan error, 2)
	for range 2 {
		go func() { results <- client.Login(context.Background()) }()
	}
	waitFor(t, initial, "first initial challenge")
	waitFor(t, initial, "second initial challenge")

	// When
	if err := waitFor(t, results, "newer session result"); err != nil {
		t.Fatal(err)
	}
	close(releaseOlder)
	if err := waitFor(t, results, "older session result"); err != nil {
		t.Fatal(err)
	}
	if err := client.Login(context.Background()); err != nil {
		t.Fatal(err)
	}

	// Then
	requestsMu.Lock()
	defer requestsMu.Unlock()
	if requests[""] != 2 || requests["older"] != 1 || requests["newer"] != 2 {
		t.Fatalf("requests=%v", requests)
	}
}
