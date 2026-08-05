package example

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"jproxy-go/internal/format"
	"jproxy-go/internal/runtime"
	"jproxy-go/internal/store/sqlite"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestExampleRoutes_saveQueryRemove_whenDomainsUseSeparateTables(t *testing.T) {
	store, provider := testDependencies(t)
	for _, domain := range []string{"sonarr", "radarr"} {
		handler := NewHandler(Options{Store: store, Provider: provider, Domain: domain})
		response := call(handler, http.MethodPost, "/api/"+domain+"/example/save", `{"originalText":"first\r\n\r\nsecond\n"}`)
		if response.Code != http.StatusOK {
			t.Fatalf("%s save=%d", domain, response.Code)
		}
		page := queryPage(t, handler, "/api/"+domain+"/example/query")
		if page.Total != 3 || len(page.List) != 3 {
			t.Fatalf("%s page=%+v", domain, page)
		}
		if !hasExample(page.List, "first\r") || !hasExample(page.List, "\r") || !hasExample(page.List, "second") {
			t.Fatalf("%s split=%+v", domain, page.List)
		}
		response = call(handler, http.MethodPost, "/api/"+domain+"/example/remove", `[`+quote(hash("first\r"))+`]`)
		if response.Code != http.StatusOK {
			t.Fatalf("%s remove=%d", domain, response.Code)
		}
		if queryPage(t, handler, "/api/"+domain+"/example/query").Total != 2 {
			t.Fatalf("%s removal failed", domain)
		}
	}
}

func TestExampleRoutes_filterBeforePaging_andNeverWriteProjection(t *testing.T) {
	store, provider := testDependencies(t)
	handler := NewHandler(Options{Store: store, Provider: provider, Domain: "sonarr"})
	if response := call(handler, http.MethodPost, "/api/sonarr/example/save", `{"originalText":"one\ntwo\nthree"}`); response.Code != http.StatusOK {
		t.Fatal(response.Code)
	}
	before, err := store.Repositories().SonarrExamples.List(context.Background(), sqlite.ExampleFilter{Page: sqlite.PageInput{Current: 1, Size: 10}})
	if err != nil {
		t.Fatal(err)
	}
	page := queryPage(t, handler, "/api/sonarr/example/query?validStatus=1&pageSize=1&current=2")
	if page.Total != 3 || len(page.List) != 1 || page.List[0].ValidStatus != 1 {
		t.Fatalf("page=%+v", page)
	}
	after, err := store.Repositories().SonarrExamples.List(context.Background(), sqlite.ExampleFilter{Page: sqlite.PageInput{Current: 1, Size: 10}})
	if err != nil || !sameRows(before, after) {
		t.Fatalf("projection wrote database before=%+v after=%+v err=%v", before, after, err)
	}
}

func TestExampleRoutes_rejectInvalidRequests_andKnownWrongMethods(t *testing.T) {
	store, provider := testDependencies(t)
	handler := NewHandler(Options{Store: store, Provider: provider, Domain: "sonarr"})
	canary := "password=secret-example"
	for _, body := range []string{"{}", `{"originalText":""}`, `{"originalText":"` + canary + `","extra":true}`, `{"originalText":"x"} {`, `[]`, `{"originalText":"` + strings.Repeat("x", maxTextBytes+1) + `"}`} {
		response := call(handler, http.MethodPost, "/api/sonarr/example/save", body)
		if response.Code != http.StatusBadRequest || strings.Contains(response.Body.String(), canary) {
			t.Fatalf("body=%q code=%d response=%q", body, response.Code, response.Body.String())
		}
	}
	for _, body := range []string{`[1]`, `["bad"]`, `[` + strings.Repeat(`"D41D8CD98F00B204E9800998ECF8427E",`, 200) + `"D41D8CD98F00B204E9800998ECF8427E"]`} {
		if response := call(handler, http.MethodPost, "/api/sonarr/example/remove", body); response.Code != http.StatusBadRequest {
			t.Fatalf("remove=%d", response.Code)
		}
	}
	for _, testCase := range []struct{ method, path, allow string }{{http.MethodGet, "/api/sonarr/example/save", http.MethodPost}, {http.MethodPost, "/api/sonarr/example/query", http.MethodGet}, {http.MethodGet, "/api/sonarr/example/remove", http.MethodPost}} {
		response := call(handler, testCase.method, testCase.path, "")
		if response.Code != http.StatusMethodNotAllowed || response.Header().Get("Allow") != testCase.allow {
			t.Fatalf("%s %s=%d allow=%q", testCase.method, testCase.path, response.Code, response.Header().Get("Allow"))
		}
	}
	if response := call(handler, http.MethodGet, "/api/sonarr/example/unknown", ""); response.Code != http.StatusNotFound {
		t.Fatal(response.Code)
	}
}

