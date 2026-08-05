package rule

import (
	"context"
	"jproxy-go/internal/store/sqlite"
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
