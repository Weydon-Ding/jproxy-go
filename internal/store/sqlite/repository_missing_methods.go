package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

func (r systemConfigRepo) List(ctx context.Context) ([]SystemConfig, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,"key",value,valid_status,create_time,update_time FROM system_config ORDER BY update_time DESC,id DESC`)
	if err != nil {
		return nil, wrap("list system config", err)
	}
	defer rows.Close()
	var values []SystemConfig
	for rows.Next() {
		var value SystemConfig
		if err := rows.Scan(&value.ID, &value.Key, &value.Value, &value.ValidStatus, &value.CreateTime, &value.UpdateTime); err != nil {
			return nil, wrap("scan system config", err)
		}
		values = append(values, value)
	}
	return values, wrap("read system config", rows.Err())
}
func (r systemConfigRepo) ValueByKey(ctx context.Context, key string) (string, error) {
	var value string
	err := r.db.QueryRowContext(ctx, `SELECT value FROM system_config WHERE "key"=? AND valid_status=1 ORDER BY id DESC LIMIT 1`, key).Scan(&value)
	return value, wrap("get system config value", err)
}

func (r sonarrRuleRepo) UpsertBatch(ctx context.Context, batch SonarrRuleBatch) error {
	return r.store.InTransaction(ctx, func(tx DatasetTransaction) error { return tx.UpsertSonarrRules(ctx, batch) })
}
func (r sonarrRuleRepo) DeleteBatch(ctx context.Context, ids RuleIDs) error {
	return r.store.InTransaction(ctx, func(tx DatasetTransaction) error { return tx.DeleteSonarrRules(ctx, ids) })
}
func (r sonarrRuleRepo) SwitchValidStatus(ctx context.Context, ids RuleIDs, status ValidStatus) error {
	if err := validStatus(status); err != nil {
		return err
	}
	return r.store.InTransaction(ctx, func(tx DatasetTransaction) error { return tx.SwitchSonarrRuleStatus(ctx, ids, status) })
}
func (r sonarrRuleRepo) Replace(ctx context.Context, batch SonarrRuleBatch) error {
	return r.store.InTransaction(ctx, func(tx DatasetTransaction) error { return tx.ReplaceSonarrRules(ctx, batch) })
}
func (r radarrRuleRepo) Page(ctx context.Context, filter RuleFilter) (PageResult[RadarrRule], error) {
	return pageRadarrRules(ctx, r.db, filter)
}
func (r radarrRuleRepo) UpsertBatch(ctx context.Context, batch RadarrRuleBatch) error {
	return r.store.InTransaction(ctx, func(tx DatasetTransaction) error { return tx.UpsertRadarrRules(ctx, batch) })
}
func (r radarrRuleRepo) DeleteBatch(ctx context.Context, ids RuleIDs) error {
	return r.store.InTransaction(ctx, func(tx DatasetTransaction) error { return tx.DeleteRadarrRules(ctx, ids) })
}
func (r radarrRuleRepo) SwitchValidStatus(ctx context.Context, ids RuleIDs, status ValidStatus) error {
	if err := validStatus(status); err != nil {
		return err
	}
	return r.store.InTransaction(ctx, func(tx DatasetTransaction) error { return tx.SwitchRadarrRuleStatus(ctx, ids, status) })
}
func (r radarrRuleRepo) Replace(ctx context.Context, batch RadarrRuleBatch) error {
	return r.store.InTransaction(ctx, func(tx DatasetTransaction) error { return tx.ReplaceRadarrRules(ctx, batch) })
}

func pageRadarrRules(ctx context.Context, db *sql.DB, filter RuleFilter) (out PageResult[RadarrRule], err error) {
	filter = filter.normalized()
	where := ` WHERE 1=1`
	args := []any{}
	if filter.Token != nil {
		where += ` AND token LIKE ?`
		args = append(args, "%"+*filter.Token+"%")
	}
	if filter.Remark != nil {
		where += ` AND remark LIKE ?`
		args = append(args, "%"+*filter.Remark+"%")
	}
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return out, wrap("begin radarr rule page", err)
	}
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			err = errors.Join(err, wrap("rollback radarr rule page", rollbackErr))
		}
	}()
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM radarr_rule`+where, args...).Scan(&out.Total); err != nil {
		return out, wrap("count radarr rules", err)
	}
	page := filter.Page
	args = append(args, page.Size, (page.Current-1)*page.Size)
	rows, err := tx.QueryContext(ctx, `SELECT id,token,priority,regex,replacement,offset,example,remark,author,valid_status,create_time,update_time FROM radarr_rule`+where+` ORDER BY update_time DESC,id DESC LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return out, wrap("list radarr rules", err)
	}
	defer rows.Close()
	for rows.Next() {
		var row SonarrRule
		if err = rows.Scan(&row.ID, &row.Token, &row.Priority, &row.Regex, &row.Replacement, &row.Offset, &row.Example, &row.Remark, &row.Author, &row.ValidStatus, &row.CreateTime, &row.UpdateTime); err != nil {
			return out, wrap("scan radarr rule", err)
		}
		out.List = append(out.List, RadarrRule(row))
	}
	if err = rows.Err(); err != nil {
		return out, wrap("read radarr rules", err)
	}
	if err = tx.Commit(); err != nil {
		return out, wrap("commit radarr rule page", err)
	}
	out.Current, out.Size = page.Current, page.Size
	return out, nil
}

var _ = fmt.Sprintf
