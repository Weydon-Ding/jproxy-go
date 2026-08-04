package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"testing"
)

type recoveryEvidence struct {
	repeatedFailureRecovers       bool
	backupCancellationRecovers    bool
	migrationCancellationRecovers bool
	openFailureReleasesLock       bool
}

func measureRecoveryEvidence(t *testing.T) recoveryEvidence {
	t.Helper()
	return recoveryEvidence{
		repeatedFailureRecovers:       measureRepeatedMigrationRecovery(t),
		backupCancellationRecovers:    measureCancellationRecovery(t, true),
		migrationCancellationRecovers: measureCancellationRecovery(t, false),
		openFailureReleasesLock:       measureOpenFailureLockRelease(t),
	}
}

func measureRepeatedMigrationRecovery(t *testing.T) bool {
	t.Helper()
	path := javaFinalFixture(t)
	before := logicalDigest(t, path)
	failing := []migration{newMigration("0003_fails", `CREATE TABLE failure_probe (id INTEGER); INSERT INTO missing_table VALUES (1);`)}
	for range 2 {
		if _, err := open(context.Background(), path, lifecycleOptions{migrations: failing}); err == nil {
			return false
		}
	}
	if logicalDigest(t, path) != before {
		return false
	}
	assertTableAbsent(t, path, "failure_probe")
	assertLedgerAbsent(t, path)
	store, err := Open(context.Background(), path)
	if err != nil {
		return false
	}
	if err := store.Close(); err != nil {
		return false
	}
	return len(appliedVersions(t, path)) == len(embeddedMigrations)
}

func measureCancellationRecovery(t *testing.T, backup bool) bool {
	t.Helper()
	path := javaFinalFixture(t)
	before := logicalDigest(t, path)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	options := lifecycleOptions{}
	if backup {
		options.backup = func(ctx context.Context, db *sql.DB, path string) (string, error) {
			cancel()
			return createVerifiedBackup(ctx, db, path)
		}
	} else {
		options.beforeMigration = func(context.Context) { cancel() }
	}
	_, err := open(ctx, path, options)
	if !errors.Is(err, context.Canceled) || logicalDigest(t, path) != before {
		return false
	}
	assertSeparateProcessCanAcquireLock(t, path)
	store, retryErr := Open(context.Background(), path)
	if retryErr != nil {
		return false
	}
	return store.Close() == nil
}

func measureOpenFailureLockRelease(t *testing.T) bool {
	t.Helper()
	path := javaFinalFixture(t)
	db := openTestDB(t, path)
	if _, err := db.Exec(`CREATE TABLE unexpected (id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatalf("create inspection failure fixture: %v", err)
	}
	closeTestDB(t, db)
	if _, err := Open(context.Background(), path); err == nil {
		return false
	}
	assertSeparateProcessCanAcquireLock(t, path)
	return true
}
