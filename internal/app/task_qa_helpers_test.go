package app

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func postRoot(t *testing.T, base, path, body string, want int) string {
	t.Helper()
	request, err := http.NewRequest(http.MethodPost, base+path, bytes.NewBufferString(body))
	if err != nil {
		t.Fatal(err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if err != nil || response.StatusCode != want {
		t.Fatalf("POST %s status=%d error=%v", path, response.StatusCode, err)
	}
	return string(data)
}

func getRoot(t *testing.T, base, path string, want int) string {
	t.Helper()
	response, err := http.Get(base + path)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if err != nil || response.StatusCode != want {
		t.Fatalf("GET %s status=%d error=%v", path, response.StatusCode, err)
	}
	return string(data)
}

func taskCanaryLeaks(observed []string) int {
	canaries := taskCanaries()
	leaks := 0
	for _, canary := range canaries {
		for _, text := range observed {
			if bytes.Contains([]byte(text), []byte(canary)) {
				leaks++
				break
			}
		}
	}
	return leaks
}

func taskCanaries() []string {
	return []string{"task-canary-db-path", "task-canary-dsn", "task-canary-api-key", "task-canary-password", "task-canary-token", "task-canary-url", "task-canary-regex", "task-canary-xml", "task-canary-upload-name"}
}

func directoryDigest(t *testing.T, root string) string {
	t.Helper()
	var entries []string
	err := filepath.WalkDir(root, func(path string, _ os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path != root {
			entries = append(entries, filepath.ToSlash(path[len(root):]))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(entries)
	sum := sha256.Sum256([]byte(strings.Join(entries, "\n")))
	return hex.EncodeToString(sum[:])
}

func measuredManagementRoutes(t *testing.T, base string, include func(managementRoute) bool) int {
	t.Helper()
	count := 0
	for _, route := range managementRouteContracts() {
		if !include(route) {
			continue
		}
		request, err := http.NewRequest(route.method, base+route.path, bytes.NewReader(route.body))
		if err != nil {
			t.Fatal(err)
		}
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		_, readErr := io.ReadAll(response.Body)
		closeErr := response.Body.Close()
		if readErr != nil || closeErr != nil || response.StatusCode != route.status || response.Header.Get("Content-Type") != route.contentType {
			t.Fatalf("route %s %s status=%d content_type=%q read=%v close=%v", route.method, route.path, response.StatusCode, response.Header.Get("Content-Type"), readErr, closeErr)
		}
		count++
	}
	return count
}

func isTodo7Route(route managementRoute) bool {
	return strings.Contains(route.path, "/rule/") || route.path == "/api/rule/test?regex=.&replacement=x&example=x" || strings.Contains(route.path, "/example/")
}

func isTodo8Route(route managementRoute) bool { return strings.Contains(route.path, "/title/") }
