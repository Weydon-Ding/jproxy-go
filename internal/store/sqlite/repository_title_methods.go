package sqlite

import (
	"context"
	"database/sql"
	"errors"
)

func (r sonarrTitleRepo) Page(ctx context.Context, f SonarrTitleFilter) (out PageResult[SonarrTitle], err error) {
	f = f.normalized()
	where := ` WHERE 1=1`
	args := []any{}
	if f.Title != nil {
		where += ` AND title LIKE ?`
		args = append(args, "%"+*f.Title+"%")
	}
	if f.TVDBID != nil {
		where += ` AND tvdb_id=?`
		args = append(args, *f.TVDBID)
	}
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return out, wrap("begin sonarr title page", err)
	}
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			err = errors.Join(err, wrap("rollback sonarr title page", rollbackErr))
		}
	}()
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM sonarr_title`+where, args...).Scan(&out.Total); err != nil {
		return out, wrap("count sonarr titles", err)
	}
	args = append(args, f.Page.Size, (f.Page.Current-1)*f.Page.Size)
	rows, err := tx.QueryContext(ctx, `SELECT id,tvdb_id,sno,main_title,title,clean_title,season_number,monitored,valid_status,create_time,update_time,series_id FROM sonarr_title`+where+` ORDER BY update_time DESC,id DESC LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return out, wrap("list sonarr titles", err)
	}
	defer rows.Close()
	for rows.Next() {
		var v SonarrTitle
		if err = rows.Scan(&v.ID, &v.TVDBID, &v.SNO, &v.MainTitle, &v.Title, &v.CleanTitle, &v.SeasonNumber, &v.Monitored, &v.ValidStatus, &v.CreateTime, &v.UpdateTime, &v.SeriesID); err != nil {
			return out, wrap("scan sonarr title", err)
		}
		out.List = append(out.List, v)
	}
	if err = rows.Err(); err != nil {
		return out, wrap("read sonarr titles", err)
	}
	if err = tx.Commit(); err != nil {
		return out, wrap("commit sonarr title page", err)
	}
	out.Current, out.Size = f.Page.Current, f.Page.Size
	return out, nil
}
func (r sonarrTitleRepo) UpsertBatch(ctx context.Context, b SonarrTitleBatch) error {
	return r.store.InTransaction(ctx, func(tx DatasetTransaction) error { return tx.UpsertSonarrTitles(ctx, b) })
}
func (r sonarrTitleRepo) DeleteBatch(ctx context.Context, ids SonarrTitleIDs) error {
	return r.store.InTransaction(ctx, func(tx DatasetTransaction) error { return tx.DeleteSonarrTitles(ctx, ids) })
}
func (r sonarrTitleRepo) Replace(ctx context.Context, b SonarrTitleBatch) error {
	return r.store.InTransaction(ctx, func(tx DatasetTransaction) error { return tx.ReplaceSonarrTitles(ctx, b) })
}
func (r sonarrTitleRepo) NeedTMDBSync(ctx context.Context) ([]int64, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT DISTINCT st.tvdb_id FROM sonarr_title st LEFT JOIN tmdb_title tt ON st.tvdb_id=tt.tvdb_id WHERE st.sno=0 AND tt.tvdb_id IS NULL ORDER BY st.tvdb_id`)
	if err != nil {
		return nil, wrap("find tmdb sync ids", err)
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			return nil, wrap("scan tmdb sync id", err)
		}
		ids = append(ids, id)
	}
	return ids, wrap("read tmdb sync ids", rows.Err())
}
func (r sonarrTitleRepo) WithTMDBTitles(ctx context.Context) ([]SonarrTitle, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT main_title,title,clean_title,season_number FROM (SELECT st.main_title,st.title,st.clean_title,st.season_number,st.monitored FROM sonarr_title st GROUP BY st.clean_title UNION SELECT st.main_title,tt.title,NULL,-1,st.monitored FROM sonarr_title st LEFT JOIN tmdb_title tt ON st.tvdb_id=tt.tvdb_id WHERE st.sno=0 GROUP BY tt.title) WHERE title IS NOT NULL ORDER BY monitored DESC,LENGTH(title) DESC`)
	if err != nil {
		return nil, wrap("join sonarr tmdb titles", err)
	}
	defer rows.Close()
	var out []SonarrTitle
	for rows.Next() {
		var v SonarrTitle
		if err = rows.Scan(&v.MainTitle, &v.Title, &v.CleanTitle, &v.SeasonNumber); err != nil {
			return nil, wrap("scan joined sonarr title", err)
		}
		out = append(out, v)
	}
	return out, wrap("read joined sonarr titles", rows.Err())
}
func (r tmdbTitleRepo) Page(ctx context.Context, f TMDBTitleFilter) (out PageResult[TMDBTitle], err error) {
	f = f.normalized()
	where := ` WHERE 1=1`
	args := []any{}
	if f.Title != nil {
		where += ` AND title LIKE ?`
		args = append(args, "%"+*f.Title+"%")
	}
	if f.TVDBID != nil {
		where += ` AND tvdb_id=?`
		args = append(args, *f.TVDBID)
	}
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return out, err
	}
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			err = errors.Join(err, wrap("rollback tmdb title page", rollbackErr))
		}
	}()
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM tmdb_title`+where, args...).Scan(&out.Total); err != nil {
		return out, err
	}
	args = append(args, f.Page.Size, (f.Page.Current-1)*f.Page.Size)
	rows, err := tx.QueryContext(ctx, `SELECT id,tvdb_id,tmdb_id,language,title,valid_status,create_time,update_time FROM tmdb_title`+where+` ORDER BY update_time DESC,id DESC LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var v TMDBTitle
		if err = rows.Scan(&v.ID, &v.TVDBID, &v.TMDBID, &v.Language, &v.Title, &v.ValidStatus, &v.CreateTime, &v.UpdateTime); err != nil {
			return out, err
		}
		out.List = append(out.List, v)
	}
	if err = rows.Err(); err != nil {
		return out, err
	}
	if err = tx.Commit(); err != nil {
		return out, err
	}
	out.Current, out.Size = f.Page.Current, f.Page.Size
	return out, nil
}
func (r tmdbTitleRepo) UpsertBatch(ctx context.Context, b TMDBTitleBatch) error {
	return r.store.InTransaction(ctx, func(tx DatasetTransaction) error { return tx.UpsertTMDBTitles(ctx, b) })
}
func (r tmdbTitleRepo) DeleteBatch(ctx context.Context, ids TMDBTitleIDs) error {
	return r.store.InTransaction(ctx, func(tx DatasetTransaction) error { return tx.DeleteTMDBTitles(ctx, ids) })
}
func (r tmdbTitleRepo) Replace(ctx context.Context, b TMDBTitleBatch) error {
	return r.store.InTransaction(ctx, func(tx DatasetTransaction) error { return tx.ReplaceTMDBTitles(ctx, b) })
}
func (r tmdbTitleRepo) FindByTVDBID(ctx context.Context, id int64) ([]TMDBTitle, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,tvdb_id,tmdb_id,language,title,valid_status,create_time,update_time FROM tmdb_title WHERE tvdb_id=? ORDER BY id`, id)
	if err != nil {
		return nil, wrap("find tmdb titles", err)
	}
	defer rows.Close()
	var titles []TMDBTitle
	for rows.Next() {
		var title TMDBTitle
		if err = rows.Scan(&title.ID, &title.TVDBID, &title.TMDBID, &title.Language, &title.Title, &title.ValidStatus, &title.CreateTime, &title.UpdateTime); err != nil {
			return nil, wrap("scan tmdb title", err)
		}
		titles = append(titles, title)
	}
	return titles, wrap("read tmdb titles", rows.Err())
}
