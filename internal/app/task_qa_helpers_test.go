package app

import (
	"bytes"
	"io"
	"net/http"
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
	canaries := []string{"task-canary-db", "task-canary-key", "task-canary-password"}
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
