package transmission

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"jproxy-go/internal/runtime"
)

func TestClient_admitsRenameMutation_whenLookupSucceeds(t *testing.T) {
	// Given
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if decodeReceivedRequest(t, request.Body).Method == "torrent-get" {
			_, _ = io.WriteString(writer, `{"result":"success","arguments":{"torrents":[{"id":7,"name":"old-root","files":[]}]}}`)
			return
		}
		_, _ = io.WriteString(writer, `{"result":"success","arguments":{"path":"old-root","name":"new-root","id":7}}`)
	}))
	defer upstream.Close()
	provider := &mutableProvider{snapshot: runtime.Snapshot{TransmissionURL: upstream.URL, TransmissionRevision: 1}}
	client := testClient(t, provider)

	// When
	err := client.Rename(context.Background(), "hash", "new-root")

	// Then
	if err != nil || provider.admissions() != 1 {
		t.Fatalf("err=%v admissions=%d", err, provider.admissions())
	}
}

func TestClient_rejectsRename_whenPublicationPrecedesMutationAdmission(t *testing.T) {
	// Given
	lookupStarted := make(chan struct{})
	lookupReleased := make(chan struct{})
	oldRequests := 0
	oldMutations := 0
	old := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		oldRequests++
		if decodeReceivedRequest(t, request.Body).Method != "torrent-get" {
			oldMutations++
			t.Fatal("stale mutation reached old endpoint")
		}
		close(lookupStarted)
		waitFor(t, lookupReleased, "lookup release")
		_, _ = io.WriteString(writer, `{"result":"success","arguments":{"torrents":[{"id":7,"name":"old-root","files":[]}]}}`)
	}))
	defer old.Close()
	newMutations := 0
	replacement := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { newMutations++ }))
	defer replacement.Close()
	provider := &mutableProvider{snapshot: runtime.Snapshot{TransmissionURL: old.URL, TransmissionRevision: 1}}
	client := testClient(t, provider)
	result := make(chan error, 1)
	go func() { result <- client.Rename(context.Background(), "hash", "new-root") }()
	waitFor(t, lookupStarted, "lookup start")
	provider.set(runtime.Snapshot{TransmissionURL: replacement.URL, TransmissionRevision: 2})

	// When
	close(lookupReleased)
	err := waitFor(t, result, "rename result")

	// Then
	if !errors.Is(err, ErrConfigChanged) || oldRequests != 1 || oldMutations != 0 || newMutations != 0 {
		t.Fatalf("err=%v old_requests=%d old_mutations=%d new_mutations=%d", err, oldRequests, oldMutations, newMutations)
	}
}

func TestClient_allowsPublicationWhileAdmittedRenameWaitsForResponse(t *testing.T) {
	// Given
	mutationStarted := make(chan struct{})
	releaseResponse := make(chan struct{})
	old := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if decodeReceivedRequest(t, request.Body).Method == "torrent-get" {
			_, _ = io.WriteString(writer, `{"result":"success","arguments":{"torrents":[{"id":7,"name":"old-root","files":[]}]}}`)
			return
		}
		close(mutationStarted)
		waitFor(t, releaseResponse, "mutation response release")
		_, _ = io.WriteString(writer, `{"result":"success","arguments":{"path":"old-root","name":"new-root","id":7}}`)
	}))
	defer old.Close()
	provider := &mutableProvider{snapshot: runtime.Snapshot{TransmissionURL: old.URL, TransmissionRevision: 1}}
	client := testClient(t, provider)
	result := make(chan error, 1)
	go func() { result <- client.Rename(context.Background(), "hash", "new-root") }()
	waitFor(t, mutationStarted, "mutation start")
	published := make(chan struct{})
	go func() {
		provider.set(runtime.Snapshot{TransmissionURL: old.URL, TransmissionRevision: 2})
		close(published)
	}()

	// When
	waitFor(t, published, "publication")
	close(releaseResponse)
	err := waitFor(t, result, "rename result")

	// Then
	if err != nil {
		t.Fatalf("err=%v", err)
	}
}

