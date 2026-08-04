package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"path/filepath"
)

type inspectionKind uint8

const (
	inspectionEmpty inspectionKind = iota
	inspectionJavaUnmanaged
	inspectionGoManaged
)

type inspection struct{ kind inspectionKind }

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

func inspectSchema(ctx context.Context, db *sql.DB) (inspection, error) {
	tables, err := tableSet(ctx, db)
	if err != nil {
		return inspection{}, err
	}
	if len(tables) == 0 {
		return inspection{kind: inspectionEmpty}, nil
	}
	if err := validateJavaSchema(ctx, db, tables); err != nil {
		return inspection{}, err
	}
	if !tables[migrationLedgerTable] {
		return inspection{kind: inspectionJavaUnmanaged}, nil
	}
	complete, err := validateLedger(ctx, db)
	if err != nil {
		return inspection{kind: inspectionJavaUnmanaged}, err
	}
	if !complete {
		return inspection{kind: inspectionJavaUnmanaged}, nil
	}
	return inspection{kind: inspectionGoManaged}, nil
}

func tableSet(ctx context.Context, db *sql.DB) (map[string]bool, error) {
	rows, err := db.QueryContext(ctx, `SELECT name FROM sqlite_schema WHERE type = 'table' AND name NOT LIKE 'sqlite_%'`)
	if err != nil {
		return nil, fmt.Errorf("list SQLite tables: %w", err)
	}
	defer rows.Close()
	tables := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scan SQLite table name: %w", err)
		}
		tables[name] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read SQLite tables: %w", err)
	}
	return tables, nil
}

func validateJavaSchema(ctx context.Context, db *sql.DB, tables map[string]bool) error {
	for table := range tables {
		if !allowedJavaTable(table) {
			return fmt.Errorf("unexpected pre-Go table %s: %w", table, ErrIncompatibleSchema)
		}
	}
	for _, table := range javaTables {
		if !tables[table.name] {
			return fmt.Errorf("missing required Java table %s: %w", table.name, ErrIncompatibleSchema)
		}
		if err := requireColumns(ctx, db, table.name, table.columns); err != nil {
			return err
		}
		if err := requireIndexes(ctx, db, table.name, table.indexes); err != nil {
			return err
		}
	}
	return nil
}

func allowedJavaTable(table string) bool {
	if table == migrationLedgerTable || table == "databasechangelog" || table == "databasechangeloglock" {
		return true
	}
	for _, contract := range javaTables {
		if contract.name == table {
			return true
		}
	}
	return false
}

type tableContract struct {
	name    string
	columns []string
	indexes []string
}

var javaTables = []tableContract{
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
