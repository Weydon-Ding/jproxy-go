package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// UpdateSystemConfigs writes one complete configuration set and derives its
// formatter snapshot before the single transaction commits.
func (s *Store) UpdateSystemConfigs(ctx context.Context, values []SystemConfig) (snapshot Snapshot, err error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Snapshot{}, fmt.Errorf("begin system config transaction: %w", err)
	}
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			err = errors.Join(err, fmt.Errorf("rollback system config transaction: %w", rollbackErr))
		}
	}()
	for _, value := range values {
		if err := javaInteger(int64(value.ID)); err != nil {
			return Snapshot{}, err
		}
		if err := validStatus(value.ValidStatus); err != nil {
			return Snapshot{}, err
		}
		if err := upsertSystemConfig(ctx, tx, value); err != nil {
			return Snapshot{}, err
		}
	}
	snapshot, err = loadSnapshot(ctx, tx)
	if err != nil {
		return Snapshot{}, err
	}
	if err := ctx.Err(); err != nil {
		return Snapshot{}, err
	}
	if err := tx.Commit(); err != nil {
		return Snapshot{}, fmt.Errorf("commit system config transaction: %w", err)
	}
	return snapshot, nil
}
