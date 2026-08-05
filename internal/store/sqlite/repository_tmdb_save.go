package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

func (r tmdbTitleRepo) Save(ctx context.Context, input TMDBTitleSaveInput) (result TMDBTitleSaveResult, err error) {
	err = r.store.InTransaction(ctx, func(tx DatasetTransaction) error {
		result, err = tx.SaveTMDBTitle(ctx, input)
		return err
	})
	if err != nil {
		return TMDBTitleSaveResult{}, err
	}
	return result, nil
}

func validateTMDBSave(input TMDBTitleSaveInput) error {
	if input.SuppliedID {
		if err := javaInteger(int64(input.Title.ID)); err != nil {
			return err
		}
	}
	if err := javaInteger(input.Title.TVDBID); err != nil {
		return err
	}
	if err := javaIntegerPointer(input.Title.TMDBID); err != nil {
		return err
	}
	return validStatus(input.Title.ValidStatus)
}

func reuseTMDBID(ctx context.Context, tx *sql.Tx, value *TMDBTitle) error {
	var reusable sql.NullInt64
	err := tx.QueryRowContext(ctx, `SELECT tmdb_id FROM tmdb_title WHERE tvdb_id=? ORDER BY id ASC LIMIT 1`, value.TVDBID).Scan(&reusable)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("find reusable tmdb id: %w", err)
	}
	if reusable.Valid {
		value.TMDBID = &reusable.Int64
	}
	return nil
}

func insertTMDBTitle(ctx context.Context, tx *sql.Tx, value TMDBTitle) (TMDBTitleID, error) {
	result, err := tx.ExecContext(ctx, `INSERT INTO tmdb_title(tvdb_id,tmdb_id,language,title,valid_status,create_time,update_time) VALUES(?,?,?,?,?,COALESCE(?,CURRENT_TIMESTAMP),COALESCE(?,CURRENT_TIMESTAMP))`, value.TVDBID, value.TMDBID, value.Language, value.Title, value.ValidStatus, value.CreateTime, value.UpdateTime)
	if err != nil {
		return 0, fmt.Errorf("insert generated tmdb title: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("read generated tmdb title id: %w", err)
	}
	if err := javaInteger(id); err != nil {
		return 0, err
	}
	return TMDBTitleID(id), nil
}
