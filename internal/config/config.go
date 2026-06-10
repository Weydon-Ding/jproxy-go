package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Addr                    string
	JackettURL              string
	ProwlarrURL             string
	MinCount                int
	IndexerResultCacheTTL   time.Duration
	OffsetCacheTTL          time.Duration
	HTTPTimeout             time.Duration
}

func LoadConfig() Config {
	return Config{
		Addr:                  env("ADDR", ":8117"),
		JackettURL:            trimRightSlash(env("JACKETT_URL", "http://127.0.0.1:9117")),
		ProwlarrURL:           trimRightSlash(env("PROWLARR_URL", "http://127.0.0.1:9696")),
		MinCount:              envInt("MIN_COUNT", 6),
		IndexerResultCacheTTL: time.Duration(envInt("INDEXER_RESULT_CACHE_EXPIRES", 15)) * time.Minute,
		OffsetCacheTTL:        time.Duration(envInt("CACHE_EXPIRES", 4320)) * time.Minute,
		HTTPTimeout:           time.Duration(envInt("HTTP_TIMEOUT_SECONDS", 60)) * time.Second,
	}
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

func trimRightSlash(s string) string { return strings.TrimRight(s, "/") }
