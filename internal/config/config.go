package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"jproxy-go/internal/format"
)

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
	RadarrFormatting      RadarrFormattingConfig
}

type RadarrFormattingConfig struct {
	Enabled bool
	Config  format.Config
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
	}
	formatEnabled, err := envBool("JPROXY_RADARR_FORMAT_ENABLED", false)
	if err != nil {
		return Config{}, err
	}
	if !formatEnabled {
		return cfg, nil
	}
	formatConfig, err := loadRadarrFormatConfig()
	if err != nil {
		return Config{}, err
	}
	cfg.RadarrFormatting = RadarrFormattingConfig{Enabled: true, Config: formatConfig}
	return cfg, nil
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
