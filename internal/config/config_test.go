package config

import (
	"testing"
	"time"
)

func TestLoadConfigDefaults(t *testing.T) {
	t.Setenv("ADDR", "")
	t.Setenv("JACKETT_URL", "")
	t.Setenv("PROWLARR_URL", "")
	t.Setenv("MIN_COUNT", "")
	t.Setenv("INDEXER_RESULT_CACHE_EXPIRES", "")
	t.Setenv("RESULT_CACHE_MAX_ENTRIES", "")
	t.Setenv("CACHE_EXPIRES", "")
	t.Setenv("OFFSET_CACHE_MAX_ENTRIES", "")
	t.Setenv("HTTP_TIMEOUT_SECONDS", "")
	t.Setenv("JPROXY_LOGIN_ENABLED", "")
	t.Setenv("JPROXY_JWT_SECRET", "")
	t.Setenv("JPROXY_TOKEN_EXPIRES_MINUTES", "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if cfg.Addr != ":8117" {
		t.Fatalf("Addr = %q, want :8117", cfg.Addr)
	}
	if cfg.JackettURL != "http://127.0.0.1:9117" {
		t.Fatalf("JackettURL = %q", cfg.JackettURL)
	}
	if cfg.ProwlarrURL != "http://127.0.0.1:9696" {
		t.Fatalf("ProwlarrURL = %q", cfg.ProwlarrURL)
	}
	if cfg.MinCount != 6 {
		t.Fatalf("MinCount = %d, want 6", cfg.MinCount)
	}
	if cfg.IndexerResultCacheTTL != 15*time.Minute {
		t.Fatalf("IndexerResultCacheTTL = %s, want 15m", cfg.IndexerResultCacheTTL)
	}
	if cfg.ResultCacheMaxEntries != 1000 {
		t.Fatalf("ResultCacheMaxEntries = %d, want 1000", cfg.ResultCacheMaxEntries)
	}
	if cfg.OffsetCacheTTL != 4320*time.Minute {
		t.Fatalf("OffsetCacheTTL = %s, want 4320m", cfg.OffsetCacheTTL)
	}
	if cfg.OffsetCacheMaxEntries != 1000 {
		t.Fatalf("OffsetCacheMaxEntries = %d, want 1000", cfg.OffsetCacheMaxEntries)
	}
	if cfg.HTTPTimeout != 60*time.Second {
		t.Fatalf("HTTPTimeout = %s, want 60s", cfg.HTTPTimeout)
	}
}

func TestLoadConfig_rejectsInvalidEnabledJWTSecrets(t *testing.T) {
	tests := []string{"", "short", "change-me", "11111111111111111111111111111111", "your-32-byte-secret-key-here"}
	for _, secret := range tests {
		t.Run(secret, func(t *testing.T) {
			// Given
			t.Setenv("JPROXY_LOGIN_ENABLED", "true")
			t.Setenv("JPROXY_JWT_SECRET", secret)

			// When
			_, err := LoadConfig()

			// Then
			if err == nil {
				t.Fatal("LoadConfig() error = nil")
			}
		})
	}
}

