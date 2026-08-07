package runtime

import (
	"errors"
	"testing"
)

func TestQBittorrentMutationAdmission_startsCurrentRevisionExactlyOnce(t *testing.T) {
	// Given
	provider := NewStaticProvider(testSnapshot("current"))
	admission, available := provider.(QBittorrentMutationAdmission)
	if !available {
		t.Fatal("provider does not expose QBittorrentMutationAdmission")
	}
	revision := provider.Snapshot().QBittorrentRevision
	starts := 0

	// When
	err := admission.AdmitQBittorrentMutation(revision, func() { starts++ })

	// Then
	if err != nil || starts != 1 {
		t.Fatalf("err=%v starts=%d", err, starts)
	}
}

func TestQBittorrentMutationAdmission_rejectsRevisionAfterPreparedPublication(t *testing.T) {
	// Given
	provider := NewStaticProvider(testSnapshot("old"))
	admission, available := provider.(QBittorrentMutationAdmission)
	if !available {
		t.Fatal("provider does not expose QBittorrentMutationAdmission")
	}
	staleRevision := provider.Snapshot().QBittorrentRevision

	// When
	if !PublishPrepared(provider, testSnapshot("new")) {
		t.Fatal("PublishPrepared() = false")
	}
	starts := 0
	err := admission.AdmitQBittorrentMutation(staleRevision, func() { starts++ })

	// Then
	if !errors.Is(err, ErrQBittorrentRevisionStale) || starts != 0 {
		t.Fatalf("err=%v starts=%d", err, starts)
	}
}

func TestQBittorrentMutationAdmission_serializesPublicationWithRequestStart(t *testing.T) {
	// Given
	provider := NewStaticProvider(testSnapshot("old"))
	admission := provider.(QBittorrentMutationAdmission)
	started := make(chan struct{})
	releaseStart := make(chan struct{})
	admissionFinished := make(chan error, 1)
	go func() {
		admissionFinished <- admission.AdmitQBittorrentMutation(provider.Snapshot().QBittorrentRevision, func() {
			close(started)
			<-releaseStart
		})
	}()
	<-started
	published := make(chan bool, 1)
	go func() { published <- PublishPrepared(provider, testSnapshot("new")) }()

	// When
	select {
	case <-published:
		t.Fatal("publication completed before request start finished")
	default:
	}
	close(releaseStart)

	// Then
	if err := <-admissionFinished; err != nil {
		t.Fatal(err)
	}
	if ok := <-published; !ok {
		t.Fatal("PublishPrepared() = false")
	}
}
