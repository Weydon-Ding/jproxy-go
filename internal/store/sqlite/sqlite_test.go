package sqlite_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	store "jproxy-go/internal/store/sqlite"
)

func TestLoad_buildsValidatedSnapshotFromActiveRows(t *testing.T) {
	// Given
	path := createDatabase(t, []string{
		`INSERT INTO system_config(key, value, valid_status) VALUES
			('radarrIndexerFormat', '{title} ({year})', 1),
			('sonarrIndexerFormat', '{title} {season}', 1),
			('cleanTitleRegex', '\b(?:the)\b', 1)`,
		`INSERT INTO radarr_rule(token, regex, valid_status) VALUES ('title', '^(.+?)\\.\\d{4}.*$', 1)`,
		`INSERT INTO sonarr_rule(token, regex, valid_status) VALUES ('title', '^(.+?)\\.S\\d+E\\d+.*$', 1)`,
		`INSERT INTO radarr_title(main_title, title, year, valid_status) VALUES ('Movie', 'Movie', 2024, 1)`,
		`INSERT INTO sonarr_title(main_title, title, valid_status) VALUES ('Show', 'Show', 1), ('Other Show', 'Other Show', 1)`,
	})

	// When
	snapshot, err := store.Load(context.Background(), path)

	// Then
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if snapshot.Radarr.Format != "{title} ({year})" || snapshot.Sonarr.Format != "{title} {season}" {
		t.Fatalf("formats = %#v %#v", snapshot.Radarr, snapshot.Sonarr)
	}
	if snapshot.Radarr.CleanTitleRegex != `\b(?:the)\b` || snapshot.Sonarr.CleanTitleRegex != `\b(?:the)\b` {
		t.Fatalf("clean regexes = %q %q", snapshot.Radarr.CleanTitleRegex, snapshot.Sonarr.CleanTitleRegex)
	}
	if len(snapshot.Radarr.Rules) != 1 || snapshot.Radarr.Rules[0].Priority != 1000 || snapshot.Radarr.Rules[0].Replacement != "" || snapshot.Radarr.Rules[0].Offset != 0 {
		t.Fatalf("Radarr rules = %#v", snapshot.Radarr.Rules)
	}
	if len(snapshot.Sonarr.Rules) != 1 || snapshot.Sonarr.Rules[0].Priority != 1000 || snapshot.Sonarr.Rules[0].Replacement != "" || snapshot.Sonarr.Rules[0].Offset != 0 {
		t.Fatalf("Sonarr rules = %#v", snapshot.Sonarr.Rules)
	}
	if len(snapshot.Radarr.Titles) != 1 || snapshot.Radarr.Titles[0].CleanTitle != "" || snapshot.Radarr.Titles[0].Year != 2024 {
		t.Fatalf("Radarr titles = %#v", snapshot.Radarr.Titles)
	}
	if len(snapshot.Sonarr.Titles) != 2 || snapshot.Sonarr.Titles[0].CleanTitle != "" || snapshot.Sonarr.Titles[0].SeasonNumber == nil || *snapshot.Sonarr.Titles[0].SeasonNumber != 1 || snapshot.Sonarr.Titles[1].SeasonNumber == nil || snapshot.Sonarr.Titles[0].SeasonNumber == snapshot.Sonarr.Titles[1].SeasonNumber {
		t.Fatalf("Sonarr titles = %#v", snapshot.Sonarr.Titles)
	}
}

func TestLoad_filtersRowsAndOrdersRulesByPriority(t *testing.T) {
	// Given
	path := createDatabase(t, []string{
		validSystemConfigStatement,
		`INSERT INTO radarr_rule(token, priority, regex, replacement, offset, valid_status) VALUES
			('title', 10, '^late$', 'late', 0, 1),
			('title', 1, '^early$', 'early', 0, 1),
			('title', 0, '^disabled$', 'disabled', 0, 0)`,
		`INSERT INTO sonarr_rule(token, priority, regex, replacement, offset, valid_status) VALUES
			('title', 5, '^enabled$', 'enabled', 2, 1),
			('title', 1, '^disabled$', 'disabled', 0, 0)`,
		`INSERT INTO radarr_title(main_title, title, year, valid_status) VALUES
			('Enabled Movie', 'Enabled Movie', 2024, 1),
			('Disabled Movie', 'Disabled Movie', 2025, 0)`,
		`INSERT INTO sonarr_title(main_title, title, season_number, valid_status) VALUES
			('Enabled Show', 'Enabled Show', 2, 1),
			('Disabled Show', 'Disabled Show', 3, 0)`,
	})

	// When
	snapshot, err := store.Load(context.Background(), path)

	// Then
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(snapshot.Radarr.Rules) != 2 || snapshot.Radarr.Rules[0].Replacement != "early" || snapshot.Radarr.Rules[1].Replacement != "late" {
		t.Fatalf("Radarr rules = %#v, want enabled rules in priority order", snapshot.Radarr.Rules)
	}
	if len(snapshot.Sonarr.Rules) != 1 || snapshot.Sonarr.Rules[0].Replacement != "enabled" || snapshot.Sonarr.Rules[0].Offset != 2 {
		t.Fatalf("Sonarr rules = %#v, want enabled rule only", snapshot.Sonarr.Rules)
	}
	if len(snapshot.Radarr.Titles) != 1 || snapshot.Radarr.Titles[0].MainTitle != "Enabled Movie" || len(snapshot.Sonarr.Titles) != 1 || snapshot.Sonarr.Titles[0].MainTitle != "Enabled Show" {
		t.Fatalf("titles = %#v %#v, want enabled titles only", snapshot.Radarr.Titles, snapshot.Sonarr.Titles)
	}
}

