package sqlite

import (
	"context"
	"crypto/sha256"
	"database/sql"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
)

const migrationLedgerTable = "jproxy_go_migration"

//go:embed migrations/0001_lifecycle.sql
var initialMigrationSQL string

//go:embed migrations/0002_examples.sql
var exampleMigrationSQL string

type migration struct {
	version  string
	checksum string
	sql      string
}

type lifecycleOptions struct {
	migrations      []migration
	backup          backupFunc
	beforeMigration func(context.Context)
	closeLock       func(*writerLock) error
}

var embeddedMigrations = []migration{
	newMigration("0001_lifecycle", initialMigrationSQL),
	newMigration("0002_examples", exampleMigrationSQL),
}

func newMigration(version, statement string) migration {
	digest := sha256.Sum256([]byte(statement))
	return migration{version: version, checksum: hex.EncodeToString(digest[:]), sql: statement}
}

func validateLedger(ctx context.Context, db *sql.DB) (bool, error) {
	if err := requireColumns(ctx, db, migrationLedgerTable, []string{"version", "checksum", "applied_at"}); err != nil {
		return false, fmt.Errorf("validate Go migration ledger schema: %w", err)
	}
	rows, err := db.QueryContext(ctx, `SELECT version, checksum FROM jproxy_go_migration ORDER BY version`)
	if err != nil {
		return false, fmt.Errorf("read Go migration ledger: %w", err)
	}
	defer rows.Close()
	known := map[string]string{}
	for _, migration := range embeddedMigrations {
		known[migration.version] = migration.checksum
	}
	seen := map[string]bool{}
	for rows.Next() {
		var version, checksum string
		if err := rows.Scan(&version, &checksum); err != nil {
			return false, fmt.Errorf("scan Go migration ledger: %w", err)
		}
		if expected, ok := known[version]; !ok || expected != checksum || seen[version] {
			return false, fmt.Errorf("migration %s ledger entry: %w", version, ErrIncompatibleSchema)
		}
		seen[version] = true
	}
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("read Go migration ledger rows: %w", err)
	}
	return len(seen) == len(known), nil
}

func applyMigrationsAfterHook(ctx context.Context, db *sql.DB, migrations []migration, beforeCommit func(context.Context)) (err error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin SQLite migration transaction: %w", err)
	}
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) && err == nil {
			err = fmt.Errorf("rollback SQLite migration transaction: %w", rollbackErr)
		}
	}()
	if _, err := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS jproxy_go_migration (version TEXT PRIMARY KEY, checksum TEXT NOT NULL, applied_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP)`); err != nil {
		return fmt.Errorf("create Go migration ledger: %w", err)
	}
	known := make(map[string]string, len(migrations))
	for _, migration := range migrations {
		known[migration.version] = migration.checksum
	}
	rows, err := tx.QueryContext(ctx, `SELECT version, checksum FROM jproxy_go_migration ORDER BY version`)
	if err != nil {
		return fmt.Errorf("read SQLite migration ledger: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var version, checksum string
		if err := rows.Scan(&version, &checksum); err != nil {
			return fmt.Errorf("scan SQLite migration ledger: %w", err)
		}
		if expected, ok := known[version]; !ok || checksum != expected {
			return fmt.Errorf("migration %s ledger entry: %w", version, ErrIncompatibleSchema)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read SQLite migration ledger rows: %w", err)
	}
	for _, migration := range migrations {
		var storedChecksum string
		err := tx.QueryRowContext(ctx, `SELECT checksum FROM jproxy_go_migration WHERE version = ?`, migration.version).Scan(&storedChecksum)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			if _, err := tx.ExecContext(ctx, migration.sql); err != nil {
				return fmt.Errorf("apply SQLite migration %s: %w", migration.version, err)
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO jproxy_go_migration (version, checksum, applied_at) VALUES (?, ?, CURRENT_TIMESTAMP)`, migration.version, migration.checksum); err != nil {
				return fmt.Errorf("record SQLite migration %s: %w", migration.version, err)
			}
		case err != nil:
			return fmt.Errorf("read SQLite migration %s: %w", migration.version, err)
		case storedChecksum != migration.checksum:
			return fmt.Errorf("migration %s: stored %s, expected %s: %w", migration.version, storedChecksum, migration.checksum, ErrIncompatibleSchema)
		}
	}
	if beforeCommit != nil {
		beforeCommit(ctx)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit SQLite migrations: %w", err)
	}
	return nil
}
