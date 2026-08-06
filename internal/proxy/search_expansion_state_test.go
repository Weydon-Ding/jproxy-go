package proxy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"jproxy-go/internal/runtime"
	"jproxy-go/internal/search"
)

func TestProxy_searchCandidateRevisionsIsolateResultAndOffsetState_whenCountsMatch(t *testing.T) {
	// Given
	provider := &candidateSnapshotProvider{snapshot: runtime.Snapshot{SonarrRevision: 1, SonarrSearchRevision: 1, SonarrCandidates: []search.Candidate{{Query: "Show", MainTitle: "Show", SeasonNumber: 1}, {Query: "Old", MainTitle: "Show", SeasonNumber: 1}}}}
	var queries []string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("q")
		queries = append(queries, query)
		if query == "Show 01" {
			_, _ = w.Write([]byte(emptyRSS))
			return
		}
		_, _ = w.Write([]byte(rssWithItems(item(query))))
	}))
	defer upstream.Close()
	provider.snapshot.JackettURL = upstream.URL
	cfg := testConfig(upstream.URL, upstream.URL)
	cfg.MinCount = 0
	cfg.ResultCacheMaxEntries = 10
	cfg.OffsetCacheMaxEntries = 10
	handler := NewServerWithRuntime(cfg, RuntimeOptions{Provider: provider}).Routes()

	// When
	serveProxyRequest(t, handler, "/sonarr/jackett/api?t=tvsearch&q=Show+01&limit=1&offset=0")
	provider.set(runtime.Snapshot{JackettURL: upstream.URL, SonarrRevision: 2, SonarrSearchRevision: 2, SonarrCandidates: []search.Candidate{{Query: "Show", MainTitle: "Show", SeasonNumber: 1}, {Query: "New", MainTitle: "Show", SeasonNumber: 1}}})
	serveProxyRequest(t, handler, "/sonarr/jackett/api?t=tvsearch&q=Show+01&limit=1&offset=0")
	serveProxyRequest(t, handler, "/sonarr/jackett/api?t=tvsearch&q=Show+01&limit=1&offset=1")

	// Then
	want := []string{"Show 01", "Old 01", "Show 01", "New 01", "New 01"}
	if len(queries) != len(want) {
		t.Fatalf("queries = %v, want %v", queries, want)
	}
	for index := range want {
		if queries[index] != want[index] {
			t.Fatalf("query %d = %q, want %q; all=%v", index, queries[index], want[index], queries)
		}
	}
}

func TestProxy_keepsGlobalOffsetContinuityAcrossDeduplicatedCandidates(t *testing.T) {
	// Given
	queries := captureExpansionQueries(t, runtime.Snapshot{SonarrSearchRevision: 1, SonarrCandidates: []search.Candidate{
		{Query: "Show", MainTitle: "Show", SeasonNumber: 1},
		{Query: "Alias", MainTitle: "Show", SeasonNumber: 1},
		{Query: "Alias", MainTitle: "Show", SeasonNumber: 1},
	}})

	// When
	queries.request(t, "/sonarr/jackett/api?t=tvsearch&q=Show+01&limit=2&offset=0")
	queries.request(t, "/sonarr/jackett/api?t=tvsearch&q=Show+01&limit=2&offset=2")

	// Then
	wantQueries := []string{"Show 01", "Alias 01", "Alias 01"}
	wantOffsets := []string{"0", "0", "1"}
	if len(queries.values) != len(wantQueries) {
		t.Fatalf("queries=%v offsets=%v", queries.values, queries.offsets)
	}
	for index := range wantQueries {
		if queries.values[index] != wantQueries[index] || queries.offsets[index] != wantOffsets[index] {
			t.Fatalf("request %d query=%q offset=%q, want query=%q offset=%q", index, queries.values[index], queries.offsets[index], wantQueries[index], wantOffsets[index])
		}
	}
}

func TestProxy_keepsCandidateSearchStateIsolatedByKind(t *testing.T) {
	// Given
	provider := &candidateSnapshotProvider{snapshot: runtime.Snapshot{SonarrRevision: 1, SonarrSearchRevision: 1, RadarrRevision: 1, RadarrSearchRevision: 1, SonarrCandidates: []search.Candidate{{Query: "Show", MainTitle: "Show", SeasonNumber: 1}, {Query: "Show Alias", MainTitle: "Show", SeasonNumber: 1}}, RadarrCandidates: []search.Candidate{{Query: "Movie", MainTitle: "Movie", Year: 2024}, {Query: "Movie Alias", MainTitle: "Movie", Year: 2024}}}}
	var sonarrQueries, radarrQueries []string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/sonarr" {
			sonarrQueries = append(sonarrQueries, r.URL.Query().Get("q"))
		} else {
			radarrQueries = append(radarrQueries, r.URL.Query().Get("q"))
		}
		_, _ = w.Write([]byte(rssWithItems(item(r.URL.Query().Get("q")))))
	}))
	defer upstream.Close()
	provider.snapshot.JackettURL = upstream.URL
	handler := NewServerWithRuntime(testConfig(upstream.URL, upstream.URL), RuntimeOptions{Provider: provider}).Routes()

	// When
	serveProxyRequest(t, handler, "/sonarr/jackett/sonarr?t=tvsearch&q=Show+01&limit=10")
	serveProxyRequest(t, handler, "/radarr/jackett/radarr?t=movie&q=Movie+2024&limit=10")

	// Then
	if len(sonarrQueries) != 2 || sonarrQueries[1] != "Show Alias 01" || len(radarrQueries) != 3 || radarrQueries[2] != "Movie Alias" {
		t.Fatalf("sonarr=%v radarr=%v", sonarrQueries, radarrQueries)
	}
}

