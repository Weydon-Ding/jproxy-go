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

type migration struct {
	version  string
	checksum string
	sql      string
}

type lifecycleOptions struct {
	migrations []migration
	backup     backupFunc
}

var embeddedMigrations = []migration{newMigration("0001_lifecycle", initialMigrationSQL)}

func newMigration(version, statement string) migration {
	digest := sha256.Sum256([]byte(statement))
	return migration{version: version, checksum: hex.EncodeToString(digest[:]), sql: statement}
}

func hasMigrationLedger(ctx context.Context, db *sql.DB) bool {
	var count int
	err := db.QueryRowContext(ctx, `SELECT count(*) FROM sqlite_schema WHERE type = 'table' AND name = ?`, migrationLedgerTable).Scan(&count)
	return err == nil && count == 1
}

func applyMigrations(ctx context.Context, db *sql.DB, migrations []migration) (err error) {
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
			if _, err := tx.ExecContext(ctx, `INSERT INTO jproxy_go_migration (version, checksum) VALUES (?, ?)`, migration.version, migration.checksum); err != nil {
				return fmt.Errorf("record SQLite migration %s: %w", migration.version, err)
			}
		case err != nil:
			return fmt.Errorf("read SQLite migration %s: %w", migration.version, err)
		case storedChecksum != migration.checksum:
			return fmt.Errorf("migration %s: stored %s, expected %s: %w", migration.version, storedChecksum, migration.checksum, ErrIncompatibleSchema)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit SQLite migrations: %w", err)
	}
	return nil
}
