package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

func (r sonarrExampleRepo) Page(ctx context.Context, filter ExampleFilter) (PageResult[SonarrExample], error) {
	return pageExamples(ctx, r.db, "sonarr_example", filter, scanSonarrExample)
}

func (r radarrExampleRepo) Page(ctx context.Context, filter ExampleFilter) (PageResult[RadarrExample], error) {
	page, err := pageExamples(ctx, r.db, "radarr_example", filter, scanSonarrExample)
	rows := make([]RadarrExample, len(page.List))
	for index, row := range page.List {
		rows[index] = RadarrExample(row)
	}
	return PageResult[RadarrExample]{Current: page.Current, Size: page.Size, Total: page.Total, List: rows}, err
}

func (r sonarrExampleRepo) List(ctx context.Context, filter ExampleFilter) ([]SonarrExample, error) {
	return listExamples(ctx, r.db, "sonarr_example", filter, scanSonarrExample)
}

func (r radarrExampleRepo) List(ctx context.Context, filter ExampleFilter) ([]RadarrExample, error) {
	rows, err := listExamples(ctx, r.db, "radarr_example", filter, scanSonarrExample)
	result := make([]RadarrExample, len(rows))
	for index, row := range rows {
		result[index] = RadarrExample(row)
	}
	return result, err
}

func pageExamples(ctx context.Context, db *sql.DB, table string, filter ExampleFilter, scan func(*sql.Rows) (SonarrExample, error)) (out PageResult[SonarrExample], err error) {
	filter = filter.normalized()
	where, args := exampleWhere(filter)
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return out, wrap("begin example page", err)
	}
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			err = errors.Join(err, wrap("rollback example page", rollbackErr))
		}
	}()
	if err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table+where, args...).Scan(&out.Total); err != nil {
		return out, wrap("count examples", err)
	}
	args = append(args, filter.Page.Size, (filter.Page.Current-1)*filter.Page.Size)
	rows, err := tx.QueryContext(ctx, "SELECT hash,original_text,format_text,valid_status,create_time,update_time FROM "+table+where+" ORDER BY update_time DESC,hash DESC LIMIT ? OFFSET ?", args...)
	if err != nil {
		return out, wrap("page examples", err)
	}
	defer rows.Close()
	for rows.Next() {
		row, scanErr := scan(rows)
		if scanErr != nil {
			return out, scanErr
		}
		out.List = append(out.List, row)
	}
	if err = rows.Err(); err != nil {
		return out, wrap("read examples", err)
	}
	if err = tx.Commit(); err != nil {
		return out, wrap("commit example page", err)
	}
	out.Current, out.Size = filter.Page.Current, filter.Page.Size
	return out, nil
}

func listExamples(ctx context.Context, db *sql.DB, table string, filter ExampleFilter, scan func(*sql.Rows) (SonarrExample, error)) ([]SonarrExample, error) {
	filter = filter.normalized()
	where, args := exampleWhere(filter)
	rows, err := db.QueryContext(ctx, "SELECT hash,original_text,format_text,valid_status,create_time,update_time FROM "+table+where+" ORDER BY update_time DESC,hash DESC", args...)
	if err != nil {
		return nil, wrap("list examples", err)
	}
	defer rows.Close()
	var result []SonarrExample
	for rows.Next() {
		row, scanErr := scan(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, row)
	}
	return result, wrap("read examples", rows.Err())
}

func exampleWhere(filter ExampleFilter) (string, []any) {
	if filter.OriginalText == nil {
		return "", nil
	}
	return " WHERE original_text LIKE ?", []any{"%" + *filter.OriginalText + "%"}
}

func scanSonarrExample(rows *sql.Rows) (SonarrExample, error) {
	var row SonarrExample
	if err := rows.Scan(&row.Hash, &row.OriginalText, &row.FormatText, &row.ValidStatus, &row.CreateTime, &row.UpdateTime); err != nil {
		return SonarrExample{}, wrap("scan example", err)
	}
	return row, nil
}

func (r sonarrExampleRepo) UpsertBatch(ctx context.Context, batch SonarrExampleBatch) error {
	return r.store.InTransaction(ctx, func(tx DatasetTransaction) error { return tx.UpsertSonarrExamples(ctx, batch) })
}
func (r sonarrExampleRepo) DeleteBatch(ctx context.Context, ids ExampleIDs) error {
	return r.store.InTransaction(ctx, func(tx DatasetTransaction) error { return tx.DeleteSonarrExamples(ctx, ids) })
}
func (r sonarrExampleRepo) Replace(ctx context.Context, batch SonarrExampleBatch) error {
	return r.store.InTransaction(ctx, func(tx DatasetTransaction) error { return tx.ReplaceSonarrExamples(ctx, batch) })
}
func (r radarrExampleRepo) UpsertBatch(ctx context.Context, batch RadarrExampleBatch) error {
	return r.store.InTransaction(ctx, func(tx DatasetTransaction) error { return tx.UpsertRadarrExamples(ctx, batch) })
}
func (r radarrExampleRepo) DeleteBatch(ctx context.Context, ids ExampleIDs) error {
	return r.store.InTransaction(ctx, func(tx DatasetTransaction) error { return tx.DeleteRadarrExamples(ctx, ids) })
}
func (r radarrExampleRepo) Replace(ctx context.Context, batch RadarrExampleBatch) error {
	return r.store.InTransaction(ctx, func(tx DatasetTransaction) error { return tx.ReplaceRadarrExamples(ctx, batch) })
}

func validateExample(row SonarrExample) error {
	if row.Hash == "" || row.OriginalText == "" {
		return fmt.Errorf("example hash and original text are required")
	}
	return validStatus(row.ValidStatus)
}
