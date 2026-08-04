package sqlite

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func javaFixture(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "java-schema.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open Java fixture: %v", err)
	}
	defer db.Close()
	for _, statement := range splitStatements(initialMigrationSQL) {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("create Java fixture schema: %v", err)
		}
	}
	statements := []string{
		`INSERT INTO system_config (id, "key", value, valid_status) VALUES (1, 'radarrIndexerFormat', '{title}', 1), (2, 'sonarrIndexerFormat', '{title}', 1), (3, 'cleanTitleRegex', '', 1)`,
		`INSERT INTO radarr_rule (id, token, regex, valid_status) VALUES ('r', 'title', '^(.+)$', 1)`,
		`INSERT INTO sonarr_rule (id, token, regex, valid_status) VALUES ('s', 'title', '^(.+)$', 1)`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("seed Java fixture: %v", err)
		}
	}
	return path
}

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
