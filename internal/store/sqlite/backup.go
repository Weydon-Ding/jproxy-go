package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

func createVerifiedBackup(ctx context.Context, db *sql.DB, databasePath string) (string, error) {
	backupPath := databasePath + ".bak-" + time.Now().UTC().Format("20060102T150405.000000000Z")
	return createVerifiedBackupTo(ctx, db, backupPath)
}

func createVerifiedBackupTo(ctx context.Context, db *sql.DB, backupPath string) (string, error) {
	quotedPath := strings.ReplaceAll(filepath.ToSlash(backupPath), "'", "''")
	if _, err := db.ExecContext(ctx, "VACUUM INTO '"+quotedPath+"'"); err != nil {
		return "", fmt.Errorf("create SQLite backup: %w", err)
	}
	backup, err := openReadOnly(ctx, backupPath)
	if err != nil {
		return "", fmt.Errorf("open SQLite backup: %w", err)
	}
	defer backup.Close()
	var integrity string
	if err := backup.QueryRowContext(ctx, `PRAGMA integrity_check`).Scan(&integrity); err != nil {
		return "", fmt.Errorf("check SQLite backup integrity: %w", err)
	}
	if integrity != "ok" {
		return "", fmt.Errorf("check SQLite backup integrity: got %q", integrity)
	}
	backupTables, err := tableSet(ctx, backup)
	if err != nil {
		return "", err
	}
	if err := validateJavaSchema(ctx, backup, backupTables); err != nil {
		return "", fmt.Errorf("verify SQLite backup schema: %w", err)
	}
	if err := verifyTableCounts(ctx, db, backup); err != nil {
		return "", err
	}
	return backupPath, nil
}

func verifyTableCounts(ctx context.Context, source, backup *sql.DB) error {
	for _, contract := range javaTables {
		var sourceCount, backupCount int
		query := `SELECT count(*) FROM "` + contract.name + `"`
		if err := source.QueryRowContext(ctx, query).Scan(&sourceCount); err != nil {
			return fmt.Errorf("count source %s for backup verification: %w", contract.name, err)
		}
		if err := backup.QueryRowContext(ctx, query).Scan(&backupCount); err != nil {
			return fmt.Errorf("count backup %s for backup verification: %w", contract.name, err)
		}
		if sourceCount != backupCount {
			return fmt.Errorf("verify backup %s row count: got %d, want %d", contract.name, backupCount, sourceCount)
		}
	}
	return nil
}
