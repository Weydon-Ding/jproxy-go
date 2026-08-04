package sqlite

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type lifecycleEvidence struct {
	BackupHash       string         `json:"backup_sha256"`
	BackupOpenable   bool           `json:"backup_openable"`
	LedgerVersions   []string       `json:"ledger_versions"`
	PreMigrationRows map[string]int `json:"pre_migration_rows"`
	RestoredRows     map[string]int `json:"restored_rows"`
	SecondBackupPath string         `json:"second_open_backup_path"`
}

func TestLifecycleEvidence_disposableJavaFixture(t *testing.T) {
	evidencePath := os.Getenv("JPROXY_SQLITE_EVIDENCE_PATH")
	if evidencePath == "" {
		t.Skip("JPROXY_SQLITE_EVIDENCE_PATH is not set")
	}

	// Given
	path := javaFixture(t)
	preRows := tableCounts(t, path)

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
	evidence := lifecycleEvidence{
		BackupHash:       fileHash(t, backupPath),
		BackupOpenable:   backupErr == nil,
		LedgerVersions:   appliedVersions(t, path),
		PreMigrationRows: preRows,
		RestoredRows:     tableCounts(t, restored),
		SecondBackupPath: secondBackupPath,
	}
	data, err := json.MarshalIndent(evidence, "", "  ")
	if err != nil {
		t.Fatalf("marshal evidence: %v", err)
	}
	if err := os.WriteFile(evidencePath, append(data, '\n'), 0o600); err != nil {
		t.Fatalf("write evidence: %v", err)
	}
}
