package sqlite

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

type lifecycleEvidence struct {
	Commands                      []string `json:"commands"`
	Platform                      string   `json:"platform"`
	BackupHash                    string   `json:"backup_sha256"`
	BackupOpenable                bool     `json:"backup_openable"`
	LedgerVersions                []string `json:"ledger_versions"`
	PreSourceDigest               string   `json:"pre_source_logical_digest"`
	BackupDigest                  string   `json:"backup_logical_digest"`
	RestoredDigest                string   `json:"restored_logical_digest"`
	SecondBackupPath              string   `json:"second_open_backup_path"`
	WALCommitted                  bool     `json:"wal_committed_non_empty"`
	WALSourceDigest               string   `json:"wal_source_logical_digest"`
	WALBackupDigest               string   `json:"wal_backup_logical_digest"`
	WALFailurePreservesSource     bool     `json:"wal_failure_preserves_source"`
	WALRetryRestoresSource        bool     `json:"wal_retry_restore_matches_source"`
	WriterLockRejected            bool     `json:"cross_process_writer_lock_rejected"`
	RepeatedFailureRecovers       bool     `json:"repeated_migration_failure_recovers"`
	BackupCancellationRecovers    bool     `json:"backup_cancellation_recovers"`
	MigrationCancellationRecovers bool     `json:"migration_cancellation_recovers"`
	OpenFailureReleasesLock       bool     `json:"open_failure_releases_lock"`
}

func TestLifecycleEvidence_disposableJavaFixture(t *testing.T) {
	evidencePath := os.Getenv("JPROXY_SQLITE_EVIDENCE_PATH")
	if evidencePath == "" {
		t.Skip("JPROXY_SQLITE_EVIDENCE_PATH is not set")
	}

	// Given
	path := javaFixture(t)
	preDigest := logicalDigest(t, path)

	// When
	first, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("first Open() error = %v", err)
	}
	backupPath := first.BackupPath()
	if err := first.Close(); err != nil {
		t.Fatalf("first Close() error = %v", err)
	}
	second, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("second Open() error = %v", err)
	}
	secondBackupPath := second.BackupPath()
	if err := second.Close(); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}
	restored := filepath.Join(t.TempDir(), "restore.db")
	bytes, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}
	if err := os.WriteFile(restored, bytes, 0o600); err != nil {
		t.Fatalf("write restore: %v", err)
	}

	// Then
	_, backupErr := Load(context.Background(), backupPath)
	wal := measureWALEvidence(t)
	recovery := measureRecoveryEvidence(t)
	lockPath := javaFinalFixture(t)
	command := startLockHelper(t, lockPath)
	_, lockErr := Open(context.Background(), lockPath)
	stopLockHelper(t, command)
	evidence := lifecycleEvidence{
		Commands: []string{
			"go test ./internal/store/sqlite -count=10 -shuffle=on",
			"go test ./... -count=1",
			"go build ./cmd/jproxy",
			"go vet ./...",
			"golangci-lint run",
		},
		Platform:                      runtime.GOOS + "/" + runtime.GOARCH,
		BackupHash:                    fileHash(t, backupPath),
		BackupOpenable:                backupErr == nil,
		LedgerVersions:                appliedVersions(t, path),
		PreSourceDigest:               preDigest,
		BackupDigest:                  logicalDigest(t, backupPath),
		RestoredDigest:                logicalDigest(t, restored),
		SecondBackupPath:              secondBackupPath,
		WALCommitted:                  wal.committed,
		WALSourceDigest:               wal.sourceDigest,
		WALBackupDigest:               wal.backupDigest,
		WALFailurePreservesSource:     wal.failurePreservesSource,
		WALRetryRestoresSource:        wal.retryRestoresSource,
		WriterLockRejected:            errors.Is(lockErr, ErrWriterOwned),
		RepeatedFailureRecovers:       recovery.repeatedFailureRecovers,
		BackupCancellationRecovers:    recovery.backupCancellationRecovers,
		MigrationCancellationRecovers: recovery.migrationCancellationRecovers,
		OpenFailureReleasesLock:       recovery.openFailureReleasesLock,
	}
	data, err := json.MarshalIndent(evidence, "", "  ")
	if err != nil {
		t.Fatalf("marshal evidence: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(evidencePath), 0o700); err != nil {
		t.Fatalf("create evidence directory: %v", err)
	}
	if err := os.WriteFile(evidencePath, append(data, '\n'), 0o600); err != nil {
		t.Fatalf("write evidence: %v", err)
	}
}

type walEvidence struct {
	committed              bool
	sourceDigest           string
	backupDigest           string
	failurePreservesSource bool
	retryRestoresSource    bool
}

func measureWALEvidence(t *testing.T) walEvidence {
	t.Helper()
	path := javaFinalFixture(t)
	db := openTestDB(t, path)
	if _, err := db.Exec(`PRAGMA journal_mode = WAL`); err != nil {
		t.Fatalf("enable WAL: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO system_user (id, username) VALUES (99, 'evidence-wal')`); err != nil {
		t.Fatalf("insert WAL row: %v", err)
	}
	info, err := os.Stat(path + "-wal")
	if err != nil || info.Size() == 0 {
		t.Fatalf("committed WAL = %v, size = %d", err, info.Size())
	}
	sourceDigest := logicalDigest(t, path)
	invalidTarget := filepath.Join(t.TempDir(), "occupied")
	if err := os.Mkdir(invalidTarget, 0o700); err != nil {
		t.Fatalf("create occupied target: %v", err)
	}
	if _, err := createVerifiedBackupTo(context.Background(), db, invalidTarget); err == nil {
		t.Fatal("WAL backup failure = nil")
	}
	failurePreservesSource := logicalDigest(t, path) == sourceDigest
	backupPath, err := createVerifiedBackupTo(context.Background(), db, path+".wal-backup.db")
	if err != nil {
		t.Fatalf("retry WAL backup: %v", err)
	}
	backupDigest := logicalDigest(t, backupPath)
	restored := filepath.Join(t.TempDir(), "wal-restored.db")
	data, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("read WAL backup: %v", err)
	}
	if err := os.WriteFile(restored, data, 0o600); err != nil {
		t.Fatalf("write WAL restore: %v", err)
	}
	closeTestDB(t, db)
	return walEvidence{
		committed:              true,
		sourceDigest:           sourceDigest,
		backupDigest:           backupDigest,
		failurePreservesSource: failurePreservesSource,
		retryRestoresSource:    logicalDigest(t, restored) == sourceDigest,
	}
}
