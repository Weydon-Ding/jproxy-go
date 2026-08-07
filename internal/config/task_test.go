package config

import "testing"

func TestLoadConfig_defaultsRenameFiles(t *testing.T) {
	// Given
	t.Setenv("RENAME_FILE", "")

	// When
	cfg, err := LoadConfig()

	// Then
	if err != nil || !cfg.Task.RenameFiles {
		t.Fatalf("LoadConfig() = %#v, %v", cfg, err)
	}
}

func TestLoadConfig_loadsRenameFilesInEnvironmentAndDatabaseModes(t *testing.T) {
	t.Run("environment", func(t *testing.T) {
		// Given
		t.Setenv("RENAME_FILE", "false")

		// When
		cfg, err := LoadConfig()

		// Then
		if err != nil || cfg.Task.RenameFiles {
			t.Fatalf("LoadConfig() = %#v, %v", cfg, err)
		}
	})
	t.Run("database", func(t *testing.T) {
		// Given
		t.Setenv("JPROXY_DB_ENABLED", "true")
		t.Setenv("JPROXY_DB_PATH", "config.db")
		t.Setenv("RENAME_FILE", "false")

		// When
		cfg, err := LoadConfig()

		// Then
		if err != nil || !cfg.Database.Enabled || cfg.Task.RenameFiles {
			t.Fatalf("LoadConfig() = %#v, %v", cfg, err)
		}
	})
}

func TestLoadConfig_rejectsInvalidRenameFiles(t *testing.T) {
	// Given
	t.Setenv("RENAME_FILE", "sometimes")

	// When
	_, err := LoadConfig()

	// Then
	if err == nil {
		t.Fatal("LoadConfig() error = nil, want invalid boolean error")
	}
}
