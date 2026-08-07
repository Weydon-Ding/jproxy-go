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
)

func TestClient_admitsRenameAndRenameFileMutations(t *testing.T) {
	for _, operation := range []struct {
		name string
		call func(*Client) error
	}{
		{name: "rename", call: func(client *Client) error { return client.Rename(context.Background(), "hash", "name") }},
		{name: "rename file", call: func(client *Client) error {
			return client.RenameFile(context.Background(), "hash", "old/file", "old/new")
		}},
	} {
		t.Run(operation.name, func(t *testing.T) {
			// Given
			mutations := 0
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/v2/auth/login" {
					http.SetCookie(w, &http.Cookie{Name: "SID", Value: "session"})
					return
				}
				mutations++
				w.WriteHeader(http.StatusNoContent)
			}))
			defer upstream.Close()
			provider := &admissionProvider{snapshot: qbSnapshot(upstream.URL, 1)}
			client := newClient(t, provider, nil)

			// When
			err := operation.call(client)

			// Then
			if err != nil || provider.admissions() != 1 || mutations != 1 {
				t.Fatalf("err=%v admissions=%d mutations=%d", err, provider.admissions(), mutations)
			}
		})
	}
}

func TestClient_rejectsMutation_whenProviderLacksAdmission(t *testing.T) {
	// Given
	requests := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		http.SetCookie(w, &http.Cookie{Name: "SID", Value: "session"})
	}))
	defer upstream.Close()
	client := newClient(t, providerWithoutAdmission{snapshot: qbSnapshot(upstream.URL, 1)}, nil)

	// When
	err := client.Rename(context.Background(), "hash", "name")

	// Then
	if !errors.Is(err, ErrConfigChanged) || requests != 1 {
		t.Fatalf("err=%v requests=%d", err, requests)
	}
}

func TestClient_readsFilesAndLogsIn_withoutMutationAdmission(t *testing.T) {
	// Given
	logins, files := 0, 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/auth/login" {
			logins++
			http.SetCookie(w, &http.Cookie{Name: "SID", Value: "session"})
			return
		}
		files++
		_, _ = io.WriteString(w, `[{"name":"file"}]`)
	}))
	defer upstream.Close()
	client := newClient(t, providerWithoutAdmission{snapshot: qbSnapshot(upstream.URL, 1)}, nil)

	// When
	err := client.Login(context.Background())
	got, filesErr := client.Files(context.Background(), "hash")

	// Then
	if err != nil || filesErr != nil || len(got) != 1 || logins != 1 || files != 1 {
		t.Fatalf("login=%v files=%v got=%v logins=%d requests=%d", err, filesErr, got, logins, files)
	}
}

func TestClient_releasesPublicationAfterOuterRoundTripEntry_whenInnerTransportBlocks(t *testing.T) {
	// Given
	provider := &admissionProvider{snapshot: qbSnapshot("http://example.test", 1), admitted: make(chan struct{}, 1)}
	innerEntered := make(chan struct{})
	releaseResponse := make(chan struct{})
	client := newClient(t, provider, roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/api/v2/auth/login" {
			return response(http.StatusOK, "", "SID=session"), nil
		}
		close(innerEntered)
		<-releaseResponse
		return response(http.StatusNoContent, "", ""), nil
	}))
	result := make(chan error, 1)
	go func() { result <- client.Rename(context.Background(), "hash", "name") }()
	<-provider.admitted
	published := make(chan struct{})
	go func() { provider.set(qbSnapshot("http://example.test", 2)); close(published) }()

	// When
	<-innerEntered
	<-published
	close(releaseResponse)

	// Then
	if err := <-result; err != nil {
		t.Fatal(err)
	}
}

func TestClient_rejectsBothMutations_whenPublicationPrecedesAdmission(t *testing.T) {
	for _, operation := range []struct {
		name string
		call func(*Client) error
	}{
		{name: "rename", call: func(client *Client) error { return client.Rename(context.Background(), "hash", "name") }},
		{name: "rename file", call: func(client *Client) error { return client.RenameFile(context.Background(), "hash", "old", "new") }},
	} {
		t.Run(operation.name, func(t *testing.T) {
			// Given
			mutations := 0
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/v2/auth/login" {
					http.SetCookie(w, &http.Cookie{Name: "SID", Value: "session"})
					return
				}
				mutations++
			}))
			defer upstream.Close()
			entered := make(chan struct{})
			release := make(chan struct{})
			provider := &admissionProvider{snapshot: qbSnapshot(upstream.URL, 1), beforeAdmission: entered, releaseAdmission: release}
			client := newClient(t, provider, nil)
			result := make(chan error, 1)
			go func() { result <- operation.call(client) }()
			<-entered
			provider.set(qbSnapshot(upstream.URL, 2))

			// When
			close(release)
			err := <-result

			// Then
			if !errors.Is(err, ErrConfigChanged) || mutations != 0 {
				t.Fatalf("err=%v mutations=%d", err, mutations)
			}
		})
	}
}

type admissionProvider struct {
	mu               sync.RWMutex
	snapshot         runtime.Snapshot
	admissionCalls   int
	admitted         chan struct{}
	beforeAdmission  chan<- struct{}
	blockAfterFirst  chan<- struct{}
	releaseAdmission <-chan struct{}
}

func (p *admissionProvider) Snapshot() runtime.Snapshot {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.snapshot
}
func (*admissionProvider) Refresh(context.Context, runtime.Scope) error { return nil }
func (p *admissionProvider) set(snapshot runtime.Snapshot) {
	p.mu.Lock()
	p.snapshot = snapshot
	p.mu.Unlock()
}
func (p *admissionProvider) admissions() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.admissionCalls
}
func (p *admissionProvider) AdmitQBittorrentMutation(revision uint64, begin func()) error {
	if p.beforeAdmission != nil {
		p.beforeAdmission <- struct{}{}
		<-p.releaseAdmission
	}
	p.mu.RLock()
	block := p.admissionCalls == 1 && p.blockAfterFirst != nil
	p.mu.RUnlock()
	if block {
		p.blockAfterFirst <- struct{}{}
		<-p.releaseAdmission
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.snapshot.QBittorrentRevision != revision {
		return runtime.ErrQBittorrentRevisionStale
	}
	p.admissionCalls++
	begin()
	if p.admitted != nil {
		p.admitted <- struct{}{}
	}
	return nil
}

type providerWithoutAdmission struct{ snapshot runtime.Snapshot }

func (p providerWithoutAdmission) Snapshot() runtime.Snapshot                 { return p.snapshot }
func (providerWithoutAdmission) Refresh(context.Context, runtime.Scope) error { return nil }

func qbSnapshot(endpoint string, revision uint64) runtime.Snapshot {
	return runtime.Snapshot{QBittorrentURL: endpoint, QBittorrentUsername: "user", QBittorrentPassword: "password", QBittorrentRevision: revision}
}

func newClient(t *testing.T, provider runtime.Provider, transport http.RoundTripper) *Client {
	t.Helper()
	client, err := New(Options{Provider: provider, HTTPClient: &http.Client{Transport: transport}, Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	return client
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (transport roundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}

func response(status int, body, cookie string) *http.Response {
	header := make(http.Header)
	if cookie != "" {
		header.Set("Set-Cookie", cookie)
	}
	return &http.Response{StatusCode: status, Header: header, Body: io.NopCloser(strings.NewReader(body))}
}