func TestClient_releasesPublicationAfterOuterRoundTripEntry_whenInnerTransportBlocks(t *testing.T) {
	// Given
	admitted := make(chan struct{}, 1)
	provider := &admissionControlledProvider{t: t, mutableProvider: mutableProvider{snapshot: runtime.Snapshot{TransmissionURL: "http://example.test", TransmissionRevision: 1}}, admitted: admitted}
	innerTransportEntered := make(chan struct{})
	releaseResponse := make(chan struct{})
	published := make(chan struct{})
	client, err := New(Options{Provider: provider, Timeout: time.Second, HTTPClient: &http.Client{Transport: roundTripperFunc(func(request *http.Request) (*http.Response, error) {
		var received receivedRequest
		if err := json.NewDecoder(request.Body).Decode(&received); err != nil {
			t.Fatal(err)
		}
		if received.Method == "torrent-get" {
			return testResponse(`{"result":"success","arguments":{"torrents":[{"id":7,"name":"old-root","files":[]}]}}`), nil
		}
		close(innerTransportEntered)
		waitFor(t, releaseResponse, "mutation response release")
		return testResponse(`{"result":"success","arguments":{"path":"old-root","name":"new-root","id":7}}`), nil
	})}})
	if err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	go func() { result <- client.Rename(context.Background(), "hash", "new-root") }()
	waitFor(t, admitted, "outer round trip entry")
	go func() {
		provider.set(runtime.Snapshot{TransmissionURL: "http://example.test", TransmissionRevision: 2})
		close(published)
	}()

	// When
	waitFor(t, innerTransportEntered, "inner transport entry")
	waitFor(t, published, "publication while inner transport is blocked")

	// Then
	close(releaseResponse)
	if err := waitFor(t, result, "rename result"); err != nil {
		t.Fatalf("err=%v", err)
	}
}

func TestClient_rejectsRenameReplay_whenPublicationFollowsInitialConflict(t *testing.T) {
	// Given
	initialMutationStarted := make(chan struct{})
	releaseConflict := make(chan struct{})
	mutations := 0
	old := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if decodeReceivedRequest(t, request.Body).Method == "torrent-get" {
			_, _ = io.WriteString(writer, `{"result":"success","arguments":{"torrents":[{"id":7,"name":"old-root","files":[]}]}}`)
			return
		}
		mutations++
		close(initialMutationStarted)
		waitFor(t, releaseConflict, "conflict release")
		writer.Header().Set("X-Transmission-Session-Id", "session")
		writer.WriteHeader(http.StatusConflict)
	}))
	defer old.Close()
	newMutations := 0
	replacement := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { newMutations++ }))
	defer replacement.Close()
	provider := &mutableProvider{snapshot: runtime.Snapshot{TransmissionURL: old.URL, TransmissionRevision: 1}}
	client := testClient(t, provider)
	result := make(chan error, 1)
	go func() { result <- client.Rename(context.Background(), "hash", "new-root") }()
	waitFor(t, initialMutationStarted, "initial mutation start")
	provider.set(runtime.Snapshot{TransmissionURL: replacement.URL, TransmissionRevision: 2})

	// When
	close(releaseConflict)
	err := waitFor(t, result, "rename result")

	// Then
	if !errors.Is(err, ErrConfigChanged) || mutations != 1 || newMutations != 0 {
		t.Fatalf("err=%v mutations=%d new_mutations=%d", err, mutations, newMutations)
	}
}

func TestClient_rejectsRename_whenProviderLacksMutationAdmission(t *testing.T) {
	// Given
	requests := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests++
		if decodeReceivedRequest(t, request.Body).Method != "torrent-get" {
			t.Fatal("mutation reached provider without admission")
		}
		_, _ = io.WriteString(writer, `{"result":"success","arguments":{"torrents":[{"id":7,"name":"old-root","files":[]}]}}`)
	}))
	defer upstream.Close()
	client := testClient(t, providerWithoutAdmission{snapshot: runtime.Snapshot{TransmissionURL: upstream.URL, TransmissionRevision: 1}})

	// When
	err := client.Rename(context.Background(), "hash", "new-root")

	// Then
	if !errors.Is(err, ErrConfigChanged) || requests != 1 {
		t.Fatalf("err=%v requests=%d", err, requests)
	}
}

type providerWithoutAdmission struct{ snapshot runtime.Snapshot }

func (p providerWithoutAdmission) Snapshot() runtime.Snapshot                 { return p.snapshot }
func (providerWithoutAdmission) Refresh(context.Context, runtime.Scope) error { return nil }

type admissionControlledProvider struct {
	t *testing.T
	mutableProvider
	admitted chan<- struct{}
}

func (p *admissionControlledProvider) AdmitTransmissionMutation(revision uint64, begin func()) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.snapshot.TransmissionRevision != revision {
		return runtime.ErrTransmissionRevisionStale
	}
	p.admissionCalls++
	begin()
	p.admitted <- struct{}{}
	return nil
}

func testResponse(body string) *http.Response {
	return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(bytes.NewBufferString(body))}
}
