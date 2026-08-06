package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"jproxy-go/internal/api/system"
	"jproxy-go/internal/runtime"
	"jproxy-go/internal/store/sqlite"
	rulesync "jproxy-go/internal/sync/rule"
)

func TestRootHandler_syncsRulesWithDynamicAuthorsAndAuthorFailures(t *testing.T) {
	// Given
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "rule-sync.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	seedRootConfigs(t, store)
	var mu sync.Mutex
	primaryCalls, backupCalls := 0, 0
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		primaryCalls++
		mu.Unlock()
		w.WriteHeader(http.StatusBadGateway)
	}))
	backup := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		backupCalls++
		mu.Unlock()
		switch r.URL.Path {
		case "/author.json":
			_, _ = w.Write([]byte(`["good"]`))
		case "/sonarr@good.json", "/radarr@good.json":
			_, _ = w.Write([]byte(ruleSyncPayload("good", "good-rule")))
		case "/sonarr@bad.json":
			_, _ = w.Write([]byte(`[{"id":"bad","token":"title","priority":1,"regex":"[","replacement":"x","offset":0,"example":"x","remark":"remark","author":"bad","validStatus":1}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(primary.Close)
	t.Cleanup(backup.Close)
	snapshot, err := store.FormatterSnapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	provider := runtime.NewProvider(snapshot, store)
	initial := provider.Snapshot()
	client := &http.Client{Timeout: time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	defaultPrimary, defaultBackup := system.DefaultRuleSyncSources()
	dependencies := ruleSyncDependencies{
		sonarr: liveRuleSyncer{config: ruleSyncConfigSource{store}, store: store, domain: rulesync.Sonarr, client: client, timeout: time.Second, primary: primary.URL, backup: backup.URL, invalidate: func(ctx context.Context, name string) error { return registryInvalidate(ctx, provider, name) }},
		radarr: liveRuleSyncer{config: ruleSyncConfigSource{store}, store: store, domain: rulesync.Radarr, client: client, timeout: time.Second, primary: primary.URL, backup: backup.URL, invalidate: func(ctx context.Context, name string) error { return registryInvalidate(ctx, provider, name) }},
	}
	handler := rootHandlerWithSyncDependencies(rootRouteConfig(true), provider, store, managementSyncDependencies{rule: dependencies})

	// When
	first := httptest.NewRecorder()
	handler.ServeHTTP(first, httptest.NewRequest(http.MethodPost, "/api/sonarr/rule/sync", nil))
	beforeFailure := provider.Snapshot().SonarrRevision
	setRuleSyncAuthors(t, store, "good,bad")
	partial := httptest.NewRecorder()
	handler.ServeHTTP(partial, httptest.NewRequest(http.MethodPost, "/api/sonarr/rule/sync", nil))
	setRuleSyncAuthors(t, store, "ALL")
	radarr := httptest.NewRecorder()
	handler.ServeHTTP(radarr, httptest.NewRequest(http.MethodPost, "/api/radarr/rule/sync", nil))
	rows, queryErr := store.Repositories().SonarrRules.Page(ctx, sqlite.RuleFilter{})

	// Then
	mu.Lock()
	gotPrimaryCalls, gotBackupCalls := primaryCalls, backupCalls
	mu.Unlock()
	calls := gotPrimaryCalls + gotBackupCalls
	redacted := !strings.Contains(partial.Body.String(), "good") && !strings.Contains(partial.Body.String(), "bad")
	current := provider.Snapshot()
	defaultsFixed := defaultPrimary != "" && defaultBackup != ""
	if first.Code != http.StatusOK || partial.Code != http.StatusInternalServerError || radarr.Code != http.StatusOK || queryErr != nil || rows.Total != 1 || beforeFailure != initial.SonarrRevision+1 || current.SonarrRevision != beforeFailure+1 || current.RadarrRevision != initial.RadarrRevision+1 || gotPrimaryCalls != 6 || gotBackupCalls != 6 || calls != 12 || !redacted || !defaultsFixed {
		t.Fatalf("status=%d/%d/%d rows=%d revisions=%d/%d calls=%d err=%v", first.Code, partial.Code, radarr.Code, rows.Total, provider.Snapshot().SonarrRevision, provider.Snapshot().RadarrRevision, calls, queryErr)
	}
}

func registryInvalidate(ctx context.Context, provider runtime.Provider, name string) error {
	return runtime.NewRegistry(provider, nilCache{}, nilCache{}, nilCache{}).Invalidate(ctx, name)
}

type nilCache struct{}

func (nilCache) Clear()        {}
func (nilCache) Delete(string) {}

func setRuleSyncAuthors(t *testing.T, store *sqlite.Store, value string) {
	t.Helper()
	rows, err := store.Repositories().SystemConfigs.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for index := range rows {
		if rows[index].Key == "ruleSyncAuthors" {
			rows[index].Value = &value
		}
	}
	if _, err := store.UpdateSystemConfigs(context.Background(), rows); err != nil {
		t.Fatal(err)
	}
}

func ruleSyncPayload(author, id string) string {
	return `[{"id":"` + id + `","token":"title","priority":1,"regex":"^x$","replacement":"y","offset":0,"example":"x","remark":"remark","author":"` + author + `","validStatus":1}]`
}
