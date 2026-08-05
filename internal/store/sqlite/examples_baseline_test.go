package sqlite

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

func TestExampleMigration_schemaAndChecksumRemainCompatible(t *testing.T) {
	// Given: the released example migration embedded by the SQLite lifecycle.
	const wantChecksum = "3e07db2885adbf53b048d7823ce96811e0746acf050fc5be57b7eb05622a2cda"

	// When: its schema and ledger checksum are characterized.
	digest := sha256.Sum256([]byte(exampleMigrationSQL))
	checksum := hex.EncodeToString(digest[:])
	t.Logf("0002_examples checksum=%s", checksum)

	// Then: both example tables and their persisted fields remain present.
	for _, fragment := range []string{
		"CREATE TABLE IF NOT EXISTS sonarr_example",
		"CREATE TABLE IF NOT EXISTS radarr_example",
		"hash TEXT PRIMARY KEY",
		"original_text TEXT NOT NULL",
		"format_text TEXT",
		"valid_status INTEGER NOT NULL DEFAULT 1",
	} {
		if !strings.Contains(exampleMigrationSQL, fragment) {
			t.Fatalf("example migration is missing %q", fragment)
		}
	}
	if checksum != wantChecksum {
		t.Fatalf("example migration checksum=%s want=%s", checksum, wantChecksum)
	}
	if embeddedMigrations[1].checksum != checksum {
		t.Fatalf("embedded checksum=%s want=%s", embeddedMigrations[1].checksum, checksum)
	}
}
