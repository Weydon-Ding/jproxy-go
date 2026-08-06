package rulesync

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestClient_authorsAndRules_usePrimaryBeforeBackupAndTrimStableOrder(t *testing.T) {
	// Given
	var primaryCalls, backupCalls atomic.Int64
	primary := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		primaryCalls.Add(1)
		switch request.URL.Path {
		case "/author.json":
			_, _ = writer.Write([]byte(`[" first ","second"]`))
		case "/sonarr@first.json":
			_, _ = writer.Write([]byte(`[{"id":"first-rule","token":"title","priority":1,"regex":"^x$","replacement":"y","offset":0,"example":"x","remark":"remark","author":"first","validStatus":1}]`))
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer primary.Close()
	backup := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		backupCalls.Add(1)
		writer.WriteHeader(http.StatusInternalServerError)
	}))
	defer backup.Close()
	client, err := NewClient(ClientOptions{PrimaryURL: primary.URL, BackupURL: backup.URL, Timeout: time.Second})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	// When
	authors, authorErr := client.Authors(context.Background())
	rules, ruleErr := client.rules(context.Background(), Sonarr, authors[0])

	// Then
	if authorErr != nil || ruleErr != nil || strings.Join(authors, ",") != "first,second" || len(rules) != 1 || primaryCalls.Load() != 2 || backupCalls.Load() != 0 {
		t.Fatalf("authors=%v authorErr=%v rules=%v ruleErr=%v primary=%d backup=%d", authors, authorErr, rules, ruleErr, primaryCalls.Load(), backupCalls.Load())
	}
}

func TestClient_usesBackupWhenPrimaryOperationFails(t *testing.T) {
	// Given
	var primaryCalls, backupCalls atomic.Int64
	primary := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		primaryCalls.Add(1)
		writer.WriteHeader(http.StatusBadGateway)
	}))
	defer primary.Close()
	backup := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		backupCalls.Add(1)
		if request.URL.Path == "/author.json" {
			_, _ = writer.Write([]byte(`["backup"]`))
			return
		}
		_, _ = writer.Write([]byte(validRule("backup", "backup-rule")))
	}))
	defer backup.Close()
	client, err := NewClient(ClientOptions{PrimaryURL: primary.URL, BackupURL: backup.URL, Timeout: time.Second})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	// When
	authors, fetchErr := client.Authors(context.Background())
	rules, rulesErr := client.rules(context.Background(), Sonarr, "backup")

	// Then
	if fetchErr != nil || rulesErr != nil || strings.Join(authors, ",") != "backup" || len(rules) != 1 || primaryCalls.Load() != 2 || backupCalls.Load() != 2 {
		t.Fatalf("authors=%v fetchErr=%v rules=%v rulesErr=%v primary=%d backup=%d", authors, fetchErr, rules, rulesErr, primaryCalls.Load(), backupCalls.Load())
	}
}

func TestClient_returnsRedactedErrorWhenBothOperationsFail(t *testing.T) {
	// Given
	secret := "secret-canary"
	primary := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) { writer.WriteHeader(http.StatusBadGateway) }))
	defer primary.Close()
	backup := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) { _, _ = writer.Write([]byte(secret)) }))
	defer backup.Close()
	client, err := NewClient(ClientOptions{PrimaryURL: primary.URL, BackupURL: backup.URL, Timeout: time.Second})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	// When
	_, fetchErr := client.Authors(context.Background())

	// Then
	if fetchErr == nil || strings.Contains(fetchErr.Error(), secret) || strings.Contains(fetchErr.Error(), primary.URL) || strings.Contains(fetchErr.Error(), backup.URL) {
		t.Fatalf("Authors() error = %v", fetchErr)
	}
}

func TestClient_rejectsMalformedAndOversizedResponses(t *testing.T) {
	for name, body := range map[string]string{"malformed": `{`, "oversized": "[\"" + strings.Repeat("a", maxResponseBytes) + "\"]"} {
		t.Run(name, func(t *testing.T) {
			// Given
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) { _, _ = writer.Write([]byte(body)) }))
			defer server.Close()
			client, err := NewClient(ClientOptions{PrimaryURL: server.URL, Timeout: time.Second})
			if err != nil {
				t.Fatalf("NewClient() error = %v", err)
			}

			// When
			_, fetchErr := client.Authors(context.Background())

			// Then
			if fetchErr == nil || strings.Contains(fetchErr.Error(), body) {
				t.Fatalf("Authors() error = %v", fetchErr)
			}
		})
	}
}

func TestClient_rejectsRedirectAndUsesBackupForThatOperation(t *testing.T) {
	// Given
	backup := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) { _, _ = writer.Write([]byte(`["backup"]`)) }))
	defer backup.Close()
	primary := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, backup.URL+"/author.json", http.StatusFound)
	}))
	defer primary.Close()
	client, err := NewClient(ClientOptions{PrimaryURL: primary.URL, BackupURL: backup.URL, Timeout: time.Second})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	// When
	authors, fetchErr := client.Authors(context.Background())

	// Then
	if fetchErr != nil || len(authors) != 1 || authors[0] != "backup" {
		t.Fatalf("authors=%v err=%v", authors, fetchErr)
	}
}
