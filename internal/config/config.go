package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"jproxy-go/internal/format"
)

var ErrDatabasePathRequired = errors.New("database path required")

var ErrJWTSecretInvalid = errors.New("JWT secret must be at least 32 bytes and not a default value")

type Config struct {
	Addr                  string
	JackettURL            string
	ProwlarrURL           string
	MinCount              int
	IndexerResultCacheTTL time.Duration
	ResultCacheMaxEntries int
	OffsetCacheTTL        time.Duration
	OffsetCacheMaxEntries int
	HTTPTimeout           time.Duration
	Database              DatabaseConfig
	RadarrFormatting      RadarrFormattingConfig
	SonarrFormatting      SonarrFormattingConfig
	Auth                  AuthConfig
}

type AuthConfig struct {
	LoginEnabled        bool
	JWTSecret           string
	TokenExpiresMinutes int
}

// DatabaseConfig selects the long-lived writable SQLite application store.
// Loading configuration intentionally does not open the database.
type DatabaseConfig struct {
	Enabled bool
	Path    string
}

type RadarrFormattingConfig struct {
	Enabled bool
	Config  format.Config
}

type SonarrFormattingConfig struct {
	Enabled bool
	Config  format.SonarrConfig
}

func LoadConfig() (Config, error) {
	cfg := Config{
		Addr:                  env("ADDR", ":8117"),
		JackettURL:            trimRightSlash(env("JACKETT_URL", "http://127.0.0.1:9117")),
		ProwlarrURL:           trimRightSlash(env("PROWLARR_URL", "http://127.0.0.1:9696")),
		MinCount:              envInt("MIN_COUNT", 6),
		IndexerResultCacheTTL: time.Duration(envInt("INDEXER_RESULT_CACHE_EXPIRES", 15)) * time.Minute,
		ResultCacheMaxEntries: envInt("RESULT_CACHE_MAX_ENTRIES", 1000),
		OffsetCacheTTL:        time.Duration(envInt("CACHE_EXPIRES", 4320)) * time.Minute,
		OffsetCacheMaxEntries: envInt("OFFSET_CACHE_MAX_ENTRIES", 1000),
		HTTPTimeout:           time.Duration(envInt("HTTP_TIMEOUT_SECONDS", 60)) * time.Second,
		Auth:                  AuthConfig{JWTSecret: strings.TrimSpace(os.Getenv("JPROXY_JWT_SECRET")), TokenExpiresMinutes: envInt("JPROXY_TOKEN_EXPIRES_MINUTES", 60)},
	}
	loginEnabled, err := envBool("JPROXY_LOGIN_ENABLED", false)
	if err != nil {
		return Config{}, err
	}
	cfg.Auth.LoginEnabled = loginEnabled
	if cfg.Auth.TokenExpiresMinutes <= 0 {
		return Config{}, errors.New("JPROXY_TOKEN_EXPIRES_MINUTES must be positive")
	}
	if cfg.Auth.LoginEnabled && !validJWTSecret(cfg.Auth.JWTSecret) {
		return Config{}, ErrJWTSecretInvalid
	}
	radarrFormatEnabled, err := envBool("JPROXY_RADARR_FORMAT_ENABLED", false)
	if err != nil {
		return Config{}, err
	}
	sonarrFormatEnabled, err := envBool("JPROXY_SONARR_FORMAT_ENABLED", false)
	if err != nil {
		return Config{}, err
	}
	dbEnabled, err := envBool("JPROXY_DB_ENABLED", false)
	if err != nil {
		return Config{}, err
	}
	if dbEnabled {
		path := strings.TrimSpace(os.Getenv("JPROXY_DB_PATH"))
		if path == "" {
			return Config{}, fmt.Errorf("JPROXY_DB_PATH is required when JPROXY_DB_ENABLED is true: %w", ErrDatabasePathRequired)
		}
		cfg.Database = DatabaseConfig{Enabled: true, Path: path}
		cfg.RadarrFormatting.Enabled = radarrFormatEnabled
		cfg.SonarrFormatting.Enabled = sonarrFormatEnabled
		return cfg, nil
	}
	if radarrFormatEnabled {
		formatConfig, loadErr := loadRadarrFormatConfig()
		if loadErr != nil {
			return Config{}, loadErr
		}
		cfg.RadarrFormatting = RadarrFormattingConfig{Enabled: true, Config: formatConfig}
	}
	if sonarrFormatEnabled {
		formatConfig, loadErr := loadSonarrFormatConfig()
		if loadErr != nil {
			return Config{}, loadErr
		}
		cfg.SonarrFormatting = SonarrFormattingConfig{Enabled: true, Config: formatConfig}
	}
	return cfg, nil
}

