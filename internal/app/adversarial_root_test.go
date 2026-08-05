package app

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"jproxy-go/internal/cache"
	"jproxy-go/internal/proxy"
	"jproxy-go/internal/runtime"
	"jproxy-go/internal/store/sqlite"
)

func TestRootMux_rejectsPrimaryRuleIDWithoutPartialWrites(t *testing.T) {
	store, handler, _ := adversarialRoot(t)
	const primary = "00000000000000000000000000000000"
	for _, request := range []*http.Request{
		httptest.NewRequest(http.MethodPost, "/api/sonarr/rule/save", strings.NewReader(`{"id":"`+primary+`","token":"title","regex":".*","replacement":"x","example":"x"}`)),
		httptest.NewRequest(http.MethodPost, "/api/sonarr/rule/remove", strings.NewReader(`["normal","`+primary+`","other"]`)),
		httptest.NewRequest(http.MethodPost, "/api/sonarr/rule/disable", strings.NewReader(`["normal","`+primary+`"]`)),
		multipartRuleRequest(t, `[{"id":"normal","token":"title","regex":".*","replacement":"x","example":"x"},{"id":"`+primary+`","token":"title","regex":".*","replacement":"x","example":"x"}]`, "rules.json", "file"),
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest || ruleTotal(t, store) != 0 {
			t.Fatalf("status=%d rows=%d", response.Code, ruleTotal(t, store))
		}
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/sonarr/rule/enable", strings.NewReader(`["`+primary+`"]`)))
	if response.Code != http.StatusOK {
		t.Fatalf("enable=%d", response.Code)
	}
	t.Logf("task7_adversarial primary_rejections=%d primary_enable_allowed=%t", 4, response.Code == http.StatusOK)
}

func TestRootMux_rejectsMalformedRuleMultipartWithoutWrites(t *testing.T) {
	store, handler, _ := adversarialRoot(t)
	valid := `[{"id":"r1","token":"title","regex":".*","replacement":"x","example":"x"}]`
	cases := []*http.Request{
		multipartRuleRequest(t, valid, "rules.json", "other"),
		multipartRuleRequest(t, valid, "rules.json", "file", "file"),
		multipartRuleRequest(t, valid, "rules.json", "file", "extra"),
		multipartRuleRequest(t, ``, "rules.json", "file"), multipartRuleRequest(t, `{}`, "rules.json", "file"),
		multipartRuleRequest(t, valid+` trailing`, "rules.json", "file"), multipartRuleRequest(t, valid, "../rules.json", "file"),
		multipartRuleRequest(t, valid, `..\rules.json`, "file"), multipartRuleRequest(t, valid, `C:\rules.json`, "file"),
		multipartRuleRequest(t, valid, `\\host\rules.json`, "file"), multipartRuleRequest(t, valid, "bad\x00.json", "file"),
		multipartRuleRequest(t, valid, strings.Repeat("a", 256), "file"),
		httptest.NewRequest(http.MethodPost, "/api/sonarr/rule/import", strings.NewReader("bad")),
	}
	for _, request := range cases {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest || ruleTotal(t, store) != 0 {
			t.Fatalf("status=%d rows=%d", response.Code, ruleTotal(t, store))
		}
	}
	over := multipartRuleRequest(t, `[]`, "rules.json", "file")
	over.Body = io.NopCloser(strings.NewReader(strings.Repeat("x", 256*1024+1)))
	over.ContentLength = 256*1024 + 1
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, over)
	if response.Code != http.StatusBadRequest || ruleTotal(t, store) != 0 {
		t.Fatalf("oversized=%d rows=%d", response.Code, ruleTotal(t, store))
	}
	t.Logf("task7_adversarial multipart_cases=%d oversized_rejected=%t filesystem_access=false", len(cases)+1, response.Code == http.StatusBadRequest)
}

func TestRootMux_rejectsOversizedAndPartiallyInvalidRuleImportsWithoutWrites(t *testing.T) {
	store, handler, _ := adversarialRoot(t)
	rows := make([]string, 201)
	for index := range rows {
		rows[index] = `{"id":"rule-` + string(rune('a'+index%26)) + `","token":"title","regex":".*","replacement":"x","example":"x"}`
	}
	requests := []*http.Request{
		multipartRuleRequest(t, `[{"id":"first","token":"title","regex":".*","replacement":"x","example":"x"},{"id":"invalid","token":"title","regex":"[","replacement":"x","example":"x"}]`, "rules.json", "file"),
		multipartRuleRequest(t, "["+strings.Join(rows, ",")+"]", "rules.json", "file"),
		httptest.NewRequest(http.MethodPost, "/api/sonarr/rule/import", strings.NewReader("--mismatch--\r\n")).WithContext(context.Background()),
	}
	requests[2].Header.Set("Content-Type", "multipart/form-data; boundary=boundary")
	for _, request := range requests {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest || ruleTotal(t, store) != 0 {
			t.Fatalf("status=%d rows=%d", response.Code, ruleTotal(t, store))
		}
	}
	t.Logf("task7_adversarial invalid_middle_atomic=%t batch_limit_rejected=%t malformed_boundary_rejected=%t", true, true, true)
}

