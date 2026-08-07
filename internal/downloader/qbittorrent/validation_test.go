package qbittorrent

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_acceptsJavaCompatibleRenameBoundaries(t *testing.T) {
	// Given
	requests := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/auth/login" {
			http.SetCookie(w, &http.Cookie{Name: "SID", Value: "session"})
			return
		}
		requests++
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()
	client := newClient(t, &admissionProvider{snapshot: qbSnapshot(upstream.URL, 1)}, nil)

	// When
	renameErr := client.Rename(context.Background(), "hash", "My Title (2026) - v2.0")
	pathErr := client.RenameFile(context.Background(), "hash", "Season 1/Episode 01.mkv", "Season 1/Episode 01 - Final.mkv")

	// Then
	if renameErr != nil || pathErr != nil || requests != 2 {
		t.Fatalf("rename=%v path=%v requests=%d", renameErr, pathErr, requests)
	}
}

func TestClient_rejectsUnsafeRenameBoundaries(t *testing.T) {
	for _, operation := range []struct {
		name string
		call func(*Client) error
	}{
		{name: "blank name", call: func(client *Client) error { return client.Rename(context.Background(), "hash", " ") }},
		{name: "name traversal", call: func(client *Client) error { return client.Rename(context.Background(), "hash", "..") }},
		{name: "name separator", call: func(client *Client) error { return client.Rename(context.Background(), "hash", "nested/name") }},
		{name: "old traversal", call: func(client *Client) error { return client.RenameFile(context.Background(), "hash", "../old", "new") }},
		{name: "new absolute", call: func(client *Client) error { return client.RenameFile(context.Background(), "hash", "old", "/new") }},
		{name: "new backslash", call: func(client *Client) error { return client.RenameFile(context.Background(), "hash", "old", `new\path`) }},
	} {
		t.Run(operation.name, func(t *testing.T) {
			// Given
			client := newClient(t, providerWithoutAdmission{snapshot: qbSnapshot("http://example.test", 1)}, nil)

			// When
			err := operation.call(client)

			// Then
			if !errors.Is(err, ErrInvalidConfig) {
				t.Fatalf("err=%v", err)
			}
		})
	}
}
