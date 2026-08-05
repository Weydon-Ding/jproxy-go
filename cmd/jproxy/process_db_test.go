package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProcess_migratesJavaFixtureBeforeReadiness(t *testing.T) {
	// Given
	fixture := newJavaFormatterFixture(t)
	assertMigrationLedgerAbsent(t, fixture.database)
	process := startRuntimeProcess(t, buildRuntimeBinary(t), fixture.runtimeEnvironment())

	// When
	address := requireReadiness(t, process)
	ledger := readMigrationLedger(t, fixture.database)
	status, body := getHealth(t, address)
	stopRuntimeProcess(t, process)

	// Then
	if len(ledger) != 2 {
		t.Fatalf("migration ledger = %#v, want two entries", ledger)
	}
	if status != 200 || body != "ok" {
		t.Fatalf("health = %d %q, want 200 ok", status, body)
	}
	t.Logf("process_pid=%d pre_migration_ledger_absent=true listen_addr=%s migration_ledger=%v health_status=%d health_body=%q", process.pid(), address, ledger, status, body)
}

func TestProcess_writerConflictReleasesLockAndRestarts(t *testing.T) {
	// Given
	fixture := newJavaFormatterFixture(t)
	first := startRuntimeProcess(t, buildRuntimeBinary(t), fixture.runtimeEnvironment())
	firstAddress := requireReadiness(t, first)
	secondAddress := reserveTestAddress(t)
	second := startRuntimeProcess(t, buildRuntimeBinary(t), fixture.runtimeEnvironment("ADDR="+secondAddress))

	// When
	firstStatus, firstBody := getHealth(t, firstAddress)
	writerExit := requireFailureWithoutListener(t, second, secondAddress)
	stopRuntimeProcess(t, first)
	restarted := startRuntimeProcess(t, buildRuntimeBinary(t), fixture.runtimeEnvironment())
	restartedAddress := requireReadiness(t, restarted)
	restartedStatus, restartedBody := getHealth(t, restartedAddress)
	stopRuntimeProcess(t, restarted)

	// Then
	if firstStatus != 200 || firstBody != "ok" || !writerExit.failedWith("database_locked") || restartedStatus != 200 || restartedBody != "ok" {
		t.Fatalf("first=%d %q writer=%s restart=%d %q", firstStatus, firstBody, writerExit.output, restartedStatus, restartedBody)
	}
	t.Logf("first_pid=%d writer_pid=%d writer_error=database_locked restart_pid=%d restart_addr=%s restart_health=%d %q", first.pid(), writerExit.pid, restarted.pid(), restartedAddress, restartedStatus, restartedBody)
}

func TestProcess_rejectsInvalidFormatterSnapshotWithoutListener(t *testing.T) {
	// Given
	fixture := newJavaEmptyFixture(t)
	address := reserveTestAddress(t)
	process := startRuntimeProcess(t, buildRuntimeBinary(t), fixture.runtimeEnvironment("ADDR="+address))

	// When
	exit := requireFailureWithoutListener(t, process, address)

	// Then
	if !exit.failedWith("invalid_formatter_snapshot") {
		t.Fatalf("startup output = %s", exit.output)
	}
	t.Logf("process_pid=%d startup_failure=invalid_formatter_snapshot listener_refused=true", exit.pid)
}

func TestProcess_rejectsIncompatibleDatabaseWithoutListener(t *testing.T) {
	// Given
	fixture := newIncompatibleFixture(t)
	address := reserveTestAddress(t)
	process := startRuntimeProcess(t, buildRuntimeBinary(t), fixture.runtimeEnvironment("ADDR="+address))

	// When
	exit := requireFailureWithoutListener(t, process, address)

	// Then
	if !exit.failedWith("incompatible_schema") {
		t.Fatalf("startup output = %s", exit.output)
	}
	t.Logf("process_pid=%d startup_failure=incompatible_schema listener_refused=true", exit.pid)
}

func TestProcess_rejectsCorruptDatabaseWithoutListener(t *testing.T) {
	// Given
	path := filepath.Join(t.TempDir(), "corrupt.db")
	if err := os.WriteFile(path, []byte("not a SQLite database"), 0o600); err != nil {
		t.Fatalf("write corrupt database: %v", err)
	}
	address := reserveTestAddress(t)
	process := startRuntimeProcess(t, buildRuntimeBinary(t), runtimeEnvironment("ADDR="+address, "JPROXY_DB_ENABLED=true", "JPROXY_DB_PATH="+path))

	// When
	exit := requireFailureWithoutListener(t, process, address)

	// Then
	if !exit.failedWith("database_open_or_migration_failed") {
		t.Fatalf("startup output = %s", exit.output)
	}
	t.Logf("process_pid=%d startup_failure=database_open_or_migration_failed listener_refused=true", exit.pid)
}