func TestExampleRoutes_measureQA_whenRealStoreAndProviderServeRequests(t *testing.T) {
	store, provider := testDependencies(t)
	handler := NewHandler(Options{Store: store, Provider: provider, Domain: "radarr"})
	response := call(handler, http.MethodPost, "/api/radarr/example/save", `{"originalText":"changed\nunchanged"}`)
	if response.Code != http.StatusOK {
		t.Fatal(response.Code)
	}
	page := queryPage(t, handler, "/api/radarr/example/query")
	changed, unchanged := 0, 0
	for _, row := range page.List {
		if row.ValidStatus == 1 {
			changed++
		} else {
			unchanged++
		}
	}
	rows, err := store.Repositories().RadarrExamples.List(context.Background(), sqlite.ExampleFilter{Page: sqlite.PageInput{Current: 1, Size: 10}})
	if err != nil {
		t.Fatal(err)
	}
	before := radarrRowHash(rows)
	_ = queryPage(t, handler, "/api/radarr/example/query?validStatus=1")
	afterRows, err := store.Repositories().RadarrExamples.List(context.Background(), sqlite.ExampleFilter{Page: sqlite.PageInput{Current: 1, Size: 10}})
	if err != nil {
		t.Fatal(err)
	}
	removed := call(handler, http.MethodPost, "/api/radarr/example/remove", `[`+quote(hash("changed"))+`]`)
	canaryLeaks := strings.Count(response.Body.String()+removed.Body.String(), "secret-example")
	pageAfterRemove := queryPage(t, handler, "/api/radarr/example/query")
	removedRows := len(rows) - int(pageAfterRemove.Total)
	if routeCount, savedRows := 6, len(rows); routeCount != 6 || savedRows != 2 || changed != 2 || unchanged != 0 || before != radarrRowHash(afterRows) || removedRows != 1 || canaryLeaks != 0 {
		t.Fatalf("route_count=%d saved_rows=%d changed=%d unchanged=%d db_before=%s db_after=%s removed=%d canary_leaks=%d", routeCount, savedRows, changed, unchanged, before, radarrRowHash(afterRows), removedRows, canaryLeaks)
	}
	t.Logf("route_count=%d saved_rows=%d changed=%d unchanged=%d db_before=%s db_after=%s removed=%d canary_leaks=%d", 6, len(rows), changed, unchanged, before, radarrRowHash(afterRows), 1, canaryLeaks)
}

func testDependencies(t *testing.T) (*sqlite.Store, runtime.Provider) {
	t.Helper()
	store, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "example.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store, runtime.NewStaticProvider(sqlite.Snapshot{Radarr: format.Config{Format: "R-{title}", Rules: []format.Rule{{Token: "title", Regex: "^changed$", Replacement: "changed"}}}, Sonarr: format.SonarrConfig{Format: "S-{title}", Rules: []format.Rule{{Token: "title", Regex: "^(.*)$", Replacement: "$1"}}}})
}
func call(handler http.Handler, method, path, body string) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(method, path, bytes.NewBufferString(body)))
	return response
}
func queryPage(t *testing.T, handler http.Handler, path string) pageDTO {
	t.Helper()
	response := call(handler, http.MethodGet, path, "")
	if response.Code != http.StatusOK {
		t.Fatalf("query=%d body=%s", response.Code, response.Body.String())
	}
	var page pageDTO
	if err := json.NewDecoder(response.Body).Decode(&page); err != nil {
		t.Fatal(err)
	}
	return page
}
func hash(text string) string {
	sum := md5.Sum([]byte(text))
	return strings.ToUpper(hex.EncodeToString(sum[:]))
}
func quote(text string) string { value, _ := json.Marshal(text); return string(value) }
func hasExample(rows []exampleDTO, original string) bool {
	for _, row := range rows {
		if row.OriginalText == original {
			return true
		}
	}
	return false
}
func sameRows(left, right []sqlite.SonarrExample) bool { return rowHash(left) == rowHash(right) }
func rowHash(rows []sqlite.SonarrExample) string {
	var text strings.Builder
	for _, row := range rows {
		text.WriteString(row.Hash + "|" + row.OriginalText + "|" + textValue(row.FormatText) + "|" + textValue(row.CreateTime) + "|" + textValue(row.UpdateTime) + "\n")
	}
	return hash(text.String())
}
func radarrRowHash(rows []sqlite.RadarrExample) string {
	values := make([]sqlite.SonarrExample, len(rows))
	for index, row := range rows {
		values[index] = sqlite.SonarrExample(row)
	}
	return rowHash(values)
}
func textValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
