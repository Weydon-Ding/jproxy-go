package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"path/filepath"
)

type schemaKind uint8

const (
	schemaEmpty schemaKind = iota
	schemaJava
	schemaGo
	schemaUnknown
)

func openWritable(ctx context.Context, path string) (*sql.DB, error) {
	dsn, err := writableDSN(path)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open writable SQLite database: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping writable SQLite database: %w", err)
	}
	return db, nil
}

func openReadOnly(ctx context.Context, path string) (*sql.DB, error) {
	dsn, err := readOnlyDSN(path)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open read-only SQLite database: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping read-only SQLite database: %w", err)
	}
	return db, nil
}

func writableDSN(path string) (string, error) {
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve SQLite database path: %w", err)
	}
	query := url.Values{
		"_pragma": []string{
			"foreign_keys(1)",
			fmt.Sprintf("busy_timeout(%d)", busyTimeoutMilliseconds),
		},
	}
	return (&url.URL{Scheme: "file", Opaque: filepath.ToSlash(absolutePath), RawQuery: query.Encode()}).String(), nil
}

func configureJournal(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `PRAGMA journal_mode = WAL`); err != nil {
		return fmt.Errorf("set SQLite WAL journal policy: %w", err)
	}
	if _, err := db.ExecContext(ctx, `PRAGMA synchronous = FULL`); err != nil {
		return fmt.Errorf("set SQLite synchronous policy: %w", err)
	}
	return nil
}

func inspectSchema(ctx context.Context, db *sql.DB) (schemaKind, error) {
	rows, err := db.QueryContext(ctx, `SELECT name FROM sqlite_schema WHERE type = 'table' AND name NOT LIKE 'sqlite_%'`)
	if err != nil {
		return schemaUnknown, fmt.Errorf("list SQLite tables: %w", err)
	}
	defer rows.Close()
	tables := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return schemaUnknown, fmt.Errorf("scan SQLite table name: %w", err)
		}
		tables[name] = true
	}
	if err := rows.Err(); err != nil {
		return schemaUnknown, fmt.Errorf("read SQLite tables: %w", err)
	}
	if len(tables) == 0 {
		return schemaEmpty, nil
	}
	for _, table := range javaTables {
		if !tables[table.name] {
			return schemaUnknown, nil
		}
		if err := requireColumns(ctx, db, table.name, table.columns); err != nil {
			return schemaUnknown, err
		}
		if err := requireIndexes(ctx, db, table.name, table.indexes); err != nil {
			return schemaUnknown, err
		}
	}
	if tables[migrationLedgerTable] {
		return schemaGo, nil
	}
	return schemaJava, nil
}

type tableContract struct {
	name    string
	columns []string
	indexes []string
}

var javaTables = []tableContract{
	{"system_user", []string{"id", "username", "password", "role", "valid_status", "create_time", "update_time"}, nil},
	{"system_config", []string{"id", "key", "value", "valid_status", "create_time", "update_time"}, nil},
	{"sonarr_title", []string{"id", "tvdb_id", "main_title", "title", "clean_title", "season_number", "valid_status", "series_id"}, []string{"sonarr_title_tvdb_id_idx", "sonarr_title_clean_title_idx"}},
	{"radarr_title", []string{"id", "tmdb_id", "main_title", "title", "clean_title", "year", "valid_status", "movie_id"}, nil},
	{"tmdb_title", []string{"id", "tvdb_id", "tmdb_id", "language", "title", "valid_status"}, []string{"tmdb_title_tvdb_id_idx", "tmdb_title_tmdb_id_idx"}},
	{"sonarr_rule", []string{"id", "token", "priority", "regex", "replacement", "offset", "valid_status"}, nil},
	{"radarr_rule", []string{"id", "token", "priority", "regex", "replacement", "offset", "valid_status"}, nil},
}

func requireColumns(ctx context.Context, db *sql.DB, table string, required []string) error {
	rows, err := db.QueryContext(ctx, `SELECT name FROM pragma_table_info(?)`, table)
	if err != nil {
		return fmt.Errorf("inspect %s columns: %w", table, err)
	}
	defer rows.Close()
	columns := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return fmt.Errorf("scan %s column: %w", table, err)
		}
		columns[name] = true
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read %s columns: %w", table, err)
	}
	for _, column := range required {
		if !columns[column] {
			return fmt.Errorf("%w: table %s is missing column %s", ErrIncompatibleSchema, table, column)
		}
	}
	return nil
}

func requireIndexes(ctx context.Context, db *sql.DB, table string, required []string) error {
	if len(required) == 0 {
		return nil
	}
	rows, err := db.QueryContext(ctx, `SELECT name FROM pragma_index_list(?)`, table)
	if err != nil {
		return fmt.Errorf("inspect %s indexes: %w", table, err)
	}
	defer rows.Close()
	indexes := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return fmt.Errorf("scan %s index: %w", table, err)
		}
		indexes[name] = true
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read %s indexes: %w", table, err)
	}
	for _, index := range required {
		if !indexes[index] {
			return fmt.Errorf("%w: table %s is missing index %s", ErrIncompatibleSchema, table, index)
		}
	}
	return nil
}
