package qbittorrent

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_readmitsReplayWithCurrentConfigAndCookie_afterUnauthorized(t *testing.T) {
	// Given
	firstMutation := make(chan struct{})
	releaseUnauthorized := make(chan struct{})
	old := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/auth/login" {
			http.SetCookie(w, &http.Cookie{Name: "SID", Value: "old"})
			return
		}
		close(firstMutation)
		<-releaseUnauthorized
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer old.Close()
	newMutations := 0
	replacement := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/auth/login" {
			http.SetCookie(w, &http.Cookie{Name: "SID", Value: "new"})
			return
		}
		if r.Header.Get("Cookie") != "SID=new" {
			t.Fatalf("replay cookie=%q", r.Header.Get("Cookie"))
		}
		newMutations++
		w.WriteHeader(http.StatusNoContent)
	}))
	defer replacement.Close()
	provider := &admissionProvider{snapshot: qbSnapshot(old.URL, 1)}
	client := newClient(t, provider, nil)
	result := make(chan error, 1)
	go func() { result <- client.Rename(context.Background(), "hash", "name") }()
	<-firstMutation
	provider.set(qbSnapshot(replacement.URL, 2))

	// When
	close(releaseUnauthorized)
	err := <-result

	// Then
	if err != nil || provider.admissions() != 2 || newMutations != 1 {
		t.Fatalf("err=%v admissions=%d new_mutations=%d", err, provider.admissions(), newMutations)
	}
}

func TestClient_rejectsMutationReplay_whenPublicationFollowsUnauthorized(t *testing.T) {
	// Given
	mutationStarted := make(chan struct{})
	releaseUnauthorized := make(chan struct{})
	oldMutations := 0
	old := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/auth/login" {
			http.SetCookie(w, &http.Cookie{Name: "SID", Value: "old"})
			return
		}
		oldMutations++
		close(mutationStarted)
		<-releaseUnauthorized
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer old.Close()
	newMutations := 0
	replacement := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/auth/login" {
			http.SetCookie(w, &http.Cookie{Name: "SID", Value: "new"})
			return
		}
		newMutations++
	}))
	defer replacement.Close()
	secondAdmission := make(chan struct{})
	releaseAdmission := make(chan struct{})
	provider := &admissionProvider{snapshot: qbSnapshot(old.URL, 1), blockAfterFirst: secondAdmission, releaseAdmission: releaseAdmission}
	client := newClient(t, provider, nil)
	result := make(chan error, 1)
	go func() { result <- client.Rename(context.Background(), "hash", "name") }()
	<-mutationStarted
	provider.set(qbSnapshot(replacement.URL, 2))

	// When
	close(releaseUnauthorized)
	<-secondAdmission
	provider.set(qbSnapshot(replacement.URL, 3))
	close(releaseAdmission)
	err := <-result

	// Then
	if !errors.Is(err, ErrConfigChanged) || oldMutations != 1 || newMutations != 0 {
		t.Fatalf("err=%v old=%d new=%d", err, oldMutations, newMutations)
	}
}
