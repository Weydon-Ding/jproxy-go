package sqlite

import (
	"context"
	"database/sql"
	"fmt"
)

type DatasetTransaction interface {
	ReplaceRadarrTitles(context.Context, RadarrTitleBatch) error
}
type datasetTransaction struct{ tx *sql.Tx }

func (s *Store) InTransaction(ctx context.Context, fn func(DatasetTransaction) error) (err error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin dataset transaction: %w", err)
	}
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && rollbackErr != sql.ErrTxDone && err == nil {
			err = fmt.Errorf("rollback dataset transaction: %w", rollbackErr)
		}
	}()
	if err = fn(datasetTransaction{tx}); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit dataset transaction: %w", err)
	}
	return nil
}
func (t datasetTransaction) ReplaceRadarrTitles(ctx context.Context, batch RadarrTitleBatch) error {
	if len(batch.Rows) > batchLimit {
		return ErrBatchTooLarge
	}
	if _, err := t.tx.ExecContext(ctx, `DELETE FROM radarr_title`); err != nil {
		return fmt.Errorf("clear radarr titles: %w", err)
	}
	for _, row := range batch.Rows {
		if err := upsertRadarr(ctx, transactionDB(t), row); err != nil {
			return err
		}
	}
	return nil
}

type transactionDB struct{ tx *sql.Tx }

func (d transactionDB) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return d.tx.ExecContext(ctx, query, args...)
}
