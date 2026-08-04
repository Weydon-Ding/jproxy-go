package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

var (
	// ErrIncompatibleSchema means an existing database is neither empty nor a supported Java JProxy schema.
	ErrIncompatibleSchema = errors.New("incompatible SQLite schema")
	// ErrWriterOwned means another cooperating jproxy-go process holds the writable lifecycle lock.
	ErrWriterOwned = errors.New("SQLite writer is already owned")
)

// Store owns a writable SQLite lifecycle. Its advisory lock is released by Close.
// The lock coordinates cooperating jproxy-go processes only; Java must be stopped during cutover.
type Store struct {
	db         *sql.DB
	lock       *writerLock
	backupPath string
}

type backupFunc func(context.Context, *sql.DB, string) (string, error)

// Open validates, backs up, and migrates path. Load remains the separate read-only snapshot API.
func Open(ctx context.Context, path string) (*Store, error) {
	return open(ctx, path, lifecycleOptions{migrations: embeddedMigrations, backup: createVerifiedBackup})
}

func open(ctx context.Context, path string, options lifecycleOptions) (_ *Store, err error) {
	if options.backup == nil {
		options.backup = createVerifiedBackup
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("open writable SQLite lifecycle: %w", err)
	}
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve SQLite database path: %w", err)
	}
	exists, err := pathExists(absolutePath)
	if err != nil {
		return nil, err
	}
	lock, err := acquireWriterLock(absolutePath)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, lock.Close())
		}
	}()
	db, err := openWritable(ctx, absolutePath)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, db.Close())
		}
	}()

	store := &Store{db: db, lock: lock}
	inspection, inspectErr := inspectSchema(ctx, db)
	if exists && inspection.kind == inspectionJavaUnmanaged {
		backupPath, backupErr := options.backup(ctx, db, absolutePath)
		if backupErr != nil {
			return nil, backupErr
		}
		store.backupPath = backupPath
	}
	if inspectErr != nil {
		return nil, fmt.Errorf("inspect existing database: %w", inspectErr)
	}
	if err := configureJournal(ctx, db); err != nil {
		return nil, err
	}
	if err := applyMigrations(ctx, db, options.migrations); err != nil {
		return nil, err
	}
	return store, nil
}

// Close releases the database handle and the process-scoped advisory writer lock.
func (s *Store) Close() error {
	closeErr := s.db.Close()
	lockErr := s.lock.Close()
	return errors.Join(closeErr, lockErr)
}

// BackupPath reports the verified pre-migration backup created by this Open call.
func (s *Store) BackupPath() string { return s.backupPath }

func pathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, fmt.Errorf("stat SQLite database: %w", err)
}