func TestProxy_keepsRadarrCandidateOffsetState_whenSonarrCandidateRevisionChanges(t *testing.T) {
	// Given
	provider := &candidateSnapshotProvider{snapshot: runtime.Snapshot{SonarrRevision: 1, SonarrSearchRevision: 1, RadarrRevision: 1, RadarrSearchRevision: 1, RadarrCandidates: []search.Candidate{{Query: "Movie", MainTitle: "Movie", Year: 2024}, {Query: "Alias", MainTitle: "Movie", Year: 2024}}}}
	var queries []string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		queries = append(queries, r.URL.Query().Encode())
		_, _ = w.Write([]byte(rssWithItems(item(r.URL.Query().Get("q")))))
	}))
	defer upstream.Close()
	provider.snapshot.JackettURL = upstream.URL
	cfg := testConfig(upstream.URL, upstream.URL)
	cfg.MinCount = 0
	cfg.ResultCacheMaxEntries = 10
	cfg.OffsetCacheMaxEntries = 10
	handler := NewServerWithRuntime(cfg, RuntimeOptions{Provider: provider}).Routes()
	serveProxyRequest(t, handler, "/radarr/jackett/api?t=movie&q=Movie+2024&limit=2&offset=0")
	provider.set(runtime.Snapshot{JackettURL: upstream.URL, SonarrRevision: 2, SonarrSearchRevision: 2, RadarrRevision: 1, RadarrSearchRevision: 1, RadarrCandidates: provider.snapshot.RadarrCandidates})

	// When
	serveProxyRequest(t, handler, "/radarr/jackett/api?t=movie&q=Movie+2024&limit=2&offset=2")

	// Then
	if len(queries) != 4 || queries[2] != "limit=2&offset=1&q=Movie&t=movie" || queries[3] != "limit=2&offset=0&q=Alias&t=movie" {
		t.Fatalf("queries=%v", queries)
	}
}

func TestProxy_retriesExpandedSearchAfterMidSequenceUpstreamFailure(t *testing.T) {
	// Given
	provider := baselineSnapshotProvider{snapshot: runtime.Snapshot{SonarrCandidates: []search.Candidate{{Query: "Show", MainTitle: "Show", SeasonNumber: 1}, {Query: "Alias", MainTitle: "Show", SeasonNumber: 1}}}}
	calls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(rssWithItems(item(r.URL.Query().Get("q")))))
	}))
	defer upstream.Close()
	provider.snapshot.JackettURL = upstream.URL
	handler := NewServerWithRuntime(testConfig(upstream.URL, upstream.URL), RuntimeOptions{Provider: provider}).Routes()

	// When
	first := httptest.NewRecorder()
	handler.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/sonarr/jackett/api?t=tvsearch&q=Show+01&limit=10", nil))
	second := httptest.NewRecorder()
	handler.ServeHTTP(second, httptest.NewRequest(http.MethodGet, "/sonarr/jackett/api?t=tvsearch&q=Show+01&limit=10", nil))

	// Then
	if first.Code != http.StatusBadGateway || second.Code != http.StatusOK || calls != 4 || countItems(second.Body.String()) != 2 {
		t.Fatalf("first=%d second=%d calls=%d xml=%q", first.Code, second.Code, calls, second.Body.String())
	}
}

type candidateSnapshotProvider struct {
	mu       sync.RWMutex
	snapshot runtime.Snapshot
}

func (provider *candidateSnapshotProvider) Snapshot() runtime.Snapshot {
	provider.mu.RLock()
	defer provider.mu.RUnlock()
	return provider.snapshot
}

func (provider *candidateSnapshotProvider) Refresh(context.Context, runtime.Scope) error { return nil }

func (provider *candidateSnapshotProvider) set(snapshot runtime.Snapshot) {
	provider.mu.Lock()
	defer provider.mu.Unlock()
	provider.snapshot = snapshot
}

func serveProxyRequest(t *testing.T, handler http.Handler, path string) {
	t.Helper()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%q", recorder.Code, recorder.Body.String())
	}
}
