package sqlite

import "context"

func (t datasetTransaction) UpsertSonarrRules(ctx context.Context, batch SonarrRuleBatch) error {
	for start := 0; start < len(batch.Rows); start += batchLimit {
		end := min(start+batchLimit, len(batch.Rows))
		for _, row := range batch.Rows[start:end] {
			if err := validStatus(row.ValidStatus); err != nil {
				return err
			}
			if err := upsertSonarrRule(ctx, t.tx, row); err != nil {
				return err
			}
		}
	}
	return nil
}

func (t datasetTransaction) DeleteSonarrRules(ctx context.Context, ids RuleIDs) error {
	for _, id := range ids.IDs {
		if _, err := t.tx.ExecContext(ctx, `DELETE FROM sonarr_rule WHERE id=?`, id); err != nil {
			return wrap("delete sonarr rule", err)
		}
	}
	return nil
}

func (t datasetTransaction) SwitchSonarrRuleStatus(ctx context.Context, ids RuleIDs, status ValidStatus) error {
	if err := validStatus(status); err != nil {
		return err
	}
	for _, id := range ids.IDs {
		if _, err := t.tx.ExecContext(ctx, `UPDATE sonarr_rule SET valid_status=?,update_time=CURRENT_TIMESTAMP WHERE id=?`, status, id); err != nil {
			return wrap("switch sonarr rule", err)
		}
	}
	return nil
}

func (t datasetTransaction) ReplaceSonarrRules(ctx context.Context, batch SonarrRuleBatch) error {
	if _, err := t.tx.ExecContext(ctx, `DELETE FROM sonarr_rule`); err != nil {
		return wrap("clear sonarr rules", err)
	}
	return t.UpsertSonarrRules(ctx, batch)
}

func (t datasetTransaction) UpsertRadarrRules(ctx context.Context, batch RadarrRuleBatch) error {
	for start := 0; start < len(batch.Rows); start += batchLimit {
		end := min(start+batchLimit, len(batch.Rows))
		for _, row := range batch.Rows[start:end] {
			value := SonarrRule(row)
			if err := validStatus(value.ValidStatus); err != nil {
				return err
			}
			if err := upsertRadarrRule(ctx, t.tx, value); err != nil {
				return err
			}
		}
	}
	return nil
}

func (t datasetTransaction) DeleteRadarrRules(ctx context.Context, ids RuleIDs) error {
	for _, id := range ids.IDs {
		if _, err := t.tx.ExecContext(ctx, `DELETE FROM radarr_rule WHERE id=?`, id); err != nil {
			return wrap("delete radarr rule", err)
		}
	}
	return nil
}

func (t datasetTransaction) SwitchRadarrRuleStatus(ctx context.Context, ids RuleIDs, status ValidStatus) error {
	if err := validStatus(status); err != nil {
		return err
	}
	for _, id := range ids.IDs {
		if _, err := t.tx.ExecContext(ctx, `UPDATE radarr_rule SET valid_status=?,update_time=CURRENT_TIMESTAMP WHERE id=?`, status, id); err != nil {
			return wrap("switch radarr rule", err)
		}
	}
	return nil
}

func (t datasetTransaction) ReplaceRadarrRules(ctx context.Context, batch RadarrRuleBatch) error {
	if _, err := t.tx.ExecContext(ctx, `DELETE FROM radarr_rule`); err != nil {
		return wrap("clear radarr rules", err)
	}
	return t.UpsertRadarrRules(ctx, batch)
}
