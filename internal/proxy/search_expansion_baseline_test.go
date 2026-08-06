package proxy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"jproxy-go/internal/runtime"
	"jproxy-go/internal/search"
)

func TestProxy_searchExpansionFallsBackToBasicTitles_whenRuntimeCandidatesDoNotApply(t *testing.T) {
	tests := []struct {
		name            string
		path            string
		snapshot        runtime.Snapshot
		wantQueryString []string
	}{
		{
			name: "sonarr candidates are empty",
			path: "/sonarr/jackett/api?t=tvsearch&cat=5000&season=1&ep=12&apikey=secret&q=Series+Name+12",
			wantQueryString: []string{
				"apikey=secret&cat=5000&ep=12&q=Series+Name+12&season=1&t=tvsearch",
			},
		},
		{
			name: "radarr candidates are empty",
			path: "/radarr/jackett/api?t=movie&cat=2000&apikey=secret&q=Movie+Title+2024",
			wantQueryString: []string{
				"apikey=secret&cat=2000&q=Movie+Title+2024&t=movie",
				"apikey=secret&cat=2000&offset=0&q=Movie+Title&t=movie",
			},
		},
		{
			name: "snapshot candidates do not match sonarr search",
			path: "/sonarr/jackett/api?t=tvsearch&cat=5000&season=2&ep=4&apikey=secret&q=Known+Show+04",
			snapshot: runtime.Snapshot{
				SonarrCandidates: []search.Candidate{{Query: "Different Show", MainTitle: "Different Show", SeasonNumber: 2}},
				RadarrCandidates: []search.Candidate{{Query: "Different Movie", MainTitle: "Different Movie", Year: 2024}},
			},
			wantQueryString: []string{
				"apikey=secret&cat=5000&ep=4&q=Known+Show+04&season=2&t=tvsearch",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given
			var gotQueryString []string
			upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				gotQueryString = append(gotQueryString, request.URL.Query().Encode())
				_, _ = writer.Write([]byte(emptyRSS))
			}))
			defer upstream.Close()

			test.snapshot.JackettURL = upstream.URL
			handler := NewServerWithRuntime(testConfig(upstream.URL, upstream.URL), RuntimeOptions{Provider: baselineSnapshotProvider{snapshot: test.snapshot}}).Routes()

			// When
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, test.path, nil))

			// Then
			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body=%q", recorder.Code, recorder.Body.String())
			}
			if len(gotQueryString) != len(test.wantQueryString) {
				t.Fatalf("upstream queries = %v, want %v", gotQueryString, test.wantQueryString)
			}
			for index := range test.wantQueryString {
				if gotQueryString[index] != test.wantQueryString[index] {
					t.Fatalf("upstream query %d = %q, want %q", index, gotQueryString[index], test.wantQueryString[index])
				}
			}
		})
	}
}

type baselineSnapshotProvider struct{ snapshot runtime.Snapshot }

func (provider baselineSnapshotProvider) Snapshot() runtime.Snapshot { return provider.snapshot }

func (baselineSnapshotProvider) Refresh(context.Context, runtime.Scope) error { return nil }
