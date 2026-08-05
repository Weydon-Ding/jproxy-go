package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"jproxy-go/internal/cache"
	"jproxy-go/internal/runtime"
	"jproxy-go/internal/store/sqlite"
)

func TestTask7_rootSurfaceMeasurements(t *testing.T) {
	// Given
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "task7.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	seedRootConfigs(t, store)
	snapshot, err := store.FormatterSnapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	provider := &controllableProvider{Provider: runtime.NewProvider(snapshot, store)}
	results := cache.NewTTLCache[string](time.Minute, 4)
	offsets := cache.NewTTLCache[[]int](time.Minute, 4)
	markers := cache.NewTTLCache[struct{}](time.Minute, 3)
	registry := runtime.NewRegistry(provider, results, offsets, markers)
	multipartAccess := newTask7MultipartAccess(t)
	handler := managementRoutesWithMultipartAccess(store, provider, registry, multipartAccess, titleSyncDependencies{})
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	routeCount := measuredManagementRoutes(t, server.URL, isTodo7Route)
	if routeCount != 25 {
		t.Fatalf("route_count=%d", routeCount)
	}

	// When
	postRoot(t, server.URL, "/api/sonarr/rule/save", `{"id":"rule-a","token":"title","regex":".*","replacement":"A","example":"Movie"}`, http.StatusOK)
	postRoot(t, server.URL, "/api/radarr/rule/save", `{"id":"rule-b","token":"title","regex":".*","replacement":"B","example":"Movie"}`, http.StatusOK)
	postRoot(t, server.URL, "/api/sonarr/example/save", `{"originalText":"example-a\nexample-b"}`, http.StatusOK)
	postRoot(t, server.URL, "/api/radarr/example/save", `{"originalText":"example-c"}`, http.StatusOK)
	beforeProjection := task7DatabaseDigest(t, store)
	projection := getRoot(t, server.URL, "/api/sonarr/example/query", http.StatusOK)
	afterProjection := task7DatabaseDigest(t, store)
	primaryRejections := 0
	primaryBodies := make([]string, 0, 4)
	for _, route := range []struct{ path, body string }{
		{"/api/sonarr/rule/save", `{"id":"00000000000000000000000000000000","token":"title","regex":".*","replacement":"x","example":"x"}`},
		{"/api/sonarr/rule/remove", `["normal","00000000000000000000000000000000"]`},
		{"/api/sonarr/rule/disable", `["normal","00000000000000000000000000000000"]`},
	} {
		primaryBodies = append(primaryBodies, postRoot(t, server.URL, route.path, route.body, http.StatusBadRequest))
		primaryRejections++
	}
	primaryBodies = append(primaryBodies, task7Import(t, server.URL, `[{"id":"normal","token":"title","regex":".*","replacement":"x","example":"x"},{"id":"00000000000000000000000000000000","token":"title","regex":".*","replacement":"x","example":"x"}]`))
	primaryRejections++
	invalidImport := task7Import(t, server.URL, `[{"id":"rule-import","token":"title","regex":".*","replacement":"I","example":"Movie"},{"id":"rule-bad","token":"title","regex":"[","replacement":"I","example":"Movie"}]`)
	afterImport := task7DatabaseDigest(t, store)
	unavailableSyncs := 0
	syncBodies := make([]string, 0, 2)
	for _, path := range []string{"/api/sonarr/rule/sync", "/api/radarr/rule/sync"} {
		syncBodies = append(syncBodies, postRoot(t, server.URL, path, "", http.StatusServiceUnavailable))
		unavailableSyncs++
	}
	canaryBodies := make([]string, 0, len(taskCanaries()))
	for _, canary := range taskCanaries() {
		canaryBodies = append(canaryBodies, postRoot(t, server.URL, "/api/sonarr/rule/save", `{"id":"00000000000000000000000000000000","token":"title","regex":".*","replacement":"`+canary+`","example":"x"}`, http.StatusBadRequest))
	}
	multipartAccess.assertAbsent(t)
	filesystemBodies := make([]string, 0, len(multipartAccess.filenames))
	for _, filename := range multipartAccess.filenames {
		filesystemBodies = append(filesystemBodies, task7ImportNamed(t, server.URL, `[{"id":"fs","token":"title","regex":".*","replacement":"x","example":"x"}]`, filename))
	}
	multipartCases := 0
	missingRequest, err := http.NewRequest(http.MethodPost, server.URL+"/api/sonarr/rule/import", nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, request := range []*http.Request{
		missingRequest,
		task7MultipartRequest(t, server.URL, `[{"id":"duplicate","token":"title","regex":".*","replacement":"x","example":"x"}]`, "rules.json", "file", "file"),
		task7MultipartRequest(t, server.URL, `[{"id":"extra","token":"title","regex":".*","replacement":"x","example":"x"}]`, "rules.json", "file", "extra"),
		task7MultipartRequest(t, server.URL, `[{"id":"bad","token":"title","regex":"[","replacement":"x","example":"x"}]`, "rules.json", "file"),
		task7MultipartRequest(t, server.URL, `[{}]`, "rules.json", "file"),
		task7MultipartRequest(t, server.URL, `[{"id":"first","token":"title","regex":".*","replacement":"x","example":"x"},{"id":"nth","token":"title","regex":"[","replacement":"x","example":"x"}]`, "rules.json", "file"),
		task7MultipartRequest(t, server.URL, "["+strings.Repeat(`{"id":"x","token":"title","regex":".*","replacement":"x","example":"x"},`, 200)+`{"id":"last","token":"title","regex":".*","replacement":"x","example":"x"}]`, "rules.json", "file"),
	} {
		if request == nil {
			t.Fatal("missing multipart request")
		}
		if request.Header.Get("Content-Type") == "" {
			request.Header.Set("Content-Type", "multipart/form-data; boundary=missing")
		}
		response, err := http.DefaultClient.Do(request)
		if err != nil || response.StatusCode != http.StatusBadRequest {
			t.Fatalf("multipart status=%v error=%v", response, err)
		}
		_ = response.Body.Close()
		multipartCases++
	}
	oversized := task7MultipartRequest(t, server.URL, `[]`, "rules.json", "file")
	oversized.Body = io.NopCloser(strings.NewReader(strings.Repeat("x", 256*1024+1)))
	oversized.ContentLength = 256*1024 + 1
	oversizedResponse, err := http.DefaultClient.Do(oversized)
	if err != nil || oversizedResponse.StatusCode != http.StatusBadRequest {
		t.Fatalf("oversized status=%v error=%v", oversizedResponse, err)
	}
	_ = oversizedResponse.Body.Close()
	multipartCases++
	multipartCases += len(filesystemBodies)
	filesystemTargetsChecked, filesystemOperations := multipartAccess.assertUnchanged(t)
	beforeCancel := task7DatabaseDigest(t, store)
	beforeCancelSnapshot := provider.Snapshot()
	results.Set("result", "value")
	offsets.Set("offset", []int{1})
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	cancelResponse := httptest.NewRecorder()
	handler.ServeHTTP(cancelResponse, httptest.NewRequest(http.MethodPost, "/api/sonarr/rule/save", strings.NewReader(`{"id":"cancelled","token":"title","regex":".*","replacement":"x","example":"x"}`)).WithContext(cancelled))
	cancelRollback := cancelResponse.Code == http.StatusInternalServerError && beforeCancel == task7DatabaseDigest(t, store) && reflect.DeepEqual(provider.Snapshot(), beforeCancelSnapshot) && cacheState(results, offsets, markers) == 2
	if !cancelRollback {
		t.Fatalf("cancel status=%d db_same=%t snapshot_same=%t cache_count=%d", cancelResponse.Code, beforeCancel == task7DatabaseDigest(t, store), reflect.DeepEqual(provider.Snapshot(), beforeCancelSnapshot), cacheState(results, offsets, markers))
	}
	provider.fail.Store(true)
	beforeRefreshFailure := provider.Snapshot()
	beforeFailureCache := cacheState(results, offsets, markers)
	refreshFailureResponse := httptest.NewRecorder()
	handler.ServeHTTP(refreshFailureResponse, httptest.NewRequest(http.MethodPost, "/api/sonarr/rule/save", strings.NewReader(`{"id":"committed","token":"title","regex":".*","replacement":"x","example":"x"}`)))
	committedAfterRefreshFailure := task7DatabaseDigest(t, store) != beforeCancel
	refreshFailureRetained := refreshFailureResponse.Code == http.StatusInternalServerError && committedAfterRefreshFailure && reflect.DeepEqual(provider.Snapshot(), beforeRefreshFailure) && cacheState(results, offsets, markers) == beforeFailureCache
	if !refreshFailureRetained {
		t.Fatal("refresh failure did not retain runtime state")
	}
	provider.fail.Store(false)
	retryResponse := httptest.NewRecorder()
	handler.ServeHTTP(retryResponse, httptest.NewRequest(http.MethodPost, "/api/sonarr/rule/save", strings.NewReader(`{"id":"retry","token":"title","regex":".*","replacement":"x","example":"x"}`)))
	cancelRetry := retryResponse.Code == http.StatusOK && provider.refreshes.Load() >= 2
	if !cancelRetry {
		t.Fatal("retry did not publish runtime state")
	}
	var logs strings.Builder
	logger := ConfiguredLogger(&logs)
	logger.Error("task7.request.rejected", "error", errors.New(strings.Join(taskCanaries(), ",")))
	errorGraph := string(FailureKindOf(errors.Join(errors.New(strings.Join(taskCanaries(), ",")), runtime.ErrSnapshotRefresh)))

	// Then
	importAtomic := beforeProjection == afterImport
	projectionUnchanged := beforeProjection == afterProjection
	if !importAtomic || !projectionUnchanged || filesystemTargetsChecked != len(multipartAccess.filenames) || filesystemOperations != 0 || multipartCases != 13 || !strings.Contains(projection, "example-a") || primaryRejections != len(primaryBodies) || unavailableSyncs != len(syncBodies) || invalidImport == "" {
		t.Fatalf("projection/import/sync contract failed")
	}
	httpLeaks := taskCanaryLeaks(append(append(append(primaryBodies, syncBodies...), append(canaryBodies, projection, invalidImport)...), filesystemBodies...))
	errorLeaks := taskCanaryLeaks([]string{errorGraph})
	logLeaks := taskCanaryLeaks([]string{logs.String()})
	canaryLeaks := httpLeaks + errorLeaks + logLeaks
	if canaryLeaks != 0 {
		t.Fatalf("canary_leaks=%d", canaryLeaks)
	}
	t.Logf("task7_qa route_count=%d db_rows_hash=%x example_projection_hash=%x primary_rejections=%d import_atomic=%t projection_db_hash_unchanged=%t unavailable_syncs=%d multipart_cases=%d part_reads=%d filesystem_targets_checked=%d filesystem_operations=%d cancel_rollback=%t cancel_retry=%t refresh_failure_retained=%t refresh_count=%d http_leaks=%d error_leaks=%d log_leaks=%d canary_leaks=%d", routeCount, beforeProjection, sha256.Sum256([]byte(projection)), primaryRejections, importAtomic, projectionUnchanged, unavailableSyncs, multipartCases, multipartAccess.partReads, filesystemTargetsChecked, filesystemOperations, cancelRollback, cancelRetry, refreshFailureRetained, provider.refreshes.Load(), httpLeaks, errorLeaks, logLeaks, canaryLeaks)
}

func task7MultipartRequest(t *testing.T, base, payload, filename string, names ...string) *http.Request {
	t.Helper()
	boundary := "task7cases"
	var body strings.Builder
	for _, name := range names {
		body.WriteString("--" + boundary + "\r\nContent-Disposition: form-data; name=\"" + name + "\"; filename=\"" + filename + "\"\r\nContent-Type: application/json\r\n\r\n" + payload + "\r\n")
	}
	body.WriteString("--" + boundary + "--\r\n")
	request, err := http.NewRequest(http.MethodPost, base+"/api/sonarr/rule/import", strings.NewReader(body.String()))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "multipart/form-data; boundary="+boundary)
	return request
}

