package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"testing"
)

func TestOpen_rollsBackAndRecoversAfterRepeatedMigrationFailure(t *testing.T) {
	// Given
	path := javaFinalFixture(t)
	before := logicalDigest(t, path)
	failing := []migration{newMigration("0003_fails", `CREATE TABLE failure_probe (id INTEGER); INSERT INTO missing_table VALUES (1);`)}

	// When
	for attempt := 0; attempt < 2; attempt++ {
		if _, err := open(context.Background(), path, lifecycleOptions{migrations: failing}); err == nil {
			t.Fatalf("attempt %d open() error = nil, want migration failure", attempt+1)
		}
	}

	// Then
	if got := logicalDigest(t, path); got != before {
		t.Fatalf("source digest after repeated failures = %s, want %s", got, before)
	}
	assertTableAbsent(t, path, "failure_probe")
	assertLedgerAbsent(t, path)
	store, err := open(context.Background(), path, lifecycleOptions{migrations: embeddedMigrations})
	if err != nil {
		t.Fatalf("recovery open() error = %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("recovery Close() error = %v", err)
	}
	if got := appliedVersions(t, path); len(got) != len(embeddedMigrations) {
		t.Fatalf("recovery ledger = %v, want all migrations", got)
	}
}

func TestOpen_releasesLockAfterInspectionBackupAndMigrationFailures(t *testing.T) {
	for _, testCase := range []struct {
		name    string
		path    func(*testing.T) string
		options lifecycleOptions
	}{
		{
			name: "inspection",
			path: func(t *testing.T) string {
				path := javaFinalFixture(t)
				db := openTestDB(t, path)
				if _, err := db.Exec(`CREATE TABLE unexpected (id INTEGER PRIMARY KEY)`); err != nil {
					t.Fatalf("create unexpected table: %v", err)
				}
				closeTestDB(t, db)
				return path
			},
		},
		{
			name: "backup",
			path: javaFinalFixture,
			options: lifecycleOptions{backup: func(context.Context, *sql.DB, string) (string, error) {
				return "", errors.New("injected backup failure")
			}},
		},
		{
			name:    "migration",
			path:    javaFinalFixture,
			options: lifecycleOptions{migrations: []migration{newMigration("0003_fails", `CREATE TABLE failure_probe (id INTEGER); INSERT INTO missing_table VALUES (1);`)}},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			// Given
			path := testCase.path(t)

			// When
			if _, err := open(context.Background(), path, testCase.options); err == nil {
				t.Fatal("open() error = nil, want failure")
			}

			// Then
			assertSeparateProcessCanAcquireLock(t, path)
		})
	}
}

func TestOpen_joinsPrimaryAndCleanupErrors(t *testing.T) {
	// Given
	path := javaFinalFixture(t)
	primary := errors.New("primary backup failure")
	cleanup := errors.New("cleanup failure")
	options := lifecycleOptions{
		backup: func(context.Context, *sql.DB, string) (string, error) { return "", primary },
		closeLock: func(lock *writerLock) error {
			return errors.Join(lock.Close(), cleanup)
		},
	}

	// When
	_, err := open(context.Background(), path, options)

	// Then
	if !errors.Is(err, primary) || !errors.Is(err, cleanup) {
		t.Fatalf("open() error = %v, want primary and cleanup errors", err)
	}
	assertSeparateProcessCanAcquireLock(t, path)
}

func TestOpen_honorsCancellationDuringBackupAndMigration(t *testing.T) {
	for _, testCase := range []struct {
		name    string
		options func(context.CancelFunc) lifecycleOptions
	}{
		{
			name: "backup",
			options: func(cancel context.CancelFunc) lifecycleOptions {
				return lifecycleOptions{backup: func(ctx context.Context, db *sql.DB, path string) (string, error) {
					cancel()
					return createVerifiedBackup(ctx, db, path)
				}}
			},
		},
		{
			name: "migration",
			options: func(cancel context.CancelFunc) lifecycleOptions {
				return lifecycleOptions{beforeMigration: func(context.Context) { cancel() }}
			},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			// Given
			path := javaFinalFixture(t)
			before := logicalDigest(t, path)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// When
			_, err := open(ctx, path, testCase.options(cancel))

			// Then
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("open() error = %v, want context.Canceled", err)
			}
			if got := logicalDigest(t, path); got != before {
				t.Fatalf("source digest after cancellation = %s, want %s", got, before)
			}
			assertSeparateProcessCanAcquireLock(t, path)
			store, retryErr := Open(context.Background(), path)
			if retryErr != nil {
				t.Fatalf("Open() retry error = %v", retryErr)
			}
			if closeErr := store.Close(); closeErr != nil {
				t.Fatalf("retry Close() error = %v", closeErr)
			}
		})
	}
}

func TestOpen_backsUpThenUpgradesEmptyLedger(t *testing.T) {
	// Given
	path := javaFinalFixture(t)
	db := openTestDB(t, path)
	if _, err := db.Exec(`CREATE TABLE jproxy_go_migration (version TEXT PRIMARY KEY, checksum TEXT NOT NULL, applied_at TEXT NOT NULL)`); err != nil {
		t.Fatalf("create empty ledger: %v", err)
	}
	closeTestDB(t, db)

	// When
	store, err := Open(context.Background(), path)

	// Then
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if store.BackupPath() == "" {
		t.Fatal("BackupPath() = empty, want backup before empty-ledger upgrade")
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func assertTableAbsent(t *testing.T, path, table string) {
	t.Helper()
	db := openTestDB(t, path)
	defer closeTestDB(t, db)
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM sqlite_schema WHERE name = ?`, table).Scan(&count); err != nil {
		t.Fatalf("query table %s: %v", table, err)
	}
	if count != 0 {
		t.Fatalf("table %s count = %d, want 0", table, count)
	}
}

func assertLedgerAbsent(t *testing.T, path string) {
	t.Helper()
	assertTableAbsent(t, path, migrationLedgerTable)
}
