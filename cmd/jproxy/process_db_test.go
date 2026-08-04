package main

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"jproxy-go/internal/store/sqlite"
)

func TestProcess_writableDatabaseLocksThenRestarts(t *testing.T) {
	// Given
	path := filepath.Join(t.TempDir(), "runtime.db")
	seedFormatterStore(t, path)
	binary := buildBinary(t)
	environment := append(runtimeEnvironment(), "JPROXY_DB_ENABLED=true", "JPROXY_DB_PATH="+path, "JPROXY_RADARR_FORMAT_ENABLED=true", "JPROXY_SONARR_FORMAT_ENABLED=true")
	first := startProcess(t, binary, environment)
	address := awaitAddress(t, first.address)

	// When
	status, body := health(t, address)
	second := exec.Command(binary)
	second.Env = environment
	output, err := second.CombinedOutput()
	stopProcess(t, first)
	restarted := startProcess(t, binary, environment)
	restartedAddress := awaitAddress(t, restarted.address)
	restartedStatus, restartedBody := health(t, restartedAddress)
	stopProcess(t, restarted)

	// Then
	if status != 200 || body != "ok" {
		t.Fatalf("first health = %d %q", status, body)
	}
	if err == nil {
		t.Fatalf("second writer succeeded: %s", output)
	}
	if !strings.Contains(string(output), `"error_kind":"database_locked"`) {
		t.Fatalf("second writer error classification = %s", output)
	}
	if restartedStatus != 200 || restartedBody != "ok" {
		t.Fatalf("restarted health = %d %q", restartedStatus, restartedBody)
	}
}

func TestProcess_rejectsInvalidFormatterSnapshotWithoutReadiness(t *testing.T) {
	// Given
	path := filepath.Join(t.TempDir(), "invalid-snapshot.db")
	store, err := sqlite.Open(context.Background(), path)
	if err != nil {
		t.Fatalf("open empty store: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close empty store: %v", err)
	}
	binary := buildBinary(t)
	command := exec.Command(binary)
	command.Env = append(runtimeEnvironment(), "JPROXY_DB_ENABLED=true", "JPROXY_DB_PATH="+path, "JPROXY_CANARY_TOKEN=canary-secret")

	// When
	output, err := command.CombinedOutput()

	// Then
	if err == nil {
		t.Fatalf("invalid snapshot exited successfully: %s", output)
	}
	if strings.Contains(string(output), "application.started") || strings.Contains(string(output), "canary-secret") {
		t.Fatalf("invalid snapshot output = %s", output)
	}
	if !strings.Contains(string(output), `"error_kind":"invalid_formatter_snapshot"`) {
		t.Fatalf("invalid snapshot error classification = %s", output)
	}
}

func seedFormatterStore(t *testing.T, path string) {
	t.Helper()
	store, err := sqlite.Open(context.Background(), path)
	if err != nil {
		t.Fatalf("open seed store: %v", err)
	}
	repository := store.Repositories().SystemConfigs
	for _, value := range []sqlite.SystemConfig{
		{ID: 1, Key: "radarrIndexerFormat", Value: stringPointer("{title}"), ValidStatus: sqlite.Valid},
		{ID: 2, Key: "sonarrIndexerFormat", Value: stringPointer("{title}"), ValidStatus: sqlite.Valid},
		{ID: 3, Key: "cleanTitleRegex", Value: stringPointer(""), ValidStatus: sqlite.Valid},
	} {
		if err := repository.Upsert(context.Background(), value); err != nil {
			_ = store.Close()
			t.Fatalf("seed formatter config: %v", err)
		}
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close seed store: %v", err)
	}
}

func stringPointer(value string) *string { return &value }
