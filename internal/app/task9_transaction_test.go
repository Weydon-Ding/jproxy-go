package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"jproxy-go/internal/config"
	"jproxy-go/internal/runtime"
	"jproxy-go/internal/store/sqlite"
)

func TestTask9_rootMuxReleasesAdmission_whenSyncRequestIsCanceled(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "task9-cancel.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	seedRootConfigs(t, store)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(task9SonarrJSON)) }))
	t.Cleanup(upstream.Close)
	snapshot, err := store.UpdateSystemConfigs(ctx, task9Configs(t, store, upstream.URL, "cancel-key"))
	if err != nil {
		t.Fatal(err)
	}
	handler := rootHandler(config.Config{Database: config.DatabaseConfig{Enabled: true}, HTTPTimeout: time.Second}, runtime.NewProvider(snapshot, store), store)
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/sonarr/title/sync", nil).WithContext(canceled))
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("canceled status=%d", response.Code)
	}
	retry := httptest.NewRecorder()
	handler.ServeHTTP(retry, httptest.NewRequest(http.MethodPost, "/api/sonarr/title/sync", nil))
	if retry.Code != http.StatusOK || len(task9SonarrRows(t, store)) != 3 {
		t.Fatalf("retry status=%d rows=%d", retry.Code, len(task9SonarrRows(t, store)))
	}
	t.Logf("task9_cancellation canceled_status=%d retry_status=%d admission_released=%t", response.Code, retry.Code, retry.Code == http.StatusOK)
}

func TestTask9_rootMuxRetainsSuccessMarker_whenRefreshFailsAfterCommit(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "task9-refresh.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	seedRootConfigs(t, store)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(task9SonarrJSON)) }))
	t.Cleanup(upstream.Close)
	snapshot, err := store.UpdateSystemConfigs(ctx, task9Configs(t, store, upstream.URL, "refresh-key"))
	if err != nil {
		t.Fatal(err)
	}
	provider := &controllableProvider{Provider: runtime.NewProvider(snapshot, store)}
	provider.fail.Store(true)
	handler := rootHandler(config.Config{Database: config.DatabaseConfig{Enabled: true}, HTTPTimeout: time.Second}, provider, store)
	before := provider.Snapshot()
	first := httptest.NewRecorder()
	handler.ServeHTTP(first, httptest.NewRequest(http.MethodPost, "/api/sonarr/title/sync", nil))
	after := provider.Snapshot()
	second := httptest.NewRecorder()
	handler.ServeHTTP(second, httptest.NewRequest(http.MethodPost, "/api/sonarr/title/sync", nil))
	if first.Code != http.StatusInternalServerError || second.Code != http.StatusBadRequest || len(task9SonarrRows(t, store)) != 3 || !reflect.DeepEqual(before, after) || provider.refreshes.Load() != 1 {
		t.Fatalf("statuses=%d/%d rows=%d refreshes=%d", first.Code, second.Code, len(task9SonarrRows(t, store)), provider.refreshes.Load())
	}
	t.Logf("task9_refresh_failure first_status=%d second_status=%d db_committed=%t runtime_retained=%t marker_retained=%t refresh_count=%d", first.Code, second.Code, len(task9SonarrRows(t, store)) == 3, reflect.DeepEqual(before, after), second.Code == http.StatusBadRequest, provider.refreshes.Load())
}
