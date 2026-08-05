package sqlite

import "context"

func (t datasetTransaction) UpsertSonarrExamples(ctx context.Context, batch SonarrExampleBatch) error {
	return t.upsertExamples(ctx, "sonarr_example", batch.Rows)
}
func (t datasetTransaction) DeleteSonarrExamples(ctx context.Context, ids ExampleIDs) error {
	return t.deleteExamples(ctx, "sonarr_example", ids)
}
func (t datasetTransaction) ReplaceSonarrExamples(ctx context.Context, batch SonarrExampleBatch) error {
	return t.replaceExamples(ctx, "sonarr_example", batch.Rows)
}
func (t datasetTransaction) UpsertRadarrExamples(ctx context.Context, batch RadarrExampleBatch) error {
	rows := make([]SonarrExample, len(batch.Rows))
	for index, row := range batch.Rows {
		rows[index] = SonarrExample(row)
	}
	return t.upsertExamples(ctx, "radarr_example", rows)
}
func (t datasetTransaction) DeleteRadarrExamples(ctx context.Context, ids ExampleIDs) error {
	return t.deleteExamples(ctx, "radarr_example", ids)
}
func (t datasetTransaction) ReplaceRadarrExamples(ctx context.Context, batch RadarrExampleBatch) error {
	rows := make([]SonarrExample, len(batch.Rows))
	for index, row := range batch.Rows {
		rows[index] = SonarrExample(row)
	}
	return t.replaceExamples(ctx, "radarr_example", rows)
}

func (t datasetTransaction) upsertExamples(ctx context.Context, table string, rows []SonarrExample) error {
	if len(rows) > batchLimit {
		return ErrBatchTooLarge
	}
	for _, row := range rows {
		if err := validateExample(row); err != nil {
			return err
		}
		if _, err := t.tx.ExecContext(ctx, "INSERT INTO "+table+"(hash,original_text,format_text,valid_status,create_time,update_time) VALUES(?,?,?,?,COALESCE(?,CURRENT_TIMESTAMP),COALESCE(?,CURRENT_TIMESTAMP)) ON CONFLICT(hash) DO UPDATE SET original_text=excluded.original_text,format_text=excluded.format_text,valid_status=excluded.valid_status,update_time=COALESCE(excluded.update_time,CURRENT_TIMESTAMP)", row.Hash, row.OriginalText, row.FormatText, row.ValidStatus, row.CreateTime, row.UpdateTime); err != nil {
			return wrap("upsert example", err)
		}
	}
	return nil
}
func (t datasetTransaction) deleteExamples(ctx context.Context, table string, ids ExampleIDs) error {
	if len(ids.IDs) > batchLimit {
		return ErrBatchTooLarge
	}
	for _, id := range ids.IDs {
		if _, err := t.tx.ExecContext(ctx, "DELETE FROM "+table+" WHERE hash=?", id); err != nil {
			return wrap("delete example", err)
		}
	}
	return nil
}
func (t datasetTransaction) replaceExamples(ctx context.Context, table string, rows []SonarrExample) error {
	if _, err := t.tx.ExecContext(ctx, "DELETE FROM "+table); err != nil {
		return wrap("clear examples", err)
	}
	return t.upsertExamples(ctx, table, rows)
}
