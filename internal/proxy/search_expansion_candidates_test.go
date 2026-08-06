package proxy

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"jproxy-go/internal/runtime"
	"jproxy-go/internal/search"
)

func TestProxy_expandsSonarrSearchWithPublishedCandidates_whenExactAnchorMatches(t *testing.T) {
	// Given
	queries := captureExpansionQueries(t, runtime.Snapshot{SonarrCandidates: []search.Candidate{
		{Query: "Show S01", MainTitle: "Show", SeasonNumber: 1},
		{Query: "Show Alias", MainTitle: "Show", SeasonNumber: -1},
		{Query: "Show Season One", MainTitle: "Show", SeasonNumber: 1},
		{Query: "Show Alias", MainTitle: "Show", SeasonNumber: -1},
		{Query: "Other Season", MainTitle: "Show", SeasonNumber: 2},
		{Query: "Other Show", MainTitle: "Other", SeasonNumber: 1},
	}})

	// When
	queries.request(t, "/sonarr/jackett/api?t=tvsearch&cat=5000&season=1&ep=2&q=Show+S01+02&limit=10")

	// Then
	want := []string{"Show S01 02", "Show Alias 02", "Show Season One 02"}
	if !reflect.DeepEqual(queries.values, want) {
		t.Fatalf("queries = %v, want %v", queries.values, want)
	}
	if !reflect.DeepEqual(queries.seasons, []string{"1", "1", "1"}) || !reflect.DeepEqual(queries.episodes, []string{"2", "2", "2"}) || !reflect.DeepEqual(queries.categories, []string{"5000", "5000", "5000"}) {
		t.Fatalf("preserved params: seasons=%v episodes=%v categories=%v", queries.seasons, queries.episodes, queries.categories)
	}
}

func TestProxy_expandsRadarrSearchWithPublishedCandidates_whenExactBaseAnchorMatches(t *testing.T) {
	// Given
	queries := captureExpansionQueries(t, runtime.Snapshot{RadarrCandidates: []search.Candidate{
		{Query: "Movie", MainTitle: "Movie", Year: 2023},
		{Query: "Movie", MainTitle: "Movie", Year: 2024},
		{Query: "Movie Alias 2024", MainTitle: "Movie", Year: 2024},
		{Query: "Movie Alias", MainTitle: "Movie", Year: 2024},
		{Query: "Movie Alias", MainTitle: "Movie", Year: 2024},
		{Query: "Other Year", MainTitle: "Movie", Year: 2023},
		{Query: "Other Movie", MainTitle: "Other", Year: 2024},
	}})

	// When
	queries.request(t, "/radarr/jackett/api?t=movie&cat=2000&q=Movie+2024&limit=10")

	// Then
	want := []string{"Movie 2024", "Movie", "Movie Alias 2024", "Movie Alias"}
	if !reflect.DeepEqual(queries.values, want) {
		t.Fatalf("queries = %v, want %v", queries.values, want)
	}
}

func TestProxy_doesNotGroupCandidates_whenExactAnchorHasEmptyMainTitle(t *testing.T) {
	// Given
	queries := captureExpansionQueries(t, runtime.Snapshot{SonarrCandidates: []search.Candidate{
		{Query: "Exact", SeasonNumber: 1},
		{Query: "Unrelated", SeasonNumber: 1},
	}})

	// When
	queries.request(t, "/sonarr/jackett/api?t=tvsearch&q=Exact+01&limit=10")

	// Then
	if !reflect.DeepEqual(queries.values, []string{"Exact 01"}) {
		t.Fatalf("queries = %v, want exact basic query only", queries.values)
	}
}

type expansionQueries struct {
	values     []string
	seasons    []string
	episodes   []string
	categories []string
	offsets    []string
	handler    http.Handler
}

func captureExpansionQueries(t *testing.T, snapshot runtime.Snapshot) *expansionQueries {
	t.Helper()
	queries := &expansionQueries{}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		queries.values = append(queries.values, r.URL.Query().Get("q"))
		queries.seasons = append(queries.seasons, r.URL.Query().Get("season"))
		queries.episodes = append(queries.episodes, r.URL.Query().Get("ep"))
		queries.categories = append(queries.categories, r.URL.Query().Get("cat"))
		queries.offsets = append(queries.offsets, r.URL.Query().Get("offset"))
		_, _ = w.Write([]byte(rssWithItems(item(r.URL.Query().Get("q")))))
	}))
	t.Cleanup(upstream.Close)
	snapshot.JackettURL = upstream.URL
	cfg := testConfig(upstream.URL, upstream.URL)
	cfg.MinCount = 0
	cfg.ResultCacheMaxEntries = 10
	cfg.OffsetCacheMaxEntries = 10
	queries.handler = NewServerWithRuntime(cfg, RuntimeOptions{Provider: baselineSnapshotProvider{snapshot: snapshot}}).Routes()
	return queries
}

func (queries *expansionQueries) request(t *testing.T, path string) {
	t.Helper()
	recorder := httptest.NewRecorder()
	queries.handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%q", recorder.Code, recorder.Body.String())
	}
}
