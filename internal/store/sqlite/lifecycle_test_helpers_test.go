package sqlite

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func javaFixture(t *testing.T) string {
	return javaFinalFixture(t)
}

var javaFinalTables = []tableContract{
	{"system_user", []string{"id", "username", "password", "role", "valid_status", "create_time", "update_time"}, nil},
	{"system_config", []string{"id", "key", "value", "valid_status", "create_time", "update_time"}, nil},
	{"sonarr_title", []string{"id", "tvdb_id", "sno", "main_title", "title", "clean_title", "season_number", "monitored", "valid_status", "create_time", "update_time", "series_id"}, []string{"sonarr_title_tvdb_id_idx", "sonarr_title_clean_title_idx"}},
	{"radarr_title", []string{"id", "tmdb_id", "sno", "main_title", "title", "clean_title", "year", "monitored", "valid_status", "create_time", "update_time", "movie_id"}, nil},
	{"tmdb_title", []string{"id", "tvdb_id", "tmdb_id", "language", "title", "valid_status", "create_time", "update_time"}, []string{"tmdb_title_tvdb_id_idx", "tmdb_title_tmdb_id_idx"}},
	{"sonarr_rule", []string{"id", "token", "priority", "regex", "replacement", "offset", "example", "remark", "author", "valid_status", "create_time", "update_time"}, nil},
	{"radarr_rule", []string{"id", "token", "priority", "regex", "replacement", "offset", "example", "remark", "author", "valid_status", "create_time", "update_time"}, nil},
	{"sonarr_example", []string{"hash", "original_text", "format_text", "valid_status", "create_time", "update_time"}, nil},
	{"radarr_example", []string{"hash", "original_text", "format_text", "valid_status", "create_time", "update_time"}, nil},
}

func javaFinalFixture(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "java-final-schema.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open Java final fixture: %v", err)
	}
	defer db.Close()
	for _, statement := range splitStatements(javaFinalSchemaSQL) {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("create Java final fixture: %v", err)
		}
	}
	for _, statement := range []string{
		`INSERT INTO system_config (id, "key", value, valid_status) VALUES (1, 'radarrIndexerFormat', '{title}', 1), (2, 'sonarrIndexerFormat', '{title}', 1), (3, 'cleanTitleRegex', '', 1)`,
		`INSERT INTO radarr_rule (id, token, regex) VALUES ('r', 'title', '^(.+)$')`,
		`INSERT INTO sonarr_rule (id, token, regex) VALUES ('s', 'title', '^(.+)$')`,
		`INSERT INTO radarr_example (hash, original_text) VALUES ('r1', 'radarr')`,
		`INSERT INTO sonarr_example (hash, original_text) VALUES ('s1', 'sonarr')`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("seed Java final fixture: %v", err)
		}
	}
	return path
}

