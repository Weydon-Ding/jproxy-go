package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

var ErrNilTransactionCallback = errors.New("SQLite transaction callback is nil")

type DatasetTransaction interface {
	ReplaceRadarrTitles(context.Context, RadarrTitleBatch) error
	UpsertRadarrTitles(context.Context, RadarrTitleBatch) error
	DeleteRadarrTitles(context.Context, RadarrTitleIDs) error
	UpsertSonarrRules(context.Context, SonarrRuleBatch) error
	UpsertRemoteSonarrRules(context.Context, []SonarrRuleInput) error
	DeleteSonarrRules(context.Context, RuleIDs) error
	SwitchSonarrRuleStatus(context.Context, RuleIDs, ValidStatus) error
	ReplaceSonarrRules(context.Context, SonarrRuleBatch) error
	UpsertRadarrRules(context.Context, RadarrRuleBatch) error
	UpsertRemoteRadarrRules(context.Context, []RadarrRuleInput) error
	DeleteRadarrRules(context.Context, RuleIDs) error
	SwitchRadarrRuleStatus(context.Context, RuleIDs, ValidStatus) error
	ReplaceRadarrRules(context.Context, RadarrRuleBatch) error
	UpsertSonarrTitles(context.Context, SonarrTitleBatch) error
	DeleteSonarrTitles(context.Context, SonarrTitleIDs) error
	ReplaceSonarrTitles(context.Context, SonarrTitleBatch) error
	UpsertTMDBTitles(context.Context, TMDBTitleBatch) error
	DeleteTMDBTitles(context.Context, TMDBTitleIDs) error
	ReplaceTMDBTitles(context.Context, TMDBTitleBatch) error
}

type datasetTransaction struct{ tx *sql.Tx }

func (s *Store) InTransaction(ctx context.Context, fn func(DatasetTransaction) error) (err error) {
	if fn == nil {
		return ErrNilTransactionCallback
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin dataset transaction: %w", err)
	}
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			err = errors.Join(err, fmt.Errorf("rollback dataset transaction: %w", rollbackErr))
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
	if _, err := t.tx.ExecContext(ctx, `DELETE FROM radarr_title`); err != nil {
		return fmt.Errorf("clear radarr titles: %w", err)
	}
	return t.UpsertRadarrTitles(ctx, batch)
}
func (t datasetTransaction) UpsertRadarrTitles(ctx context.Context, batch RadarrTitleBatch) error {
	for start := 0; start < len(batch.Rows); start += batchLimit {
		end := start + batchLimit
		if end > len(batch.Rows) {
			end = len(batch.Rows)
		}
		for _, row := range batch.Rows[start:end] {
			if err := upsertRadarr(ctx, transactionDB(t), row); err != nil {
				return err
			}
		}
	}
	return nil
}
func (t datasetTransaction) DeleteRadarrTitles(ctx context.Context, ids RadarrTitleIDs) error {
	for _, id := range ids.IDs {
		if err := javaInteger(int64(id)); err != nil {
			return err
		}
		if _, err := t.tx.ExecContext(ctx, `DELETE FROM radarr_title WHERE id=?`, id); err != nil {
			return wrap("delete radarr title", err)
		}
	}
	return nil
}

func (t datasetTransaction) UpsertSonarrTitles(ctx context.Context, b SonarrTitleBatch) error {
	for start := 0; start < len(b.Rows); start += batchLimit {
		end := min(start+batchLimit, len(b.Rows))
		for _, v := range b.Rows[start:end] {
			if err := validateSonarrTitle(v); err != nil {
				return err
			}
			if err := upsertSonarrTitle(ctx, t.tx, v); err != nil {
				return err
			}
		}
	}
	return nil
}
func (t datasetTransaction) DeleteSonarrTitles(ctx context.Context, ids SonarrTitleIDs) error {
	for _, id := range ids.IDs {
		if err := javaInteger(int64(id)); err != nil {
			return err
		}
		if _, err := t.tx.ExecContext(ctx, `DELETE FROM sonarr_title WHERE id=?`, id); err != nil {
			return wrap("delete sonarr title", err)
		}
	}
	return nil
}
func (t datasetTransaction) ReplaceSonarrTitles(ctx context.Context, b SonarrTitleBatch) error {
	if _, err := t.tx.ExecContext(ctx, `DELETE FROM sonarr_title`); err != nil {
		return wrap("clear sonarr titles", err)
	}
	return t.UpsertSonarrTitles(ctx, b)
}
func (t datasetTransaction) UpsertTMDBTitles(ctx context.Context, b TMDBTitleBatch) error {
	for start := 0; start < len(b.Rows); start += batchLimit {
		end := min(start+batchLimit, len(b.Rows))
		for _, v := range b.Rows[start:end] {
			if err := validStatus(v.ValidStatus); err != nil {
				return err
			}
			if err := upsertTMDBTitle(ctx, t.tx, v); err != nil {
				return err
			}
		}
	}
	return nil
}
func (t datasetTransaction) DeleteTMDBTitles(ctx context.Context, ids TMDBTitleIDs) error {
	for _, id := range ids.IDs {
		if err := javaInteger(int64(id)); err != nil {
			return err
		}
		if _, err := t.tx.ExecContext(ctx, `DELETE FROM tmdb_title WHERE id=?`, id); err != nil {
			return wrap("delete tmdb title", err)
		}
	}
	return nil
}
func (t datasetTransaction) ReplaceTMDBTitles(ctx context.Context, b TMDBTitleBatch) error {
	if _, err := t.tx.ExecContext(ctx, `DELETE FROM tmdb_title`); err != nil {
		return wrap("clear tmdb titles", err)
	}
	return t.UpsertTMDBTitles(ctx, b)
}

type transactionDB struct{ tx *sql.Tx }

func (d transactionDB) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return d.tx.ExecContext(ctx, query, args...)
}
