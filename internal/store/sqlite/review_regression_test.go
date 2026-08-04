package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
)

func TestOpen_createsBackupBeforeRejectingMalformedLedger(t *testing.T) {
	// Given
	path := javaFinalFixture(t)
	db := openTestDB(t, path)
	if _, err := db.Exec(`CREATE TABLE jproxy_go_migration (version TEXT)`); err != nil {
		t.Fatalf("create malformed ledger: %v", err)
	}
	closeTestDB(t, db)

	// When
	_, err := Open(context.Background(), path)

	// Then
	if err == nil {
		t.Fatal("Open() error = nil, want malformed ledger rejection")
	}
	backups, err := filepath.Glob(path + ".bak-*")
	if err != nil {
		t.Fatalf("glob backups: %v", err)
	}
	if len(backups) != 1 {
		t.Fatalf("backup count = %d, want 1", len(backups))
	}
}

func TestOpen_initializesCompleteJavaFinalSchema(t *testing.T) {
	// Given
	path := filepath.Join(t.TempDir(), "empty.db")

	// When
	store, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	// Then
	db := openTestDB(t, path)
	defer closeTestDB(t, db)
	for _, contract := range javaFinalTables {
		var count int
		if err := db.QueryRow(`SELECT count(*) FROM sqlite_schema WHERE type = 'table' AND name = ?`, contract.name).Scan(&count); err != nil {
			t.Fatalf("query table %s: %v", contract.name, err)
		}
		if count != 1 {
			t.Fatalf("table %s count = %d, want 1", contract.name, count)
		}
	}
}

func TestOpen_upgradesCompleteSchemaWithValidPriorLedger(t *testing.T) {
	// Given
	path := javaFinalFixture(t)
	db := openTestDB(t, path)
	if _, err := db.Exec(`CREATE TABLE jproxy_go_migration (version TEXT PRIMARY KEY, checksum TEXT NOT NULL, applied_at TEXT NOT NULL)`); err != nil {
		t.Fatalf("create ledger: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO jproxy_go_migration (version, checksum, applied_at) VALUES (?, ?, 'now')`, embeddedMigrations[0].version, embeddedMigrations[0].checksum); err != nil {
		t.Fatalf("seed prior ledger: %v", err)
	}
	closeTestDB(t, db)

	// When
	store, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	// Then
	if store.BackupPath() == "" {
		t.Fatal("BackupPath() = empty, want pre-upgrade backup")
	}
	versions := appliedVersions(t, path)
	if len(versions) != len(embeddedMigrations) || versions[1] != "0002_examples" {
		t.Fatalf("ledger versions = %v, want [0001_lifecycle 0002_examples]", versions)
	}
}

func TestOpen_rejectsUnknownPreGoTableAfterBackup(t *testing.T) {
	path := javaFinalFixture(t)
	before := logicalDigest(t, path)
	db := openTestDB(t, path)
	if _, err := db.Exec(`CREATE TABLE unknown_pre_go (id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatalf("create unknown table: %v", err)
	}
	closeTestDB(t, db)
	if _, err := Open(context.Background(), path); err == nil {
		t.Fatal("Open() error = nil, want unknown table rejection")
	}
	if got := logicalDigest(t, path); got != before {
		t.Fatalf("source logical digest = %s, want %s", got, before)
	}
}

func TestOpen_rejectsUnknownLedgerEntryAfterBackup(t *testing.T) {
	// Given
	path := javaFinalFixture(t)
	db := openTestDB(t, path)
	if _, err := db.Exec(`CREATE TABLE jproxy_go_migration (version TEXT PRIMARY KEY, checksum TEXT NOT NULL, applied_at TEXT NOT NULL)`); err != nil {
		t.Fatalf("create ledger: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO jproxy_go_migration VALUES ('unknown', 'unknown', 'now')`); err != nil {
		t.Fatalf("seed unknown ledger: %v", err)
	}
	closeTestDB(t, db)

	// When
	_, err := Open(context.Background(), path)

	// Then
	if !errors.Is(err, ErrIncompatibleSchema) {
		t.Fatalf("Open() error = %v, want ErrIncompatibleSchema", err)
	}
	backups, globErr := filepath.Glob(path + ".bak-*")
	if globErr != nil {
		t.Fatalf("glob backups: %v", globErr)
	}
	if len(backups) != 1 {
		t.Fatalf("backup count = %d, want 1", len(backups))
	}
}

func openTestDB(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	return db
}

func closeTestDB(t *testing.T, db *sql.DB) {
	t.Helper()
	if err := db.Close(); err != nil {
		t.Fatalf("close test database: %v", err)
	}
}
