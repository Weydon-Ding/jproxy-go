package main

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

type databaseFixture struct{ database string }

func (fixture databaseFixture) runtimeEnvironment(extra ...string) []string {
	return runtimeEnvironment(append([]string{
		"JPROXY_DB_ENABLED=true",
		"JPROXY_DB_PATH=" + fixture.database,
		"JPROXY_RADARR_FORMAT_ENABLED=true",
		"JPROXY_SONARR_FORMAT_ENABLED=true",
	}, extra...)...)
}

func newJavaFormatterFixture(t *testing.T) databaseFixture {
	t.Helper()
	fixture := newJavaEmptyFixture(t)
	db := openFixtureDatabase(t, fixture.database)
	for _, row := range []struct {
		key   string
		value string
	}{
		{"radarrIndexerFormat", "{title}"},
		{"sonarrIndexerFormat", "{title}"},
		{"cleanTitleRegex", ""},
	} {
		if _, err := db.ExecContext(context.Background(), `INSERT INTO system_config ("key", value, valid_status) VALUES (?, ?, 1)`, row.key, row.value); err != nil {
			t.Fatalf("insert Java formatter row: %v", err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close Java formatter fixture: %v", err)
	}
	return fixture
}

func newJavaEmptyFixture(t *testing.T) databaseFixture {
	t.Helper()
	path := filepath.Join(t.TempDir(), "runtime.db")
	db := openFixtureDatabase(t, path)
	if _, err := db.ExecContext(context.Background(), javaSchemaSQL); err != nil {
		t.Fatalf("create Java-compatible fixture schema: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close Java-compatible fixture: %v", err)
	}
	return databaseFixture{database: path}
}

func newIncompatibleFixture(t *testing.T) databaseFixture {
	t.Helper()
	path := filepath.Join(t.TempDir(), "incompatible.db")
	db := openFixtureDatabase(t, path)
	if _, err := db.ExecContext(context.Background(), `CREATE TABLE foreign_schema (id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatalf("create incompatible fixture: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close incompatible fixture: %v", err)
	}
	return databaseFixture{database: path}
}

func openFixtureDatabase(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open fixture database: %v", err)
	}
	return db
}

func assertMigrationLedgerAbsent(t *testing.T, path string) {
	t.Helper()
	if records := readMigrationLedger(t, path); len(records) != 0 {
		t.Fatalf("pre-start migration ledger = %#v, want absent", records)
	}
}

func readMigrationLedger(t *testing.T, path string) []string {
	t.Helper()
	db := openFixtureDatabase(t, path)
	t.Cleanup(func() { _ = db.Close() })
	var exists int
	if err := db.QueryRowContext(context.Background(), `SELECT count(*) FROM sqlite_schema WHERE type = 'table' AND name = 'jproxy_go_migration'`).Scan(&exists); err != nil {
		t.Fatalf("inspect migration ledger: %v", err)
	}
	if exists == 0 {
		return nil
	}
	rows, err := db.QueryContext(context.Background(), `SELECT version || ':' || checksum FROM jproxy_go_migration ORDER BY version`)
	if err != nil {
		t.Fatalf("read migration ledger: %v", err)
	}
	defer rows.Close()
	var records []string
	for rows.Next() {
		var record string
		if err := rows.Scan(&record); err != nil {
			t.Fatalf("scan migration ledger: %v", err)
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate migration ledger: %v", err)
	}
	return records
}

const javaSchemaSQL = `
CREATE TABLE system_user (id INTEGER PRIMARY KEY, username TEXT, password TEXT, role TEXT, valid_status INTEGER, create_time TEXT, update_time TEXT);
CREATE TABLE system_config (id INTEGER PRIMARY KEY, "key" TEXT, value TEXT, valid_status INTEGER, create_time TEXT, update_time TEXT);
CREATE TABLE sonarr_title (id INTEGER PRIMARY KEY, tvdb_id INTEGER, sno INTEGER, main_title TEXT, title TEXT, clean_title TEXT, season_number INTEGER, monitored INTEGER, valid_status INTEGER, create_time TEXT, update_time TEXT, series_id INTEGER);
CREATE INDEX sonarr_title_tvdb_id_idx ON sonarr_title (tvdb_id); CREATE INDEX sonarr_title_clean_title_idx ON sonarr_title (clean_title);
CREATE TABLE radarr_title (id INTEGER PRIMARY KEY, tmdb_id INTEGER, sno INTEGER, main_title TEXT, title TEXT, clean_title TEXT, year INTEGER, monitored INTEGER, valid_status INTEGER, create_time TEXT, update_time TEXT, movie_id INTEGER);
CREATE TABLE tmdb_title (id INTEGER PRIMARY KEY, tvdb_id INTEGER, tmdb_id INTEGER, language TEXT, title TEXT, valid_status INTEGER, create_time TEXT, update_time TEXT);
CREATE INDEX tmdb_title_tvdb_id_idx ON tmdb_title (tvdb_id); CREATE INDEX tmdb_title_tmdb_id_idx ON tmdb_title (tmdb_id);
CREATE TABLE sonarr_rule (id TEXT PRIMARY KEY, token TEXT, priority INTEGER, regex TEXT, replacement TEXT, offset INTEGER, example TEXT, remark TEXT, author TEXT, valid_status INTEGER, create_time TEXT, update_time TEXT);
CREATE TABLE radarr_rule (id TEXT PRIMARY KEY, token TEXT, priority INTEGER, regex TEXT, replacement TEXT, offset INTEGER, example TEXT, remark TEXT, author TEXT, valid_status INTEGER, create_time TEXT, update_time TEXT);
CREATE TABLE sonarr_example (hash TEXT PRIMARY KEY, original_text TEXT, format_text TEXT, valid_status INTEGER, create_time TEXT, update_time TEXT);
CREATE TABLE radarr_example (hash TEXT PRIMARY KEY, original_text TEXT, format_text TEXT, valid_status INTEGER, create_time TEXT, update_time TEXT);`
