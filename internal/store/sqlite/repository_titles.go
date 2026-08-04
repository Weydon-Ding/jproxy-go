package sqlite

import (
	"context"
	"database/sql"
	"fmt"
)

type executor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func (r sonarrTitleRepo) Upsert(c context.Context, v SonarrTitle) error {
	_, e := r.db.ExecContext(c, `INSERT INTO sonarr_title(id,tvdb_id,sno,main_title,title,clean_title,season_number,monitored,valid_status,create_time,update_time,series_id) VALUES(?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET tvdb_id=excluded.tvdb_id,sno=excluded.sno,main_title=excluded.main_title,title=excluded.title,clean_title=excluded.clean_title,season_number=excluded.season_number,monitored=excluded.monitored,valid_status=excluded.valid_status,create_time=excluded.create_time,update_time=excluded.update_time,series_id=excluded.series_id`, v.ID, v.TVDBID, v.SNO, v.MainTitle, v.Title, v.CleanTitle, v.SeasonNumber, v.Monitored, v.ValidStatus, v.CreateTime, v.UpdateTime, v.SeriesID)
	return wrap("upsert sonarr title", e)
}
func (r sonarrTitleRepo) Get(c context.Context, id SonarrTitleID) (v SonarrTitle, e error) {
	e = r.db.QueryRowContext(c, `SELECT id,tvdb_id,sno,main_title,title,clean_title,season_number,monitored,valid_status,create_time,update_time,series_id FROM sonarr_title WHERE id=?`, id).Scan(&v.ID, &v.TVDBID, &v.SNO, &v.MainTitle, &v.Title, &v.CleanTitle, &v.SeasonNumber, &v.Monitored, &v.ValidStatus, &v.CreateTime, &v.UpdateTime, &v.SeriesID)
	return v, wrap("get sonarr title", e)
}
func (r radarrTitleRepo) Upsert(c context.Context, v RadarrTitle) error {
	return upsertRadarr(c, r.db, v)
}
func upsertRadarr(c context.Context, db executor, v RadarrTitle) error {
	_, e := db.ExecContext(c, `INSERT INTO radarr_title(id,tmdb_id,sno,main_title,title,clean_title,year,monitored,valid_status,create_time,update_time,movie_id) VALUES(?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET tmdb_id=excluded.tmdb_id,sno=excluded.sno,main_title=excluded.main_title,title=excluded.title,clean_title=excluded.clean_title,year=excluded.year,monitored=excluded.monitored,valid_status=excluded.valid_status,create_time=excluded.create_time,update_time=excluded.update_time,movie_id=excluded.movie_id`, v.ID, v.TMDBID, v.SNO, v.MainTitle, v.Title, v.CleanTitle, v.Year, v.Monitored, v.ValidStatus, v.CreateTime, v.UpdateTime, v.MovieID)
	return wrap("upsert radarr title", e)
}
func (r radarrTitleRepo) Get(c context.Context, id RadarrTitleID) (v RadarrTitle, e error) {
	e = r.db.QueryRowContext(c, `SELECT id,tmdb_id,sno,main_title,title,clean_title,year,monitored,valid_status,create_time,update_time,movie_id FROM radarr_title WHERE id=?`, id).Scan(&v.ID, &v.TMDBID, &v.SNO, &v.MainTitle, &v.Title, &v.CleanTitle, &v.Year, &v.Monitored, &v.ValidStatus, &v.CreateTime, &v.UpdateTime, &v.MovieID)
	return v, wrap("get radarr title", e)
}
func (r tmdbTitleRepo) Upsert(c context.Context, v TMDBTitle) error {
	_, e := r.db.ExecContext(c, `INSERT INTO tmdb_title(id,tvdb_id,tmdb_id,language,title,valid_status,create_time,update_time) VALUES(?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET tvdb_id=excluded.tvdb_id,tmdb_id=excluded.tmdb_id,language=excluded.language,title=excluded.title,valid_status=excluded.valid_status,create_time=excluded.create_time,update_time=excluded.update_time`, v.ID, v.TVDBID, v.TMDBID, v.Language, v.Title, v.ValidStatus, v.CreateTime, v.UpdateTime)
	return wrap("upsert tmdb title", e)
}
func (r tmdbTitleRepo) Get(c context.Context, id TMDBTitleID) (v TMDBTitle, e error) {
	e = r.db.QueryRowContext(c, `SELECT id,tvdb_id,tmdb_id,language,title,valid_status,create_time,update_time FROM tmdb_title WHERE id=?`, id).Scan(&v.ID, &v.TVDBID, &v.TMDBID, &v.Language, &v.Title, &v.ValidStatus, &v.CreateTime, &v.UpdateTime)
	return v, wrap("get tmdb title", e)
}
func (r radarrTitleRepo) Page(c context.Context, f RadarrTitleFilter) (out PageResult[RadarrTitle], e error) {
	p := f.Page.normalized()
	where := ` WHERE 1=1`
	args := []any{}
	if f.Title != nil {
		where += ` AND title LIKE ?`
		args = append(args, "%"+*f.Title+"%")
	}
	if f.TMDBID != nil {
		where += ` AND tmdb_id=?`
		args = append(args, *f.TMDBID)
	}
	e = r.db.QueryRowContext(c, `SELECT COUNT(*) FROM radarr_title`+where, args...).Scan(&out.Total)
	if e != nil {
		return out, e
	}
	args = append(args, p.Size, (p.Current-1)*p.Size)
	rows, e := r.db.QueryContext(c, `SELECT id,tmdb_id,sno,main_title,title,clean_title,year,monitored,valid_status,create_time,update_time,movie_id FROM radarr_title`+where+` ORDER BY update_time DESC,id DESC LIMIT ? OFFSET ?`, args...)
	if e != nil {
		return out, e
	}
	defer rows.Close()
	for rows.Next() {
		var v RadarrTitle
		if e = rows.Scan(&v.ID, &v.TMDBID, &v.SNO, &v.MainTitle, &v.Title, &v.CleanTitle, &v.Year, &v.Monitored, &v.ValidStatus, &v.CreateTime, &v.UpdateTime, &v.MovieID); e != nil {
			return out, e
		}
		out.List = append(out.List, v)
	}
	out.Current, out.Size = p.Current, p.Size
	return out, rows.Err()
}
func (r radarrTitleRepo) UpsertBatch(c context.Context, b RadarrTitleBatch) error {
	if len(b.Rows) > batchLimit {
		return ErrBatchTooLarge
	}
	for _, v := range b.Rows {
		if e := r.Upsert(c, v); e != nil {
			return e
		}
	}
	return nil
}
func (r radarrTitleRepo) DeleteBatch(c context.Context, b RadarrTitleIDs) error {
	if len(b.IDs) > batchLimit {
		return ErrBatchTooLarge
	}
	for _, id := range b.IDs {
		if _, e := r.db.ExecContext(c, `DELETE FROM radarr_title WHERE id=?`, id); e != nil {
			return fmt.Errorf("delete radarr title: %w", e)
		}
	}
	return nil
}
func (r radarrTitleRepo) Replace(c context.Context, b RadarrTitleBatch) error {
	return r.store.InTransaction(c, func(tx DatasetTransaction) error { return tx.ReplaceRadarrTitles(c, b) })
}
