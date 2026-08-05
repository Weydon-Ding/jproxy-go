package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

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
	provider := runtime.NewProvider(snapshot, store)
	server := httptest.NewServer(rootHandler(rootRouteConfig(true), provider, store))
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
	canaryBodies := []string{
		postRoot(t, server.URL, "/api/sonarr/rule/save", `{"id":"00000000000000000000000000000000","token":"title","regex":"task-canary-regex","replacement":"task-canary-api-key","example":"task-canary-xml"}`, http.StatusBadRequest),
		task7ImportNamed(t, server.URL, `[{"id":"task-canary-db-path","token":"title","regex":"[","replacement":"task-canary-password","example":"task-canary-dsn task-canary-token task-canary-url"}]`, "../task-canary-upload-name"),
	}
	filesystemBoundary := t.TempDir()
	beforeFilesystem := directoryDigest(t, filesystemBoundary)
	filesystemBodies := make([]string, 0, 5)
	for _, filename := range []string{"../traverse.json", `C:\absolute.json`, `\\host\share.json`, "bad\x00.json", strings.Repeat("x", 256)} {
		filesystemBodies = append(filesystemBodies, task7ImportNamed(t, server.URL, `[{"id":"fs","token":"title","regex":".*","replacement":"x","example":"x"}]`, filename))
	}
	filesystemUnchanged := beforeFilesystem == directoryDigest(t, filesystemBoundary)

	// Then
	importAtomic := beforeProjection == afterImport
	projectionUnchanged := beforeProjection == afterProjection
	if !importAtomic || !projectionUnchanged || !filesystemUnchanged || !strings.Contains(projection, "example-a") || primaryRejections != len(primaryBodies) || unavailableSyncs != len(syncBodies) || invalidImport == "" {
		t.Fatalf("projection/import/sync contract failed")
	}
	canaryLeaks := taskCanaryLeaks(append(append(append(primaryBodies, syncBodies...), append(canaryBodies, projection, invalidImport)...), filesystemBodies...))
	if canaryLeaks != 0 {
		t.Fatalf("canary_leaks=%d", canaryLeaks)
	}
	t.Logf("task7_qa route_count=%d db_rows_hash=%x example_projection_hash=%x primary_rejections=%d import_atomic=%t projection_db_hash_unchanged=%t unavailable_syncs=%d filesystem_unchanged=%t canary_leaks=%d", routeCount, beforeProjection, sha256.Sum256([]byte(projection)), primaryRejections, importAtomic, projectionUnchanged, unavailableSyncs, filesystemUnchanged, canaryLeaks)
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
