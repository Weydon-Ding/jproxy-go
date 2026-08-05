package system

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestRemoteDefaults_matchJavaProductionResources(t *testing.T) {
	if defaultVersionURL != "https://api.github.com/repos/LuckyPuppy514/jproxy/releases/latest" || defaultAuthorURL != "https://raw.githubusercontent.com/LuckyPuppy514/jproxy/main/src/main/resources/rule/author.json" || defaultAuthorBackupURL != "https://github.rn.lckp.top/LuckyPuppy514/jproxy/main/src/main/resources/rule/author.json" {
		t.Fatalf("defaults=%q/%q/%q", defaultVersionURL, defaultAuthorURL, defaultAuthorBackupURL)
	}
}

func TestFetchJSON_rejectsRemoteBoundaryResponses(t *testing.T) {
	overLimit := `{"tag_name":"v1"}` + strings.Repeat(" ", remoteBodyLimit)
	cases := []struct {
		name, body string
		status     int
	}{
		{"oversized_trailing_whitespace", overLimit, http.StatusOK}, {"misleading_200", `{"tag_name":""}`, http.StatusOK},
		{"unknown_field", `{"tag_name":"v1","extra":true}`, http.StatusOK}, {"second_value", `{"tag_name":"v1"} {}`, http.StatusOK},
		{"non_2xx", `{"tag_name":"v1"}`, http.StatusBadGateway},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(testCase.status)
				_, _ = w.Write([]byte(testCase.body))
			}))
			defer server.Close()
			if _, ok := fetchVersion(context.Background(), server.URL); ok {
				t.Fatal("accepted invalid response")
			}
		})
	}
}

func TestFetchJSON_rejectsRedirectAndCancellation(t *testing.T) {
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		http.Redirect(w, request, "/next", http.StatusFound)
	}))
	defer redirect.Close()
	if _, ok := fetchVersion(context.Background(), redirect.URL); ok {
		t.Fatal("followed redirect")
	}
	started := make(chan struct{})
	hung := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) { close(started); <-request.Context().Done() }))
	defer hung.Close()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan bool, 1)
	go func() { _, ok := fetchVersion(ctx, hung.URL); done <- ok }()
	<-started
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("cancellation did not stop request")
	}
}

func TestAuthorList_retriesPrimaryThenBackupOnEveryRequest(t *testing.T) {
	var primaryHits, backupHits atomic.Int32
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { primaryHits.Add(1); _, _ = w.Write([]byte(`[]`)) }))
	backup := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { backupHits.Add(1); _, _ = w.Write([]byte(`["backup"]`)) }))
	defer primary.Close()
	defer backup.Close()
	handler := NewHandler(Options{AuthorURL: primary.URL, AuthorBackupURL: backup.URL})
	for range 2 {
		authors := handler.authorList(context.Background())
		if len(authors) != 1 || authors[0] != "backup" {
			t.Fatalf("authors=%v", authors)
		}
	}
	if primaryHits.Load() != 2 || backupHits.Load() != 2 {
		t.Fatalf("primary=%d backup=%d", primaryHits.Load(), backupHits.Load())
	}
}
