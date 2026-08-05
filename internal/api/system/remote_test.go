package system

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestVersion_acceptsRealisticGitHubReleaseSchema(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"url":"https://api.github.test/releases/1","assets_url":"https://api.github.test/assets","author":{"login":"octocat"},"id":1,"tag_name":"v9.9.9","assets":[],"prerelease":false}`)
	}))
	defer server.Close()
	handler := NewHandler(Options{Version: "1.0.0", VersionURL: server.URL})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/system/config/version", nil))
	if response.Code != http.StatusOK || response.Body.String() != "1.0.0 🚨" {
		t.Fatalf("status=%d body=%q", response.Code, response.Body.String())
	}
}

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
		{"missing_tag", `{}`, http.StatusOK}, {"blank_tag", `{"tag_name":" "}`, http.StatusOK},
		{"non_string_tag", `{"tag_name":1}`, http.StatusOK}, {"second_value", `{"tag_name":"v1"} {}`, http.StatusOK},
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
	t.Logf("task6_remote_fallback primary=%d backup=%d default=false", primaryHits.Load(), backupHits.Load())
}

func TestAuthorList_rejectsUnknownFieldsBeforeUsingBackup(t *testing.T) {
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `["primary"] {}`)
	}))
	backup := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `["backup"]`)
	}))
	defer primary.Close()
	defer backup.Close()

	authors := NewHandler(Options{AuthorURL: primary.URL, AuthorBackupURL: backup.URL}).authorList(context.Background())
	if len(authors) != 1 || authors[0] != "backup" {
		t.Fatalf("authors=%q", authors)
	}
}

func TestAuthorList_returnsDefaultAfterBothSourcesFail(t *testing.T) {
	failing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusBadGateway) }))
	defer failing.Close()
	handler := NewHandler(Options{AuthorURL: failing.URL, AuthorBackupURL: failing.URL})
	authors := handler.authorList(context.Background())
	if len(authors) != 1 || authors[0] != "LuckyPuppy514" {
		t.Fatalf("authors=%v", authors)
	}
	t.Log("task6_remote_fallback primary=true backup=true default=true")
}
