package qbittorrent

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
	"jproxy-go/internal/store/sqlite"
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

func TestClient_usesQBittorrentProtocol_whenOperationsSucceed(t *testing.T) {
	// Given
	requests := make([]string, 0, 4)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := ""
		if r.Body != nil {
			data := make([]byte, r.ContentLength)
			_, _ = r.Body.Read(data)
			body = string(data)
		}
		requests = append(requests, r.Method+" "+r.URL.RequestURI()+" "+body+" "+r.Header.Get("Cookie"))
		switch r.URL.Path {
		case "/api/v2/auth/login":
			http.SetCookie(w, &http.Cookie{Name: "SID", Value: "session"})
		case "/api/v2/torrents/files":
			_, _ = w.Write([]byte(`[{"name":"one"},{"name":"two","extra":1}]`))
		default:
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer upstream.Close()
	client := testClient(t, upstream.URL)

	// When
	files, err := client.Files(context.Background(), "hash")
	if err == nil {
		err = client.Rename(context.Background(), "hash", "name")
	}
	if err == nil {
		err = client.RenameFile(context.Background(), "hash", "old", "new")
	}

	// Then
	if err != nil || strings.Join(files, ",") != "one,two" || len(requests) != 4 || !strings.Contains(requests[0], "password=password&username=user") || !strings.Contains(requests[1], "hash=hash") || !strings.Contains(requests[2], "hash=hash&name=name") || !strings.Contains(requests[3], "hash=hash&newPath=new&oldPath=old") || !strings.Contains(requests[3], "SID=session") {
		t.Fatalf("files=%v err=%v requests=%v", files, err, requests)
	}
}

func TestClient_retriesOnceAfterUnauthorized_whenRefreshSucceeds(t *testing.T) {
	// Given
	var calls, logins int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/auth/login" {
			logins++
			http.SetCookie(w, &http.Cookie{Name: "QBT_SID_8080", Value: "s" + string(rune('0'+logins))})
			return
		}
		calls++
		if calls == 1 {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`[{"name":"file"}]`))
	}))
	defer upstream.Close()

	// When
	files, err := testClient(t, upstream.URL).Files(context.Background(), "hash")

	// Then
	if err != nil || len(files) != 1 || calls != 2 || logins != 2 {
		t.Fatalf("files=%v err=%v calls=%d logins=%d", files, err, calls, logins)
	}
}

func testClient(t *testing.T, upstream string) *Client {
	t.Helper()
	provider := runtime.NewStaticProvider(sqlite.Snapshot{QBittorrentURL: upstream, QBittorrentUsername: "user", QBittorrentPassword: "password"})
	client, err := New(Options{Provider: provider, Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func TestClient_stopsAfterSecondAuthenticationFailure_whenRefreshFails(t *testing.T) {
	// Given
	var mu sync.Mutex
	operations, logins := 0, 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.URL.Path == "/api/v2/auth/login" {
			logins++
			http.SetCookie(w, &http.Cookie{Name: "SID", Value: "s"})
			return
		}
		operations++
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer upstream.Close()

	// When
	_, err := testClient(t, upstream.URL).Files(context.Background(), "hash")

	// Then
	mu.Lock()
	defer mu.Unlock()
	if !errors.Is(err, ErrAuthentication) || operations != 2 || logins != 2 {
		t.Fatalf("err=%v operations=%d logins=%d", err, operations, logins)
	}
}

func TestClient_sharesOneLogin_whenOperationsStartConcurrently(t *testing.T) {
	// Given
	var mu sync.Mutex
	logins := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/auth/login" {
			mu.Lock()
			logins++
			mu.Unlock()
			http.SetCookie(w, &http.Cookie{Name: "SID", Value: "session"})
			return
		}
		_, _ = io.WriteString(w, `[{"name":"file"}]`)
	}))
	defer upstream.Close()
	client := testClient(t, upstream.URL)
	start := make(chan struct{})
	results := make(chan error, 12)
	for range 12 {
		go func() { <-start; _, err := client.Files(context.Background(), "hash"); results <- err }()
	}
	close(start)

	// When / Then
	for range 12 {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if logins != 1 {
		t.Fatalf("logins=%d", logins)
	}
}