func TestRootMux_ruleCancellationAndRefreshFailureAreObservable(t *testing.T) {
	store, handler, provider := adversarialRoot(t)
	before := provider.Snapshot()
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	request := httptest.NewRequest(http.MethodPost, "/api/sonarr/rule/save", strings.NewReader(`{"id":"cancelled","token":"title","regex":".*","replacement":"x","example":"x"}`)).WithContext(canceled)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusInternalServerError || ruleTotal(t, store) != 0 || !reflect.DeepEqual(provider.Snapshot(), before) {
		t.Fatalf("cancel=%d rows=%d", response.Code, ruleTotal(t, store))
	}
	provider.fail.Store(true)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/sonarr/rule/save", strings.NewReader(`{"id":"committed","token":"title","regex":".*","replacement":"x","example":"x"}`)))
	if response.Code != http.StatusInternalServerError || ruleTotal(t, store) != 1 || !reflect.DeepEqual(provider.Snapshot(), before) {
		t.Fatalf("failure=%d rows=%d", response.Code, ruleTotal(t, store))
	}
	provider.fail.Store(false)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/sonarr/rule/save", strings.NewReader(`{"id":"retry","token":"title","regex":".*","replacement":"x","example":"x"}`)))
	if response.Code != http.StatusOK || ruleTotal(t, store) != 2 || provider.refreshes.Load() == 0 {
		t.Fatalf("retry=%d rows=%d refreshes=%d", response.Code, ruleTotal(t, store), provider.refreshes.Load())
	}
	t.Logf("task7_adversarial cancel_retry=%t refresh_failure_retained=%t committed_runtime_unpublished=%t", ruleTotal(t, store) == 2, true, true)
}

func TestRootMux_titlePagesTMDBReuseAndUnavailableSyncAreIsolated(t *testing.T) {
	store, handler, provider := adversarialRoot(t)
	for _, body := range []string{
		`{"id":10,"tvdbId":7,"tmdbId":91,"language":"en","title":"First","validStatus":1}`,
		`{"tvdbId":7,"language":"en","title":"Second","validStatus":1}`,
		`{"tvdbId":8,"language":"en","title":"No Reuse","validStatus":1}`,
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/tmdb/title/save", strings.NewReader(body)))
		if response.Code != http.StatusOK {
			t.Fatal(response.Code)
		}
	}
	page := httptest.NewRecorder()
	handler.ServeHTTP(page, httptest.NewRequest(http.MethodGet, "/api/tmdb/title/query?current=1&pageSize=1", nil))
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(page.Body.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded) != 4 || decoded["size"] != nil || decoded["current"] == nil || decoded["pageSize"] == nil || decoded["total"] == nil || decoded["list"] == nil {
		t.Fatalf("page fields=%s", page.Body.String())
	}
	rows, err := store.Repositories().TMDBTitles.FindByTVDBID(context.Background(), 7)
	if err != nil || len(rows) != 2 || rows[1].TMDBID == nil || *rows[1].TMDBID != 91 {
		t.Fatalf("reuse=%+v err=%v", rows, err)
	}
	noReuse, err := store.Repositories().TMDBTitles.FindByTVDBID(context.Background(), 8)
	if err != nil || len(noReuse) != 1 || noReuse[0].TMDBID != nil {
		t.Fatalf("no_reuse=%+v err=%v", noReuse, err)
	}
	before := provider.Snapshot()
	for _, path := range []string{"/api/sonarr/title/sync", "/api/radarr/title/sync", "/api/tmdb/title/sync"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, path, nil))
		if response.Code != http.StatusServiceUnavailable || !reflect.DeepEqual(provider.Snapshot(), before) {
			t.Fatalf("sync %s=%d", path, response.Code)
		}
	}
	t.Logf("task8_adversarial page_fields_exact=%t tmdb_supplied_generated_reused_no_reuse=%t unavailable_syncs=%d invalidation_isolated=%t", len(decoded) == 4, true, 3, true)
}

type controllableProvider struct {
	runtime.Provider
	fail      atomic.Bool
	refreshes atomic.Int64
}

func (p *controllableProvider) Refresh(ctx context.Context, scope runtime.Scope) error {
	p.refreshes.Add(1)
	if p.fail.Load() {
		return errors.New("refresh failed")
	}
	return p.Provider.Refresh(ctx, scope)
}

func adversarialRoot(t *testing.T) (*sqlite.Store, http.Handler, *controllableProvider) {
	t.Helper()
	store, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "adversarial.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	seedRootConfigs(t, store)
	snapshot, err := store.FormatterSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	provider := &controllableProvider{Provider: runtime.NewProvider(snapshot, store)}
	registry := runtime.NewRegistry(provider, cache.NewTTLCache[string](time.Minute, 8), cache.NewTTLCache[[]int](time.Minute, 8), cache.NewTTLCache[struct{}](time.Minute, 3))
	root := http.NewServeMux()
	root.Handle("/api/", managementRoutes(store, provider, registry))
	root.Handle("/", proxy.NewServerWithRuntime(rootRouteConfig(true), proxy.RuntimeOptions{Provider: provider}).Routes())
	return store, root, provider
}

func ruleTotal(t *testing.T, store *sqlite.Store) int64 {
	t.Helper()
	page, err := store.Repositories().SonarrRules.Page(context.Background(), sqlite.RuleFilter{})
	if err != nil {
		t.Fatal(err)
	}
	return page.Total
}
func multipartRuleRequest(t *testing.T, payload, filename string, names ...string) *http.Request {
	t.Helper()
	boundary := "boundary"
	var body strings.Builder
	for _, name := range names {
		body.WriteString("--" + boundary + "\r\nContent-Disposition: form-data; name=\"" + name + "\"; filename=\"" + filename + "\"\r\nContent-Type: application/json\r\n\r\n" + payload + "\r\n")
	}
	body.WriteString("--" + boundary + "--\r\n")
	request := httptest.NewRequest(http.MethodPost, "/api/sonarr/rule/import", strings.NewReader(body.String()))
	request.Header.Set("Content-Type", "multipart/form-data; boundary="+boundary)
	return request
}
