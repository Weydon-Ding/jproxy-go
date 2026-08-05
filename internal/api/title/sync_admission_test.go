package title

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"jproxy-go/internal/runtime"
)

func TestHandlerSyncAdmission_rejectsConcurrentDomainWithoutCallingSyncer(t *testing.T) {
	// Given
	fixture := newSyncFixture()
	started := make(chan struct{})
	release := make(chan struct{})
	finished := make(chan struct{})
	syncer := &blockingSyncer{started: started, release: release, finished: finished}
	options := fixture.optionsWithSyncers(syncer, fixture.radarr, fixture.tmdb)
	options.Admission = fixture.registry
	handler := NewHandler(options)
	first := httptest.NewRecorder()
	go handler.ServeHTTP(first, httptest.NewRequest(http.MethodPost, "/api/sonarr/title/sync", nil))
	<-started

	// When
	second := request(handler, http.MethodPost, "/api/sonarr/title/sync", nil)
	close(release)
	<-finished

	// Then
	if second.Code != http.StatusBadRequest || syncer.calls != 1 {
		t.Fatalf("status=%d calls=%d", second.Code, syncer.calls)
	}
}

func TestHandlerSyncAdmission_releasesFailuresAndRetainsSuccessfulPublicationFailure(t *testing.T) {
	// Given
	fixture := newSyncFixture()
	syncer := &scriptedSyncer{results: []syncOutcome{{err: context.Canceled}, {result: SyncSucceeded}}}
	options := fixture.optionsWithSyncers(syncer, fixture.radarr, fixture.tmdb)
	options.Admission = fixture.registry
	fixture.provider.err = errors.New("publication failed")
	handler := NewHandler(options)

	// When
	cancelled := request(handler, http.MethodPost, "/api/sonarr/title/sync", nil)
	firstSuccess := request(handler, http.MethodPost, "/api/sonarr/title/sync", nil)
	fixture.provider.err = nil
	rejected := request(handler, http.MethodPost, "/api/sonarr/title/sync", nil)

	// Then
	if cancelled.Code != http.StatusInternalServerError || firstSuccess.Code != http.StatusInternalServerError || rejected.Code != http.StatusBadRequest || syncer.calls != 2 || !fixture.markers.deletedExactly(runtime.SonarrTitleSyncInterval) {
		t.Fatalf("statuses=%d/%d/%d calls=%d deleted=%v", cancelled.Code, firstSuccess.Code, rejected.Code, syncer.calls, fixture.markers.deleted)
	}
}

func TestHandlerSyncAdmission_releasesUnavailableAndLegacyTooFrequent(t *testing.T) {
	// Given
	fixture := newSyncFixture()
	syncer := &scriptedSyncer{results: []syncOutcome{{err: ErrSyncUnavailable}, {result: SyncTooFrequent}, {result: SyncSucceeded}}}
	options := fixture.optionsWithSyncers(fixture.sonarr, fixture.radarr, syncer)
	options.Admission = fixture.registry
	handler := NewHandler(options)

	// When
	unavailable := request(handler, http.MethodPost, "/api/tmdb/title/sync", nil)
	tooFrequent := request(handler, http.MethodPost, "/api/tmdb/title/sync", nil)
	success := request(handler, http.MethodPost, "/api/tmdb/title/sync", nil)

	// Then
	if unavailable.Code != http.StatusServiceUnavailable || tooFrequent.Code != http.StatusBadRequest || success.Code != http.StatusOK || syncer.calls != 3 || fixture.markers.deleteCount() != 0 {
		t.Fatalf("statuses=%d/%d/%d calls=%d deleted=%v", unavailable.Code, tooFrequent.Code, success.Code, syncer.calls, fixture.markers.deleted)
	}
}

type blockingSyncer struct {
	started  chan<- struct{}
	release  <-chan struct{}
	finished chan<- struct{}
	calls    int
}

func (s *blockingSyncer) Sync(context.Context) (SyncResult, error) {
	s.calls++
	s.started <- struct{}{}
	<-s.release
	close(s.finished)
	return SyncSucceeded, nil
}
