package rule

import (
	"context"
	"jproxy-go/internal/store/sqlite"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuleRoutes_whenMethodsMatch(t *testing.T) {
	s, e := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "x.db"))
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _ = s.Close() })
	for _, d := range []string{"sonarr", "radarr"} {
		h := NewHandler(Options{Store: s, Domain: d})
		for _, p := range []string{"sync", "query", "save", "remove", "enable", "disable", "export", "import", "token/list"} {
			m := "POST"
			body := "[]"
			if p == "query" || p == "token/list" {
				m = "GET"
			}
			if p == "save" {
				body = `{"token":"t","regex":"x","replacement":"x","example":"x"}`
			}
			r := httptest.NewRecorder()
			h.ServeHTTP(r, httptest.NewRequest(m, "/api/"+d+"/rule/"+p, strings.NewReader(body)))
			if r.Code == 404 || r.Code == 405 {
				t.Fatalf("%s %s %d", d, p, r.Code)
			}
		}
	}
	r := httptest.NewRecorder()
	NewTestHandler().ServeHTTP(r, httptest.NewRequest("GET", "/api/rule/test?regex=(%5Cd%2B)&replacement=$1&offset=1&example=E01", nil))
	if r.Code != 200 {
		t.Fatal(r.Code)
	}
}

func TestRuleRoutes_whenPathOnlyPartiallyMatchesKnownRoute(t *testing.T) {
	store, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "x.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	handler := NewHandler(Options{Store: store, Domain: "sonarr"})

	for _, path := range []string{"/api/sonarr/rule/port", "/api/sonarr/rule/synchronization", "/api/sonarr/rule/imported"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusNotFound {
			t.Fatalf("GET %s returned %d, want %d", path, response.Code, http.StatusNotFound)
		}
		if response.Header().Get("Allow") != "" {
			t.Fatalf("GET %s returned Allow %q, want empty", path, response.Header().Get("Allow"))
		}
	}
}

func TestRuleRoutes_whenMethodDoesNotMatchKnownRoute(t *testing.T) {
	store, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "x.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	handler := NewHandler(Options{Store: store, Domain: "radarr"})

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/radarr/rule/save", nil))
	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET /save returned %d, want %d", response.Code, http.StatusMethodNotAllowed)
	}
	if response.Header().Get("Allow") != http.MethodPost {
		t.Fatalf("GET /save returned Allow %q, want %q", response.Header().Get("Allow"), http.MethodPost)
	}
}