func task7Import(t *testing.T, base, payload string) string {
	return task7ImportNamed(t, base, payload, "rules.json")
}

func task7ImportNamed(t *testing.T, base, payload, filename string) string {
	t.Helper()
	boundary := "task7"
	body := "--" + boundary + "\r\nContent-Disposition: form-data; name=\"file\"; filename=\"" + filename + "\"\r\nContent-Type: application/json\r\n\r\n" + payload + "\r\n--" + boundary + "--\r\n"
	request, err := http.NewRequest(http.MethodPost, base+"/api/sonarr/rule/import", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "multipart/form-data; boundary="+boundary)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	buffer := new(bytes.Buffer)
	if _, err := buffer.ReadFrom(response.Body); err != nil || response.StatusCode != http.StatusBadRequest {
		t.Fatalf("import status=%d error=%v", response.StatusCode, err)
	}
	return buffer.String()
}

func task7DatabaseDigest(t *testing.T, store *sqlite.Store) [32]byte {
	t.Helper()
	rules, err := store.Repositories().SonarrRules.Page(context.Background(), sqlite.RuleFilter{})
	if err != nil {
		t.Fatal(err)
	}
	examples, err := store.Repositories().SonarrExamples.Page(context.Background(), sqlite.ExampleFilter{})
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(struct {
		Rules    []sqlite.SonarrRule
		Examples []sqlite.SonarrExample
	}{rules.List, examples.List})
	if err != nil {
		t.Fatal(err)
	}
	return sha256.Sum256(data)
}