func TestClient_switchesEndpointAndDiscardsOldCookie_whenRevisionChanges(t *testing.T) {
	// Given
	old := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/auth/login" {
			http.SetCookie(w, &http.Cookie{Name: "SID", Value: "old"})
			return
		}
		_, _ = io.WriteString(w, `[{"name":"old"}]`)
	}))
	defer old.Close()
	replacement := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Cookie") == "SID=old" {
			t.Error("old cookie sent")
		}
		if r.URL.Path == "/api/v2/auth/login" {
			http.SetCookie(w, &http.Cookie{Name: "SID", Value: "new"})
			return
		}
		_, _ = io.WriteString(w, `[{"name":"new"}]`)
	}))
	defer replacement.Close()
	provider := &mutableProvider{snapshot: runtime.Snapshot{QBittorrentURL: old.URL, QBittorrentUsername: "old", QBittorrentPassword: "old", QBittorrentRevision: 1}}
	client, err := New(Options{Provider: provider, Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Files(context.Background(), "hash")
	if err != nil {
		t.Fatal(err)
	}
	provider.set(runtime.Snapshot{QBittorrentURL: replacement.URL, QBittorrentUsername: "new", QBittorrentPassword: "new", QBittorrentRevision: 2})

	// When
	files, err := client.Files(context.Background(), "hash")

	// Then
	if err != nil || strings.Join(files, ",") != "new" {
		t.Fatalf("files=%v err=%v", files, err)
	}
}

func TestClient_classifiesTimeoutRedirectAndMalformedFiles(t *testing.T) {
	// Given / When / Then
	for _, test := range []struct {
		name, body string
		status     int
		want       error
	}{{"malformed", "{", 200, ErrMalformedResponse}, {"trailing", `[]{}`, 200, ErrMalformedResponse}, {"nonarray", `{}`, 200, ErrMalformedResponse}, {"missing name", `[{}]`, 200, ErrMalformedResponse}, {"blank name", `[{"name":" "}]`, 200, ErrMalformedResponse}, {"large", strings.Repeat("x", maxResponseBytes+1), 200, ErrResponseTooLarge}, {"status", "", 500, ErrUpstreamStatus}} {
		t.Run(test.name, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/v2/auth/login" {
					http.SetCookie(w, &http.Cookie{Name: "SID", Value: "s"})
					return
				}
				w.WriteHeader(test.status)
				_, _ = io.WriteString(w, test.body)
			}))
			defer upstream.Close()
			_, err := testClient(t, upstream.URL).Files(context.Background(), "h")
			if !errors.Is(err, test.want) {
				t.Fatalf("err=%v", err)
			}
		})
	}
}

func TestClient_prefersSIDAndPreservesCallerCancellation(t *testing.T) {
	// Given
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/auth/login" {
			http.SetCookie(w, &http.Cookie{Name: "QBT_SID_1", Value: "wrong"})
			http.SetCookie(w, &http.Cookie{Name: "SID", Value: "right"})
			return
		}
		if r.Header.Get("Cookie") != "SID=right" {
			t.Errorf("cookie=%q", r.Header.Get("Cookie"))
		}
		<-r.Context().Done()
	}))
	defer upstream.Close()
	client := testClient(t, upstream.URL)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// When
	_, err := client.Files(ctx, "hash")

	// Then
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v", err)
	}
}

func TestClient_discardsStaleLogin_whenRuntimeRevisionChangesDuringLogin(t *testing.T) {
	// Given
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	var oldMu sync.Mutex
	oldOperations := 0
	old := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/auth/login" {
			started <- struct{}{}
			<-release
			http.SetCookie(w, &http.Cookie{Name: "SID", Value: "stale"})
			return
		}
		oldMu.Lock()
		oldOperations++
		oldMu.Unlock()
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer old.Close()
	var newMu sync.Mutex
	newLogins, newOperations := 0, 0
	replacement := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/auth/login" {
			newMu.Lock()
			newLogins++
			newMu.Unlock()
			http.SetCookie(w, &http.Cookie{Name: "SID", Value: "fresh"})
			return
		}
		if r.Header.Get("Cookie") != "SID=fresh" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		newMu.Lock()
		newOperations++
		newMu.Unlock()
		_, _ = io.WriteString(w, `[{"name":"latest"}]`)
	}))
	defer replacement.Close()
	provider := &mutableProvider{snapshot: runtime.Snapshot{QBittorrentURL: old.URL, QBittorrentUsername: "old", QBittorrentPassword: "old", QBittorrentRevision: 1}}
	client, err := New(Options{Provider: provider, Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	result := make(chan struct {
		files []string
		err   error
	}, 1)
	go func() {
		files, callErr := client.Files(context.Background(), "hash")
		result <- struct {
			files []string
			err   error
		}{files, callErr}
	}()
	<-started
	provider.set(runtime.Snapshot{QBittorrentURL: replacement.URL, QBittorrentUsername: "new", QBittorrentPassword: "new", QBittorrentRevision: 2})

	// When
	close(release)
	got := <-result

	// Then
	oldMu.Lock()
	observedOldOperations := oldOperations
	oldMu.Unlock()
	newMu.Lock()
	observedNewLogins, observedNewOperations := newLogins, newOperations
	newMu.Unlock()
	if got.err != nil || strings.Join(got.files, ",") != "latest" || observedOldOperations != 0 || observedNewLogins != 1 || observedNewOperations != 1 {
		t.Fatalf("files=%v err=%v old_operations=%d new_logins=%d new_operations=%d", got.files, got.err, observedOldOperations, observedNewLogins, observedNewOperations)
	}
}
