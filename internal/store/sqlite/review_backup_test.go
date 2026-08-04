package sqlite

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCreateVerifiedBackup_preservesUncheckpointedWALContent(t *testing.T) {
	path := javaFinalFixture(t)
	db := openTestDB(t, path)
	if _, err := db.Exec(`PRAGMA journal_mode = WAL`); err != nil {
		t.Fatalf("enable WAL: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO system_user (id, username) VALUES (99, 'wal-row')`); err != nil {
		t.Fatalf("write WAL row: %v", err)
	}
	before := logicalDigest(t, path)
	backupPath, err := createVerifiedBackupTo(context.Background(), db, path+".backup.db")
	if err != nil {
		t.Fatalf("createVerifiedBackupTo() error = %v", err)
	}
	if got := logicalDigest(t, backupPath); got != before {
		t.Fatalf("backup logical digest = %s, want %s", got, before)
	}
	closeTestDB(t, db)
}

func TestCreateVerifiedBackupTo_leavesSourceUntouched_whenTargetInvalid(t *testing.T) {
	path := javaFinalFixture(t)
	before := logicalDigest(t, path)
	db := openTestDB(t, path)
	invalidTarget := filepath.Join(t.TempDir(), "existing-directory")
	if err := os.Mkdir(invalidTarget, 0o700); err != nil {
		t.Fatalf("create invalid target: %v", err)
	}
	_, err := createVerifiedBackupTo(context.Background(), db, invalidTarget)
	closeTestDB(t, db)
	if err == nil {
		t.Fatal("createVerifiedBackupTo() error = nil, want target failure")
	}
	if got := logicalDigest(t, path); got != before {
		t.Fatalf("source logical digest = %s, want %s", got, before)
	}
}

func TestCreateVerifiedBackupTo_rejectsExistingTarget(t *testing.T) {
	path := javaFinalFixture(t)
	before := logicalDigest(t, path)
	db := openTestDB(t, path)
	target := filepath.Join(t.TempDir(), "collision.db")
	if err := os.WriteFile(target, []byte("occupied"), 0o600); err != nil {
		t.Fatalf("seed collision target: %v", err)
	}
	if _, err := createVerifiedBackupTo(context.Background(), db, target); err == nil {
		closeTestDB(t, db)
		t.Fatal("createVerifiedBackupTo() error = nil, want collision rejection")
	}
	closeTestDB(t, db)
	if got := logicalDigest(t, path); got != before {
		t.Fatalf("source logical digest = %s, want %s", got, before)
	}
}

func TestOpen_rejectsLedgerChecksumDriftAfterVerifiedBackup(t *testing.T) {
	path := javaFinalFixture(t)
	db := openTestDB(t, path)
	if _, err := db.Exec(`CREATE TABLE jproxy_go_migration (version TEXT PRIMARY KEY, checksum TEXT NOT NULL, applied_at TEXT NOT NULL)`); err != nil {
		t.Fatalf("create ledger: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO jproxy_go_migration VALUES ('0001_lifecycle', 'drift', 'now')`); err != nil {
		t.Fatalf("seed drift ledger: %v", err)
	}
	closeTestDB(t, db)
	_, err := Open(context.Background(), path)
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
