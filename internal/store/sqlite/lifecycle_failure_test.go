package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestOpen_rejectsIncompatibleOrCorruptDatabase(t *testing.T) {
	for _, testCase := range []struct {
		name string
		data []byte
	}{{name: "corrupt", data: []byte("not sqlite")}, {name: "unknown schema"}} {
		t.Run(testCase.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "input.db")
			if testCase.data != nil {
				if err := os.WriteFile(path, testCase.data, 0o600); err != nil {
					t.Fatalf("write fixture: %v", err)
				}
			} else {
				db, err := sql.Open("sqlite", path)
				if err != nil {
					t.Fatalf("open fixture: %v", err)
				}
				if _, err := db.Exec(`CREATE TABLE unexpected (id INTEGER PRIMARY KEY)`); err != nil {
					t.Fatalf("create drift fixture: %v", err)
				}
				if err := db.Close(); err != nil {
					t.Fatalf("close fixture: %v", err)
				}
			}
			before := fileHash(t, path)
			if _, err := Open(context.Background(), path); err == nil {
				t.Fatal("Open() error = nil, want rejection")
			}
			if got := fileHash(t, path); got != before {
				t.Fatalf("input hash = %s, want %s", got, before)
			}
		})
	}
}

func TestOpen_preservesDatabase_whenMigrationFails(t *testing.T) {
	path := javaFixture(t)
	before := tableCounts(t, path)
	failing := []migration{newMigration("0001_fails", `CREATE TABLE failure_probe (id INTEGER); INSERT INTO missing_table VALUES (1);`)}
	if _, err := open(context.Background(), path, lifecycleOptions{migrations: failing}); err == nil {
		t.Fatal("open() error = nil, want forced migration failure")
	}
	if got := tableCounts(t, path); !sameCounts(got, before) {
		t.Fatalf("post-failure table counts = %#v, want %#v", got, before)
	}
}

func TestOpen_honorsCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Open(ctx, filepath.Join(t.TempDir(), "cancelled.db")); !errors.Is(err, context.Canceled) {
		t.Fatalf("Open() error = %v, want context.Canceled", err)
	}
}

func TestOpen_leavesOriginalUnchanged_whenBackupFails(t *testing.T) {
	path := javaFixture(t)
	before := fileHash(t, path)
	backupFailure := func(context.Context, *sql.DB, string) (string, error) {
		return "", errors.New("backup target is unwritable")
	}
	if _, err := open(context.Background(), path, lifecycleOptions{migrations: embeddedMigrations, backup: backupFailure}); err == nil {
		t.Fatal("open() error = nil, want backup failure")
	}
	if got := fileHash(t, path); got != before {
		t.Fatalf("input hash = %s, want %s", got, before)
	}
}

func TestOpen_rejectsConcurrentWriter(t *testing.T) {
	path := javaFixture(t)
	first, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("first Open() error = %v", err)
	}
	t.Cleanup(func() { _ = first.Close() })
	if _, err = Open(context.Background(), path); !errors.Is(err, ErrWriterOwned) {
		t.Fatalf("Open() error = %v, want ErrWriterOwned", err)
	}
}

func TestOpen_rejectsSchemaMissingRequiredIndex(t *testing.T) {
	path := javaFixture(t)
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	if _, err := db.Exec(`DROP INDEX sonarr_title_clean_title_idx`); err != nil {
		t.Fatalf("drop required index: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close fixture: %v", err)
	}
	before := fileHash(t, path)
	if _, err = Open(context.Background(), path); !errors.Is(err, ErrIncompatibleSchema) {
		t.Fatalf("Open() error = %v, want ErrIncompatibleSchema", err)
	}
	if got := fileHash(t, path); got != before {
		t.Fatalf("input hash = %s, want %s", got, before)
	}
}

func TestOpen_rejectsMigrationLedgerChecksumDrift(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ledger.db")
	store, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open ledger fixture: %v", err)
	}
	if _, err := db.Exec(`UPDATE jproxy_go_migration SET checksum = 'drift'`); err != nil {
		t.Fatalf("drift ledger checksum: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close ledger fixture: %v", err)
	}
	if _, err = Open(context.Background(), path); !errors.Is(err, ErrIncompatibleSchema) {
		t.Fatalf("Open() error = %v, want ErrIncompatibleSchema", err)
	}
}
