package sqlite

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestOpen_initializesEmptyDatabase(t *testing.T) {
	// Given
	path := filepath.Join(t.TempDir(), "new.db")

	// When
	store, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	// Then
	if got := appliedVersions(t, path); len(got) == 0 {
		t.Fatal("migration ledger is empty")
	}
	backups, err := filepath.Glob(path + ".bak-*")
	if err != nil {
		t.Fatalf("glob backups: %v", err)
	}
	if len(backups) != 0 {
		t.Fatalf("empty database backup = %v, want none", err)
	}
}

func TestOpen_backsUpJavaSchemaBeforeMigration(t *testing.T) {
	// Given
	path := javaFixture(t)
	beforeCounts := tableCounts(t, path)

	// When
	store, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	// Then
	backup := store.BackupPath()
	if backup == "" {
		t.Fatal("BackupPath() is empty")
	}
	if got := tableCounts(t, backup); !sameCounts(got, beforeCounts) {
		t.Fatalf("backup table counts = %#v, want %#v", got, beforeCounts)
	}
	backupHash := fileHash(t, backup)
	if _, err := Load(context.Background(), backup); err != nil {
		t.Fatalf("Load(backup) error = %v", err)
	}
	if got := fileHash(t, backup); got != backupHash {
		t.Fatalf("backup hash changed after read: got %s, want %s", got, backupHash)
	}
	restored := filepath.Join(t.TempDir(), "restored.db")
	backupBytes, err := os.ReadFile(backup)
	if err != nil {
		t.Fatalf("read backup for restore: %v", err)
	}
	if err := os.WriteFile(restored, backupBytes, 0o600); err != nil {
		t.Fatalf("restore backup copy: %v", err)
	}
	if got := tableCounts(t, restored); !sameCounts(got, beforeCounts) {
		t.Fatalf("restored table counts = %#v, want %#v", got, beforeCounts)
	}
}

func TestOpen_acceptsJavaSchemaWithLiquibaseDefaultTableNames(t *testing.T) {
	// Given
	path := javaFixture(t)
	db := openTestDB(t, path)
	for _, statement := range []string{
		`CREATE TABLE DATABASECHANGELOG (ID TEXT NOT NULL)`,
		`CREATE TABLE DATABASECHANGELOGLOCK (ID INTEGER NOT NULL PRIMARY KEY, LOCKED BOOLEAN NOT NULL)`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("create Liquibase metadata table: %v", err)
		}
	}
	closeTestDB(t, db)

	// When
	store, err := Open(context.Background(), path)

	// Then
	if err != nil {
		t.Fatalf("Open() error = %v, want Liquibase default tables accepted", err)
	}
	t.Cleanup(func() { _ = store.Close() })
}

func TestOpen_appliesMigrationsOnlyOnce_whenReopened(t *testing.T) {
	// Given
	path := javaFixture(t)
	first, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("first Open() error = %v", err)
	}
	firstBackup := first.BackupPath()
	if err := first.Close(); err != nil {
		t.Fatalf("first Close() error = %v", err)
	}

	// When
	second, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("second Open() error = %v", err)
	}
	t.Cleanup(func() { _ = second.Close() })

	// Then
	if second.BackupPath() != "" {
		t.Fatalf("second BackupPath() = %q, want empty after ledger exists", second.BackupPath())
	}
	if _, err := os.Stat(firstBackup); err != nil {
		t.Fatalf("first backup missing: %v", err)
	}
}