const javaFinalSchemaSQL = `
CREATE TABLE system_user (id INTEGER PRIMARY KEY, username TEXT NOT NULL, password TEXT, role TEXT DEFAULT 'admin', valid_status INTEGER NOT NULL DEFAULT 1, create_time DATETIME, update_time DATETIME);
CREATE TABLE system_config (id INTEGER PRIMARY KEY, "key" TEXT NOT NULL, value TEXT, valid_status INTEGER NOT NULL DEFAULT 1, create_time DATETIME, update_time DATETIME);
CREATE TABLE sonarr_title (id INTEGER PRIMARY KEY, tvdb_id INTEGER NOT NULL, sno INTEGER NOT NULL DEFAULT 0, main_title TEXT NOT NULL, title TEXT NOT NULL, clean_title TEXT NOT NULL, season_number INTEGER NOT NULL DEFAULT 1, monitored INTEGER NOT NULL DEFAULT 1, valid_status INTEGER NOT NULL DEFAULT 1, create_time DATETIME, update_time DATETIME, series_id INTEGER);
CREATE INDEX sonarr_title_tvdb_id_idx ON sonarr_title(tvdb_id);
CREATE INDEX sonarr_title_clean_title_idx ON sonarr_title(clean_title);
CREATE TABLE tmdb_title (id INTEGER PRIMARY KEY AUTOINCREMENT, tvdb_id INTEGER NOT NULL, tmdb_id INTEGER, language VARCHAR(8) NOT NULL, title TEXT NOT NULL, valid_status INTEGER NOT NULL DEFAULT 1, create_time DATETIME, update_time DATETIME);
CREATE INDEX tmdb_title_tvdb_id_idx ON tmdb_title(tvdb_id);
CREATE INDEX tmdb_title_tmdb_id_idx ON tmdb_title(tmdb_id);
CREATE TABLE sonarr_rule (id TEXT PRIMARY KEY, token TEXT NOT NULL, priority INTEGER NOT NULL DEFAULT 1000, regex TEXT NOT NULL, replacement TEXT NOT NULL DEFAULT '', offset INTEGER NOT NULL DEFAULT 0, example TEXT NOT NULL DEFAULT '', remark TEXT, author TEXT, valid_status INTEGER NOT NULL DEFAULT 1, create_time DATETIME, update_time DATETIME);
CREATE TABLE sonarr_example (hash TEXT PRIMARY KEY, original_text TEXT NOT NULL, format_text TEXT, valid_status INTEGER NOT NULL DEFAULT 1, create_time DATETIME, update_time DATETIME);
CREATE TABLE radarr_title (id INTEGER PRIMARY KEY, tmdb_id INTEGER NOT NULL, sno INTEGER NOT NULL DEFAULT 0, main_title TEXT NOT NULL, title TEXT NOT NULL, clean_title TEXT NOT NULL, year INTEGER NOT NULL, monitored INTEGER NOT NULL DEFAULT 1, valid_status INTEGER NOT NULL DEFAULT 1, create_time DATETIME, update_time DATETIME, movie_id INTEGER);
CREATE TABLE radarr_rule (id TEXT PRIMARY KEY, token TEXT NOT NULL, priority INTEGER NOT NULL DEFAULT 1000, regex TEXT NOT NULL, replacement TEXT NOT NULL DEFAULT '', offset INTEGER NOT NULL DEFAULT 0, example TEXT NOT NULL DEFAULT '', remark TEXT, author TEXT, valid_status INTEGER NOT NULL DEFAULT 1, create_time DATETIME, update_time DATETIME);
CREATE TABLE radarr_example (hash TEXT PRIMARY KEY, original_text TEXT NOT NULL, format_text TEXT, valid_status INTEGER NOT NULL DEFAULT 1, create_time DATETIME, update_time DATETIME);`

func splitStatements(statement string) []string {
	var statements []string
	for _, part := range strings.Split(statement, ";") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			statements = append(statements, trimmed)
		}
	}
	return statements
}

func appliedVersions(t *testing.T, path string) []string {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open database for ledger: %v", err)
	}
	defer db.Close()
	rows, err := db.Query(`SELECT version FROM jproxy_go_migration ORDER BY version`)
	if err != nil {
		t.Fatalf("query migration ledger: %v", err)
	}
	defer rows.Close()
	var versions []string
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			t.Fatalf("scan ledger version: %v", err)
		}
		versions = append(versions, version)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read ledger versions: %v", err)
	}
	return versions
}

func tableCounts(t *testing.T, path string) map[string]int {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open counts database: %v", err)
	}
	defer db.Close()
	counts := make(map[string]int, len(javaTables))
	for _, contract := range javaTables {
		var count int
		if err := db.QueryRow(`SELECT count(*) FROM "` + contract.name + `"`).Scan(&count); err != nil {
			t.Fatalf("count %s: %v", contract.name, err)
		}
		counts[contract.name] = count
	}
	return counts
}

func sameCounts(left, right map[string]int) bool {
	if len(left) != len(right) {
		return false
	}
	for table, count := range left {
		if right[table] != count {
			return false
		}
	}
	return true
}

func fileHash(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

func logicalDigest(t *testing.T, path string) string {
	t.Helper()
	db := openTestDB(t, path)
	defer closeTestDB(t, db)
	hash := sha256.New()
	for _, contract := range javaFinalTables {
		fmt.Fprintf(hash, "table:%s\n", contract.name)
		rows, err := db.Query(`SELECT * FROM "` + contract.name + `" ORDER BY rowid`)
		if err != nil {
			t.Fatalf("query %s for digest: %v", contract.name, err)
		}
		columns, err := rows.Columns()
		if err != nil {
			rows.Close()
			t.Fatalf("columns %s for digest: %v", contract.name, err)
		}
		for rows.Next() {
			values := make([]any, len(columns))
			destinations := make([]any, len(columns))
			for index := range values {
				destinations[index] = &values[index]
			}
			if err := rows.Scan(destinations...); err != nil {
				rows.Close()
				t.Fatalf("scan %s for digest: %v", contract.name, err)
			}
			for _, value := range values {
				fmt.Fprintf(hash, "%T:%v|", value, value)
			}
			fmt.Fprintln(hash)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			t.Fatalf("read %s for digest: %v", contract.name, err)
		}
		if err := rows.Close(); err != nil {
			t.Fatalf("close %s digest rows: %v", contract.name, err)
		}
	}
	return hex.EncodeToString(hash.Sum(nil))
}
