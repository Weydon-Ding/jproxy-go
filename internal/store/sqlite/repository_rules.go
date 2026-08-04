package sqlite

import (
	"context"
	"database/sql"
	"fmt"
)

func (r sonarrRuleRepo) Upsert(c context.Context, v SonarrRule) error {
	return upsertRule(c, r.db, "sonarr_rule", v)
}
func (r radarrRuleRepo) Upsert(c context.Context, v RadarrRule) error {
	return upsertRule(c, r.db, "radarr_rule", SonarrRule(v))
}
func upsertRule(c context.Context, db *sql.DB, table string, v SonarrRule) error {
	_, e := db.ExecContext(c, fmt.Sprintf(`INSERT INTO %s(id,token,priority,regex,replacement,offset,example,remark,author,valid_status,create_time,update_time) VALUES(?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET token=excluded.token,priority=excluded.priority,regex=excluded.regex,replacement=excluded.replacement,offset=excluded.offset,example=excluded.example,remark=excluded.remark,author=excluded.author,valid_status=excluded.valid_status,create_time=excluded.create_time,update_time=excluded.update_time`, table), v.ID, v.Token, v.Priority, v.Regex, v.Replacement, v.Offset, v.Example, v.Remark, v.Author, v.ValidStatus, v.CreateTime, v.UpdateTime)
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
	e = db.QueryRowContext(c, fmt.Sprintf(`SELECT id,token,priority,regex,replacement,offset,example,remark,author,valid_status,create_time,update_time FROM %s WHERE id=?`, t), id).Scan(&v.ID, &v.Token, &v.Priority, &v.Regex, &v.Replacement, &v.Offset, &v.Example, &v.Remark, &v.Author, &v.ValidStatus, &v.CreateTime, &v.UpdateTime)
	return v, wrap("get rule", e)
}
func (r sonarrRuleRepo) Page(c context.Context, f RuleFilter) (out PageResult[SonarrRule], e error) {
	p := f.Page.normalized()
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
	e = r.db.QueryRowContext(c, `SELECT COUNT(*) FROM sonarr_rule`+where, args...).Scan(&out.Total)
	if e != nil {
		return out, wrap("count sonarr rules", e)
	}
	args = append(args, p.Size, (p.Current-1)*p.Size)
	rows, e := r.db.QueryContext(c, `SELECT id,token,priority,regex,replacement,offset,example,remark,author,valid_status,create_time,update_time FROM sonarr_rule`+where+` ORDER BY update_time DESC,id DESC LIMIT ? OFFSET ?`, args...)
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
	out.Current, out.Size = p.Current, p.Size
	return out, wrap("read sonarr rules", rows.Err())
}
