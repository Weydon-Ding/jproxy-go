package sqlite

import (
	"context"
	"database/sql"
	"fmt"
)

type Repositories struct {
	SystemConfigs SystemConfigRepository
	SystemUsers   SystemUserRepository
	SonarrRules   SonarrRuleRepository
	RadarrRules   RadarrRuleRepository
	SonarrTitles  SonarrTitleRepository
	RadarrTitles  RadarrTitleRepository
	TMDBTitles    TMDBTitleRepository
}

func (s *Store) Repositories() Repositories {
	return Repositories{systemConfigRepo{s.db, s}, systemUserRepo{s.db}, sonarrRuleRepo{s.db, s}, radarrRuleRepo{s.db, s}, sonarrTitleRepo{s.db, s}, radarrTitleRepo{db: s.db, store: s}, tmdbTitleRepo{s.db, s}}
}

type systemConfigRepo struct {
	db    *sql.DB
	store *Store
}
type systemUserRepo struct{ db *sql.DB }
type sonarrRuleRepo struct {
	db    *sql.DB
	store *Store
}
type radarrRuleRepo struct {
	db    *sql.DB
	store *Store
}
type sonarrTitleRepo struct {
	db    *sql.DB
	store *Store
}
type radarrTitleRepo struct {
	db    *sql.DB
	store *Store
}
type tmdbTitleRepo struct {
	db    *sql.DB
	store *Store
}

func (r systemConfigRepo) Upsert(c context.Context, v SystemConfig) error {
	if err := javaInteger(int64(v.ID)); err != nil {
		return err
	}
	if err := validStatus(v.ValidStatus); err != nil {
		return err
	}
	return r.store.InTransaction(c, func(tx DatasetTransaction) error {
		return upsertSystemConfig(c, tx.(datasetTransaction).tx, v)
	})
}
func (r systemConfigRepo) Get(c context.Context, id SystemConfigID) (v SystemConfig, e error) {
	e = r.db.QueryRowContext(c, `SELECT id,"key",value,valid_status,create_time,update_time FROM system_config WHERE id=?`, id).Scan(&v.ID, &v.Key, &v.Value, &v.ValidStatus, &v.CreateTime, &v.UpdateTime)
	return v, wrap("get system config", e)
}
func (r systemConfigRepo) UpsertBatch(c context.Context, values []SystemConfig) error {
	return r.store.InTransaction(c, func(tx DatasetTransaction) error {
		for _, value := range values {
			if err := javaInteger(int64(value.ID)); err != nil {
				return err
			}
			if err := validStatus(value.ValidStatus); err != nil {
				return err
			}
			if err := upsertSystemConfig(c, tx.(datasetTransaction).tx, value); err != nil {
				return err
			}
		}
		return nil
	})
}
func (r systemUserRepo) Upsert(c context.Context, v SystemUser) error {
	if err := javaInteger(int64(v.ID)); err != nil {
		return err
	}
	if err := validStatus(v.ValidStatus); err != nil {
		return err
	}
	_, e := r.db.ExecContext(c, `INSERT INTO system_user(id,username,password,role,valid_status,create_time,update_time) VALUES(?,?,?,?,?,COALESCE(?,CURRENT_TIMESTAMP),COALESCE(?,CURRENT_TIMESTAMP)) ON CONFLICT(id) DO UPDATE SET username=excluded.username,password=excluded.password,role=excluded.role,valid_status=excluded.valid_status,update_time=COALESCE(excluded.update_time,CURRENT_TIMESTAMP)`, v.ID, v.Username, v.Password, v.Role, v.ValidStatus, v.CreateTime, v.UpdateTime)
	return wrap("upsert system user", e)
}

func upsertSystemConfig(c context.Context, db executor, v SystemConfig) error {
	_, err := db.ExecContext(c, `INSERT INTO system_config(id,"key",value,valid_status,create_time,update_time) VALUES(?,?,?,?,COALESCE(?,CURRENT_TIMESTAMP),COALESCE(?,CURRENT_TIMESTAMP)) ON CONFLICT(id) DO UPDATE SET "key"=excluded."key",value=excluded.value,valid_status=excluded.valid_status,update_time=COALESCE(excluded.update_time,CURRENT_TIMESTAMP)`, v.ID, v.Key, v.Value, v.ValidStatus, v.CreateTime, v.UpdateTime)
	return wrap("upsert system config", err)
}
func (r systemUserRepo) Get(c context.Context, id SystemUserID) (v SystemUser, e error) {
	e = r.db.QueryRowContext(c, `SELECT id,username,password,role,valid_status,create_time,update_time FROM system_user WHERE id=?`, id).Scan(&v.ID, &v.Username, &v.Password, &v.Role, &v.ValidStatus, &v.CreateTime, &v.UpdateTime)
	return v, wrap("get system user", e)
}
func (r systemUserRepo) FindByUsername(c context.Context, username string) (v SystemUser, e error) {
	e = r.db.QueryRowContext(c, `SELECT id,username,password,role,valid_status,create_time,update_time FROM system_user WHERE username=? ORDER BY id DESC LIMIT 1`, username).Scan(&v.ID, &v.Username, &v.Password, &v.Role, &v.ValidStatus, &v.CreateTime, &v.UpdateTime)
	return v, wrap("find system user", e)
}
func wrap(op string, e error) error {
	if e != nil {
		return fmt.Errorf("%s: %w", op, e)
	}
	return nil
}