func TestLoad_rejectsDuplicateActiveSystemConfig(t *testing.T) {
	// Given
	path := createDatabase(t, []string{
		`INSERT INTO system_config(key, value, valid_status) VALUES
			('radarrIndexerFormat', '{title}', 1),
			('radarrIndexerFormat', '{title} duplicate', 1),
			('sonarrIndexerFormat', '{title}', 1),
			('cleanTitleRegex', '', 1)`,
	})

	// When
	_, err := store.Load(context.Background(), path)

	// Then
	if err == nil || !strings.Contains(err.Error(), "radarrIndexerFormat") {
		t.Fatalf("Load() error = %v, want duplicate system config error", err)
	}
}

func TestLoad_rejectsMissingDatabaseWithoutCreatingIt(t *testing.T) {
	// Given
	path := filepath.Join(t.TempDir(), "missing formatters.db")

	// When
	_, err := store.Load(context.Background(), path)

	// Then
	if err == nil {
		t.Fatal("Load() error = nil, want missing database error")
	}
}

func TestLoad_handlesDatabasePathWithSpaces(t *testing.T) {
	// Given
	path := createDatabaseAt(t, filepath.Join(t.TempDir(), "format data.db"), []string{
		validSystemConfigStatement,
		`INSERT INTO radarr_rule(token, regex, valid_status) VALUES ('title', '^(.+)$', 1)`,
		`INSERT INTO sonarr_rule(token, regex, valid_status) VALUES ('title', '^(.+)$', 1)`,
	})

	// When
	_, err := store.Load(context.Background(), path)

	// Then
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
}

func TestLoad_rejectsMissingActiveSystemConfig(t *testing.T) {
	// Given
	path := createDatabase(t, nil)

	// When
	_, err := store.Load(context.Background(), path)

	// Then
	if err == nil {
		t.Fatal("Load() error = nil, want missing system config error")
	}
}

func createDatabase(t *testing.T, statements []string) string {
	t.Helper()
	return createDatabaseAt(t, filepath.Join(t.TempDir(), "formatters.db"), statements)
}

func createDatabaseAt(t *testing.T, path string, statements []string) string {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := db.Close(); closeErr != nil {
			t.Errorf("close test database: %v", closeErr)
		}
	})
	for _, statement := range append(schemaStatements, statements...) {
		if _, execErr := db.Exec(statement); execErr != nil {
			t.Fatalf("execute %q: %v", statement, execErr)
		}
	}
	return path
}

const validSystemConfigStatement = `INSERT INTO system_config(key, value, valid_status) VALUES
	('radarrIndexerFormat', '{title}', 1),
	('sonarrIndexerFormat', '{title}', 1),
	('cleanTitleRegex', '', 1)`

var schemaStatements = []string{
	`CREATE TABLE system_config (key TEXT, value TEXT, valid_status INTEGER)`,
	`CREATE TABLE radarr_rule (token TEXT, priority INTEGER DEFAULT 1000, regex TEXT, replacement TEXT DEFAULT '', offset INTEGER DEFAULT 0, valid_status INTEGER DEFAULT 1)`,
	`CREATE TABLE sonarr_rule (token TEXT, priority INTEGER DEFAULT 1000, regex TEXT, replacement TEXT DEFAULT '', offset INTEGER DEFAULT 0, valid_status INTEGER DEFAULT 1)`,
	`CREATE TABLE radarr_title (main_title TEXT, title TEXT, clean_title TEXT, year INTEGER, valid_status INTEGER DEFAULT 1)`,
	`CREATE TABLE sonarr_title (main_title TEXT, title TEXT, clean_title TEXT, season_number INTEGER DEFAULT 1, valid_status INTEGER DEFAULT 1)`,
}
