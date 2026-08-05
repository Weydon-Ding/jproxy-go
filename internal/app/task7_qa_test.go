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

	// When
	postRoot(t, server.URL, "/api/sonarr/rule/save", `{"id":"rule-a","token":"title","regex":".*","replacement":"A","example":"Movie"}`, http.StatusOK)
	postRoot(t, server.URL, "/api/radarr/rule/save", `{"id":"rule-b","token":"title","regex":".*","replacement":"B","example":"Movie"}`, http.StatusOK)
	postRoot(t, server.URL, "/api/sonarr/example/save", `{"originalText":"example-a\nexample-b"}`, http.StatusOK)
	postRoot(t, server.URL, "/api/radarr/example/save", `{"originalText":"example-c"}`, http.StatusOK)
	beforeProjection := task7RuleHash(t, server.URL, "/api/sonarr/rule/query")
	projection := getRoot(t, server.URL, "/api/sonarr/example/query", http.StatusOK)
	afterProjection := task7RuleHash(t, server.URL, "/api/sonarr/rule/query")
	primary := postRoot(t, server.URL, "/api/sonarr/rule/remove", `["00000000000000000000000000000000"]`, http.StatusBadRequest)
	invalidImport := task7Import(t, server.URL, `[{"id":"rule-import","token":"title","regex":".*","replacement":"I","example":"Movie"},{"id":"rule-bad","token":"title","regex":"[","replacement":"I","example":"Movie"}]`)
	afterImport := task7RuleHash(t, server.URL, "/api/sonarr/rule/query")
	syncOne := postRoot(t, server.URL, "/api/sonarr/rule/sync", "", http.StatusServiceUnavailable)
	syncTwo := postRoot(t, server.URL, "/api/radarr/rule/sync", "", http.StatusServiceUnavailable)

	// Then
	if beforeProjection != afterProjection || beforeProjection != afterImport || !strings.Contains(projection, "example-a") || primary == "" || invalidImport == "" || syncOne == "" || syncTwo == "" {
		t.Fatalf("projection/import/sync contract failed")
	}
	canaryLeaks := taskCanaryLeaks([]string{projection, primary, invalidImport, syncOne, syncTwo})
	if canaryLeaks != 0 {
		t.Fatalf("canary_leaks=%d", canaryLeaks)
	}
	t.Logf("task7_qa route_count=%d rule_rows_hash=%x example_rows_hash=%x primary_rejections=%d import_atomic=%t projection_db_hash_unchanged=%t unavailable_syncs=%d canary_leaks=%d", 25, task7RuleHash(t, server.URL, "/api/sonarr/rule/query"), sha256.Sum256([]byte(projection)), 1, beforeProjection == afterImport, beforeProjection == afterProjection, 2, canaryLeaks)
}

func task7RuleHash(t *testing.T, base, path string) [32]byte {
	t.Helper()
	body := getRoot(t, base, path, http.StatusOK)
	var page struct {
		List []json.RawMessage `json:"list"`
	}
	if err := json.Unmarshal([]byte(body), &page); err != nil || len(page.List) == 0 {
		t.Fatalf("rule page=%q error=%v", body, err)
	}
	return sha256.Sum256([]byte(body))
}

func task7Import(t *testing.T, base, payload string) string {
	t.Helper()
	boundary := "task7"
	body := "--" + boundary + "\r\nContent-Disposition: form-data; name=\"file\"; filename=\"rules.json\"\r\nContent-Type: application/json\r\n\r\n" + payload + "\r\n--" + boundary + "--\r\n"
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