func TestLoadConfig_loadsEnabledAuth(t *testing.T) {
	// Given
	t.Setenv("JPROXY_LOGIN_ENABLED", "true")
	t.Setenv("JPROXY_JWT_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("JPROXY_TOKEN_EXPIRES_MINUTES", "15")

	// When
	cfg, err := LoadConfig()

	// Then
	if err != nil || !cfg.Auth.LoginEnabled || cfg.Auth.TokenExpiresMinutes != 15 {
		t.Fatalf("LoadConfig() = %#v, %v", cfg, err)
	}
}

func TestLoadConfigEnvironmentOverridesAndTrimsURLs(t *testing.T) {
	t.Setenv("ADDR", " 127.0.0.1:9000 ")
	t.Setenv("JACKETT_URL", " http://jackett.local:9117/// ")
	t.Setenv("PROWLARR_URL", " http://prowlarr.local:9696/ ")
	t.Setenv("MIN_COUNT", "9")
	t.Setenv("INDEXER_RESULT_CACHE_EXPIRES", "2")
	t.Setenv("RESULT_CACHE_MAX_ENTRIES", "11")
	t.Setenv("CACHE_EXPIRES", "3")
	t.Setenv("OFFSET_CACHE_MAX_ENTRIES", "12")
	t.Setenv("HTTP_TIMEOUT_SECONDS", "4")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if cfg.Addr != "127.0.0.1:9000" {
		t.Fatalf("Addr = %q", cfg.Addr)
	}
	if cfg.JackettURL != "http://jackett.local:9117" {
		t.Fatalf("JackettURL = %q", cfg.JackettURL)
	}
	if cfg.ProwlarrURL != "http://prowlarr.local:9696" {
		t.Fatalf("ProwlarrURL = %q", cfg.ProwlarrURL)
	}
	if cfg.MinCount != 9 || cfg.IndexerResultCacheTTL != 2*time.Minute || cfg.ResultCacheMaxEntries != 11 || cfg.OffsetCacheTTL != 3*time.Minute || cfg.OffsetCacheMaxEntries != 12 || cfg.HTTPTimeout != 4*time.Second {
		t.Fatalf("unexpected config overrides: %+v", cfg)
	}
}

func TestEnvIntFallsBackOnInvalidValue(t *testing.T) {
	t.Setenv("MIN_COUNT", "not-a-number")
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if got := cfg.MinCount; got != 6 {
		t.Fatalf("MinCount with invalid env = %d, want default 6", got)
	}
}

func TestLoadConfigFallsBackOnInvalidCacheMaxEntries(t *testing.T) {
	// Given
	t.Setenv("RESULT_CACHE_MAX_ENTRIES", "not-a-number")
	t.Setenv("OFFSET_CACHE_MAX_ENTRIES", "also-not-a-number")

	// When
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	// Then
	if cfg.ResultCacheMaxEntries != 1000 {
		t.Fatalf("ResultCacheMaxEntries with invalid env = %d, want default 1000", cfg.ResultCacheMaxEntries)
	}
	if cfg.OffsetCacheMaxEntries != 1000 {
		t.Fatalf("OffsetCacheMaxEntries with invalid env = %d, want default 1000", cfg.OffsetCacheMaxEntries)
	}
}

func TestLoadConfigLoadsEnabledRadarrFormatting(t *testing.T) {
	// Given
	t.Setenv("JPROXY_RADARR_FORMAT_ENABLED", "true")
	t.Setenv("JPROXY_RADARR_FORMAT", "{title} {year}")
	t.Setenv("JPROXY_RADARR_TITLE_CLEAN_REGEX", `\b2024\b`)
	t.Setenv("JPROXY_RADARR_FORMAT_RULES", `[{"token":"title","regex":"^(.+)$","replacement":"$1"}]`)
	t.Setenv("JPROXY_RADARR_TITLES", `[{"mainTitle":"Movie","title":"Movie","year":2024}]`)

	// When
	cfg, err := LoadConfig()

	// Then
	if err != nil || !cfg.RadarrFormatting.Enabled || cfg.RadarrFormatting.Config.Format != "{title} {year}" || len(cfg.RadarrFormatting.Config.Titles) != 1 {
		t.Fatalf("LoadConfig() = %#v, %v", cfg, err)
	}
}

func TestLoadConfigDefaultsOmittedRadarrRulePriorityAndStatus(t *testing.T) {
	// Given
	t.Setenv("JPROXY_RADARR_FORMAT_ENABLED", "true")
	t.Setenv("JPROXY_RADARR_FORMAT", "{title}")
	t.Setenv("JPROXY_RADARR_FORMAT_RULES", `[{"token":"title","regex":"^(.+)$","replacement":"$1"}]`)

	// When
	cfg, err := LoadConfig()

	// Then
	if err != nil || len(cfg.RadarrFormatting.Config.Rules) != 1 || cfg.RadarrFormatting.Config.Rules[0].Priority != 0 || cfg.RadarrFormatting.Config.Rules[0].ValidStatus != nil {
		t.Fatalf("LoadConfig() = %#v, %v, want omitted priority=0 and validStatus=nil", cfg, err)
	}
}

func TestLoadConfigRejectsMalformedEnabledRadarrFormatting(t *testing.T) {
	// Given
	t.Setenv("JPROXY_RADARR_FORMAT_ENABLED", "true")
	t.Setenv("JPROXY_RADARR_FORMAT", "{title}")
	t.Setenv("JPROXY_RADARR_FORMAT_RULES", `[{"token":"title","regex":"["}]`)

	// When
	_, err := LoadConfig()

	// Then
	if err == nil {
		t.Fatal("LoadConfig() error = nil, want invalid enabled formatting config error")
	}
}

func TestLoadConfigRejectsMalformedRadarrFormattingEnabledFlag(t *testing.T) {
	// Given
	t.Setenv("JPROXY_RADARR_FORMAT_ENABLED", "sometimes")

	// When
	_, err := LoadConfig()

	// Then
	if err == nil {
		t.Fatal("LoadConfig() error = nil, want invalid boolean error")
	}
}

func TestLoadConfigRejectsInvalidEnabledRadarrCleanTitleRegex(t *testing.T) {
	// Given
	t.Setenv("JPROXY_RADARR_FORMAT_ENABLED", "true")
	t.Setenv("JPROXY_RADARR_FORMAT", "{title}")
	t.Setenv("JPROXY_RADARR_TITLE_CLEAN_REGEX", "[")

	// When
	_, err := LoadConfig()

	// Then
	if err == nil {
		t.Fatal("LoadConfig() error = nil, want invalid clean title regex error")
	}
}

func TestLoadConfigRejectsInvalidEnabledRadarrRuleValidStatus(t *testing.T) {
	// Given
	t.Setenv("JPROXY_RADARR_FORMAT_ENABLED", "true")
	t.Setenv("JPROXY_RADARR_FORMAT", "{title}")
	t.Setenv("JPROXY_RADARR_FORMAT_RULES", `[{"token":"title","regex":".*","validStatus":2}]`)

	// When
	_, err := LoadConfig()

	// Then
	if err == nil {
		t.Fatal("LoadConfig() error = nil, want invalid validStatus error")
	}
}

func TestLoadConfigLoadsEnabledSonarrFormattingIndependently(t *testing.T) {
	// Given
	t.Setenv("JPROXY_SONARR_FORMAT_ENABLED", "true")
	t.Setenv("JPROXY_SONARR_FORMAT", "{title} {season}")
	t.Setenv("JPROXY_SONARR_TITLE_CLEAN_REGEX", `\bseason\b`)
	t.Setenv("JPROXY_SONARR_FORMAT_RULES", `[{"token":"title","regex":"^(.+)$","replacement":"$1"}]`)
	t.Setenv("JPROXY_SONARR_TITLES", `[{"mainTitle":"Show","title":"Show","seasonNumber":2}]`)

	// When
	cfg, err := LoadConfig()

	// Then
	if err != nil || !cfg.SonarrFormatting.Enabled || cfg.SonarrFormatting.Config.Format != "{title} {season}" || len(cfg.SonarrFormatting.Config.Titles) != 1 || cfg.RadarrFormatting.Enabled {
		t.Fatalf("LoadConfig() = %#v, %v", cfg, err)
	}
}

func TestLoadConfigRejectsInvalidEnabledSonarrFormatting(t *testing.T) {
	// Given
	t.Setenv("JPROXY_SONARR_FORMAT_ENABLED", "true")
	t.Setenv("JPROXY_SONARR_FORMAT", "{title}")
	t.Setenv("JPROXY_SONARR_TITLES", `[{"mainTitle":"Show","title":"Show"}]`)

	// When
	_, err := LoadConfig()

	// Then
	if err == nil {
		t.Fatal("LoadConfig() error = nil, want Sonarr title validation error")
	}
}

func TestLoadConfig_recordsEnabledDatabaseWithoutOpeningIt(t *testing.T) {
	// Given
	t.Setenv("JPROXY_DB_ENABLED", "true")
	t.Setenv("JPROXY_DB_PATH", "  unopenable-secret-path.db  ")
	t.Setenv("JPROXY_RADARR_FORMAT_ENABLED", "true")
	t.Setenv("JPROXY_SONARR_FORMAT_ENABLED", "true")
	t.Setenv("JPROXY_RADARR_FORMAT", "not a valid env formatter")
	t.Setenv("JPROXY_SONARR_FORMAT", "not a valid env formatter")

	// When
	cfg, err := LoadConfig()

	// Then
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if !cfg.Database.Enabled || cfg.Database.Path != "unopenable-secret-path.db" || !cfg.RadarrFormatting.Enabled || !cfg.SonarrFormatting.Enabled || cfg.RadarrFormatting.Config.Format != "" || cfg.SonarrFormatting.Config.Format != "" {
		t.Fatalf("LoadConfig() = %#v", cfg)
	}
}

func TestLoadConfig_keepsEnvironmentFormattersWhenDatabaseIsDisabled(t *testing.T) {
	// Given
	t.Setenv("JPROXY_DB_ENABLED", "false")
	t.Setenv("JPROXY_DB_PATH", " ignored.db ")
	t.Setenv("JPROXY_RADARR_FORMAT_ENABLED", "true")
	t.Setenv("JPROXY_RADARR_FORMAT", "{title}")
	t.Setenv("JPROXY_RADARR_FORMAT_RULES", `[{"token":"title","regex":"^(.+)$","replacement":"$1"}]`)

	// When
	cfg, err := LoadConfig()

	// Then
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if cfg.Database.Enabled || !cfg.RadarrFormatting.Enabled || cfg.RadarrFormatting.Config.Format != "{title}" {
		t.Fatalf("LoadConfig() = %#v", cfg)
	}
}

func TestLoadConfig_rejectsEnabledDatabaseWithoutPath(t *testing.T) {
	// Given
	t.Setenv("JPROXY_DB_ENABLED", "true")
	t.Setenv("JPROXY_DB_PATH", "")

	// When
	_, err := LoadConfig()

	// Then
	if err == nil {
		t.Fatal("LoadConfig() error = nil, want required database path error")
	}
}
