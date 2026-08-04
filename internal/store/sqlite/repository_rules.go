package sqlite

import (
	"context"
	"database/sql"
	"errors"
)

func (r sonarrRuleRepo) Upsert(c context.Context, v SonarrRule) error {
	if err := validStatus(v.ValidStatus); err != nil {
		return err
	}
	return upsertSonarrRule(c, r.db, v)
}
func (r radarrRuleRepo) Upsert(c context.Context, v RadarrRule) error {
	if err := validStatus(v.ValidStatus); err != nil {
		return err
	}
	return upsertRadarrRule(c, r.db, SonarrRule(v))
}
func upsertSonarrRule(c context.Context, db executor, v SonarrRule) error {
	_, e := db.ExecContext(c, `INSERT INTO sonarr_rule(id,token,priority,regex,replacement,offset,example,remark,author,valid_status,create_time,update_time) VALUES(?,?,?,?,?,?,?,?,?,?,COALESCE(?,CURRENT_TIMESTAMP),COALESCE(?,CURRENT_TIMESTAMP)) ON CONFLICT(id) DO UPDATE SET token=excluded.token,priority=excluded.priority,regex=excluded.regex,replacement=excluded.replacement,offset=excluded.offset,example=excluded.example,remark=excluded.remark,author=excluded.author,valid_status=excluded.valid_status,update_time=COALESCE(excluded.update_time,CURRENT_TIMESTAMP)`, v.ID, v.Token, v.Priority, v.Regex, v.Replacement, v.Offset, v.Example, v.Remark, v.Author, v.ValidStatus, v.CreateTime, v.UpdateTime)
	return wrap("upsert rule", e)
}
func upsertRadarrRule(c context.Context, db executor, v SonarrRule) error {
	_, e := db.ExecContext(c, `INSERT INTO radarr_rule(id,token,priority,regex,replacement,offset,example,remark,author,valid_status,create_time,update_time) VALUES(?,?,?,?,?,?,?,?,?,?,COALESCE(?,CURRENT_TIMESTAMP),COALESCE(?,CURRENT_TIMESTAMP)) ON CONFLICT(id) DO UPDATE SET token=excluded.token,priority=excluded.priority,regex=excluded.regex,replacement=excluded.replacement,offset=excluded.offset,example=excluded.example,remark=excluded.remark,author=excluded.author,valid_status=excluded.valid_status,update_time=COALESCE(excluded.update_time,CURRENT_TIMESTAMP)`, v.ID, v.Token, v.Priority, v.Regex, v.Replacement, v.Offset, v.Example, v.Remark, v.Author, v.ValidStatus, v.CreateTime, v.UpdateTime)
	return wrap("upsert rule", e)
}
func (r sonarrRuleRepo) Get(c context.Context, id RuleID) (v SonarrRule, e error) {
	return getRule(c, r.db, "sonarr_rule", id)
}
func (r radarrRuleRepo) Get(c context.Context, id RuleID) (v RadarrRule, e error) {
	x, e := getRule(c, r.db, "radarr_rule", id)
	return RadarrRule(x), e
}
func getRule(c context.Context, db *sql.DB, t string, id RuleID) (v SonarrRule, e error) {
	if t == "sonarr_rule" {
		e = db.QueryRowContext(c, `SELECT id,token,priority,regex,replacement,offset,example,remark,author,valid_status,create_time,update_time FROM sonarr_rule WHERE id=?`, id).Scan(&v.ID, &v.Token, &v.Priority, &v.Regex, &v.Replacement, &v.Offset, &v.Example, &v.Remark, &v.Author, &v.ValidStatus, &v.CreateTime, &v.UpdateTime)
	} else {
		e = db.QueryRowContext(c, `SELECT id,token,priority,regex,replacement,offset,example,remark,author,valid_status,create_time,update_time FROM radarr_rule WHERE id=?`, id).Scan(&v.ID, &v.Token, &v.Priority, &v.Regex, &v.Replacement, &v.Offset, &v.Example, &v.Remark, &v.Author, &v.ValidStatus, &v.CreateTime, &v.UpdateTime)
	}
	return v, wrap("get rule", e)
}
func (r sonarrRuleRepo) Page(c context.Context, f RuleFilter) (out PageResult[SonarrRule], e error) {
	f = f.normalized()
	p := f.Page
	where := ` WHERE 1=1`
	args := []any{}
	if f.Token != nil {
		where += ` AND token LIKE ?`
		args = append(args, "%"+*f.Token+"%")
	}
	if f.Remark != nil {
		where += ` AND remark LIKE ?`
		args = append(args, "%"+*f.Remark+"%")
	}
	tx, e := r.db.BeginTx(c, &sql.TxOptions{ReadOnly: true})
	if e != nil {
		return out, wrap("begin sonarr rule page", e)
	}
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			e = errors.Join(e, wrap("rollback sonarr rule page", rollbackErr))
		}
	}()
	e = tx.QueryRowContext(c, `SELECT COUNT(*) FROM sonarr_rule`+where, args...).Scan(&out.Total)
	if e != nil {
		return out, wrap("count sonarr rules", e)
	}
	args = append(args, p.Size, (p.Current-1)*p.Size)
	rows, e := tx.QueryContext(c, `SELECT id,token,priority,regex,replacement,offset,example,remark,author,valid_status,create_time,update_time FROM sonarr_rule`+where+` ORDER BY update_time DESC,id DESC LIMIT ? OFFSET ?`, args...)
	if e != nil {
		return out, wrap("page sonarr rules", e)
	}
	defer rows.Close()
	for rows.Next() {
		var v SonarrRule
		if e = rows.Scan(&v.ID, &v.Token, &v.Priority, &v.Regex, &v.Replacement, &v.Offset, &v.Example, &v.Remark, &v.Author, &v.ValidStatus, &v.CreateTime, &v.UpdateTime); e != nil {
			return out, wrap("scan sonarr rule", e)
		}
		out.List = append(out.List, v)
	}
	if e = rows.Err(); e != nil {
		return out, wrap("read sonarr rules", e)
	}
	if e = tx.Commit(); e != nil {
		return out, wrap("commit sonarr rule page", e)
	}
	out.Current, out.Size = p.Current, p.Size
	return out, nil
}
