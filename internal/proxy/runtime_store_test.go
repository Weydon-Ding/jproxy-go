package proxy

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"jproxy-go/internal/runtime"
	"jproxy-go/internal/store/sqlite"
)

type countingStoreLoader struct {
	store *sqlite.Store
	mu    sync.Mutex
	loads int
}

func (l *countingStoreLoader) LoadFormatterSnapshot(ctx context.Context) (sqlite.Snapshot, error) {
	l.mu.Lock()
	l.loads++
	l.mu.Unlock()
	return l.store.LoadFormatterSnapshot(ctx)
}

func (l *countingStoreLoader) count() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.loads
}

func TestServer_usesRealStoreSnapshot_afterRadarrRuleInvalidation(t *testing.T) {
	// Given
	store, loader, initial := runtimeStoreFixture(t)
	defer store.Close()
	counters := map[string]int{}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		counters[r.URL.Path]++
		_, _ = w.Write([]byte(rssWithItems(item("Movie.2024"))))
	}))
	defer upstream.Close()
	server := storeRuntimeServer(upstream.URL, initial, loader)
	radarrOld := warm(t, server, "/radarr/jackett/api?t=search")
	sonarrOld := warm(t, server, "/sonarr/jackett/api?t=search")
	if err := replaceRadarrRule(context.Background(), store, "radarr-new", `.*`); err != nil {
		t.Fatalf("replace Radarr rule: %v", err)
	}

	// When
	if err := server.CacheRegistry().Invalidate(context.Background(), runtime.RadarrRule); err != nil {
		t.Fatalf("Invalidate() error = %v", err)
	}
	radarrNew := warm(t, server, "/radarr/jackett/api?t=search")
	sonarrNew := warm(t, server, "/sonarr/jackett/api?t=search")

	// Then
	if !contains(radarrOld, "radarr-old") || !contains(radarrNew, "radarr-new") || sonarrNew != sonarrOld || counters["/api"] != 3 || loader.count() != 2 {
		t.Fatalf("old=%q new=%q sonarr=%q calls=%v loads=%d", radarrOld, radarrNew, sonarrNew, counters, loader.count())
	}
	t.Logf("task5_http_qa invalidation=%s radarr_old_hash=%s radarr_new_hash=%s sonarr_hash=%s sonarr_unchanged=%t upstream_count=%d loader_count=%d", runtime.RadarrRule, evidenceHash(radarrOld), evidenceHash(radarrNew), evidenceHash(sonarrNew), sonarrNew == sonarrOld, counters["/api"], loader.count())
}

func TestServer_retainsLastKnownGood_whenRealStoreRefreshIsMalformed_thenRetries(t *testing.T) {
	// Given
	store, loader, initial := runtimeStoreFixture(t)
	defer store.Close()
	calls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		_, _ = w.Write([]byte(rssWithItems(item("Movie.2024"))))
	}))
	defer upstream.Close()
	server := storeRuntimeServer(upstream.URL, initial, loader)
	old := warm(t, server, "/radarr/jackett/api?t=search")
	before := server.provider.Snapshot()
	if err := replaceRadarrRule(context.Background(), store, "broken", "["); err != nil {
		t.Fatalf("replace malformed rule: %v", err)
	}

	// When
	err := server.CacheRegistry().Invalidate(context.Background(), runtime.RadarrRule, runtime.IndexerResult)
	afterFailure := warm(t, server, "/radarr/jackett/api?t=search")
	failureRevision := server.provider.Snapshot().RadarrRevision
	if repairErr := replaceRadarrRule(context.Background(), store, "repaired", `.*`); repairErr != nil {
		t.Fatalf("repair rule: %v", repairErr)
	}
	retryErr := server.CacheRegistry().Invalidate(context.Background(), runtime.RadarrRule)
	afterRetry := warm(t, server, "/radarr/jackett/api?t=search")

	// Then
	after := server.provider.Snapshot()
	if err == nil || before.RadarrRevision != failureRevision || afterFailure != old || retryErr != nil || !contains(afterRetry, "repaired") || after.RadarrRevision != before.RadarrRevision+1 || calls != 2 || loader.count() != 3 {
		t.Fatalf("refresh=%v retry=%v old=%q failed=%q retried=%q calls=%d loads=%d revisions=%d/%d", err, retryErr, old, afterFailure, afterRetry, calls, loader.count(), before.RadarrRevision, after.RadarrRevision)
	}
	t.Logf("task5_http_qa lkg_retry=true initial_hash=%s failed_hash=%s retry_hash=%s calls=%d loader_count=%d revision_before=%d revision_after=%d", evidenceHash(old), evidenceHash(afterFailure), evidenceHash(afterRetry), calls, loader.count(), before.RadarrRevision, after.RadarrRevision)
}

func evidenceHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return fmt.Sprintf("sha256:%x", sum[:8])
}

func runtimeStoreFixture(t *testing.T) (*sqlite.Store, *countingStoreLoader, sqlite.Snapshot) {
	t.Helper()
	store, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "runtime.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := seedRuntimeStore(context.Background(), store); err != nil {
		store.Close()
		t.Fatalf("seed store: %v", err)
	}
	loader := &countingStoreLoader{store: store}
	initial, err := loader.LoadFormatterSnapshot(context.Background())
	if err != nil {
		store.Close()
		t.Fatalf("initial snapshot: %v", err)
	}
	return store, loader, initial
}

func seedRuntimeStore(ctx context.Context, store *sqlite.Store) error {
	repos := store.Repositories()
	radarrFormat, sonarrFormat, clean := "{title}", "{title}", ""
	if err := repos.SystemConfigs.UpsertBatch(ctx, []sqlite.SystemConfig{{ID: 1, Key: "radarrIndexerFormat", Value: &radarrFormat, ValidStatus: sqlite.Valid}, {ID: 2, Key: "sonarrIndexerFormat", Value: &sonarrFormat, ValidStatus: sqlite.Valid}, {ID: 3, Key: "cleanTitleRegex", Value: &clean, ValidStatus: sqlite.Valid}}); err != nil {
		return err
	}
	if err := replaceRadarrRule(ctx, store, "radarr-old", `.*`); err != nil {
		return err
	}
	return repos.SonarrRules.Upsert(ctx, sqlite.SonarrRule{ID: "sonarr", Token: "title", Regex: `.*`, Replacement: "sonarr", ValidStatus: sqlite.Valid})
}

func replaceRadarrRule(ctx context.Context, store *sqlite.Store, replacement, expression string) error {
	return store.Repositories().RadarrRules.Replace(ctx, sqlite.RadarrRuleBatch{Rows: []sqlite.RadarrRule{{ID: "radarr", Token: "title", Regex: expression, Replacement: replacement, ValidStatus: sqlite.Valid}}})
}

func storeRuntimeServer(url string, initial sqlite.Snapshot, loader runtime.Loader) *Server {
	cfg := testConfig(url, url)
	cfg.RadarrFormatting.Enabled = true
	cfg.SonarrFormatting.Enabled = true
	cfg.IndexerResultCacheTTL = time.Minute
	cfg.OffsetCacheTTL = time.Minute
	cfg.ResultCacheMaxEntries = 10
	cfg.OffsetCacheMaxEntries = 10
	return NewServerWithRuntime(cfg, RuntimeOptions{Provider: runtime.NewProvider(initial, loader)})
}