func validJWTSecret(secret string) bool {
	if len(secret) < 32 {
		return false
	}
	if strings.Trim(secret, secret[:1]) == "" {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(secret)) {
	case "change-me", "changeme", "secret", "jwt-secret", "your-32-byte-secret-key-here":
		return false
	default:
		return true
	}
}

func loadRadarrFormatConfig() (format.Config, error) {
	formatText := strings.TrimSpace(os.Getenv("JPROXY_RADARR_FORMAT"))
	if formatText == "" {
		return format.Config{}, fmt.Errorf("JPROXY_RADARR_FORMAT is required when JPROXY_RADARR_FORMAT_ENABLED is true")
	}
	var rules []format.Rule
	if err := json.Unmarshal([]byte(env("JPROXY_RADARR_FORMAT_RULES", "[]")), &rules); err != nil {
		return format.Config{}, fmt.Errorf("parse JPROXY_RADARR_FORMAT_RULES: %w", err)
	}
	var titles []format.Title
	if err := json.Unmarshal([]byte(env("JPROXY_RADARR_TITLES", "[]")), &titles); err != nil {
		return format.Config{}, fmt.Errorf("parse JPROXY_RADARR_TITLES: %w", err)
	}
	cfg := format.Config{Format: formatText, CleanTitleRegex: os.Getenv("JPROXY_RADARR_TITLE_CLEAN_REGEX"), Rules: rules, Titles: titles}
	if err := format.ValidateConfig(cfg); err != nil {
		return format.Config{}, fmt.Errorf("validate Radarr formatting config: %w", err)
	}
	return cfg, nil
}

func loadSonarrFormatConfig() (format.SonarrConfig, error) {
	formatText := strings.TrimSpace(os.Getenv("JPROXY_SONARR_FORMAT"))
	if formatText == "" {
		return format.SonarrConfig{}, fmt.Errorf("JPROXY_SONARR_FORMAT is required when JPROXY_SONARR_FORMAT_ENABLED is true")
	}
	var rules []format.Rule
	if err := json.Unmarshal([]byte(env("JPROXY_SONARR_FORMAT_RULES", "[]")), &rules); err != nil {
		return format.SonarrConfig{}, fmt.Errorf("parse JPROXY_SONARR_FORMAT_RULES: %w", err)
	}
	var titles []format.SonarrTitle
	if err := json.Unmarshal([]byte(env("JPROXY_SONARR_TITLES", "[]")), &titles); err != nil {
		return format.SonarrConfig{}, fmt.Errorf("parse JPROXY_SONARR_TITLES: %w", err)
	}
	cfg := format.SonarrConfig{Format: formatText, CleanTitleRegex: os.Getenv("JPROXY_SONARR_TITLE_CLEAN_REGEX"), Rules: rules, Titles: titles}
	if err := format.ValidateSonarrConfig(cfg); err != nil {
		return format.SonarrConfig{}, fmt.Errorf("validate Sonarr formatting config: %w", err)
	}
	return cfg, nil
}

func env(key, def string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	return v
}

func envInt(key string, def int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func envBool(key string, def bool) (bool, error) {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def, nil
	}
	parsed, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("parse %s: %w", key, err)
	}
	return parsed, nil
}

func trimRightSlash(s string) string { return strings.TrimRight(s, "/") }
