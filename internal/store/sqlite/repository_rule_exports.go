package sqlite

import (
	"context"
	"database/sql"
)

func (r sonarrRuleRepo) Export(ctx context.Context, ids RuleIDs) ([]SonarrRule, error) {
	return exportRules(ctx, r.db, "sonarr_rule", ids)
}
func (r radarrRuleRepo) Export(ctx context.Context, ids RuleIDs) ([]RadarrRule, error) {
	rows, err := exportRules(ctx, r.db, "radarr_rule", ids)
	result := make([]RadarrRule, len(rows))
	for index, row := range rows {
		result[index] = RadarrRule(row)
	}
	return result, err
}
func (r sonarrRuleRepo) Tokens(ctx context.Context) ([]string, error) {
	return ruleTokens(ctx, r.db, "sonarr_rule")
}
func (r radarrRuleRepo) Tokens(ctx context.Context) ([]string, error) {
	return ruleTokens(ctx, r.db, "radarr_rule")
}
func (r sonarrRuleRepo) Import(ctx context.Context, batch SonarrRuleBatch) error {
	return r.store.InTransaction(ctx, func(tx DatasetTransaction) error { return tx.ImportSonarrRules(ctx, batch) })
}
func (r radarrRuleRepo) Import(ctx context.Context, batch RadarrRuleBatch) error {
	return r.store.InTransaction(ctx, func(tx DatasetTransaction) error { return tx.ImportRadarrRules(ctx, batch) })
}

func exportRules(ctx context.Context, db *sql.DB, table string, ids RuleIDs) ([]SonarrRule, error) {
	if len(ids.IDs) > batchLimit {
		return nil, ErrBatchTooLarge
	}
	query, args := "SELECT id,token,priority,regex,replacement,offset,example,remark,author,valid_status,create_time,update_time FROM "+table, []any{}
	if len(ids.IDs) > 0 {
		query += " WHERE id IN ("
		for index, id := range ids.IDs {
			if index > 0 {
				query += ","
			}
			query += "?"
			args = append(args, id)
		}
		query += ")"
	}
	rows, err := db.QueryContext(ctx, query+" ORDER BY update_time DESC,id DESC", args...)
	if err != nil {
		return nil, wrap("export rules", err)
	}
	defer rows.Close()
	var result []SonarrRule
	for rows.Next() {
		var row SonarrRule
		if err := rows.Scan(&row.ID, &row.Token, &row.Priority, &row.Regex, &row.Replacement, &row.Offset, &row.Example, &row.Remark, &row.Author, &row.ValidStatus, &row.CreateTime, &row.UpdateTime); err != nil {
			return nil, wrap("scan exported rule", err)
		}
		result = append(result, row)
	}
	return result, wrap("read exported rules", rows.Err())
}
func ruleTokens(ctx context.Context, db *sql.DB, table string) ([]string, error) {
	rows, err := db.QueryContext(ctx, "SELECT DISTINCT token FROM "+table+" ORDER BY token ASC")
	if err != nil {
		return nil, wrap("list rule tokens", err)
	}
	defer rows.Close()
	var result []string
	for rows.Next() {
		var token string
		if err := rows.Scan(&token); err != nil {
			return nil, wrap("scan rule token", err)
		}
		result = append(result, token)
	}
	return result, wrap("read rule tokens", rows.Err())
}
