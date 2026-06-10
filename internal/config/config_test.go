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
	t.Setenv("CACHE_EXPIRES", "")
	t.Setenv("HTTP_TIMEOUT_SECONDS", "")

	cfg := LoadConfig()
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
	if cfg.OffsetCacheTTL != 4320*time.Minute {
		t.Fatalf("OffsetCacheTTL = %s, want 4320m", cfg.OffsetCacheTTL)
	}
	if cfg.HTTPTimeout != 60*time.Second {
		t.Fatalf("HTTPTimeout = %s, want 60s", cfg.HTTPTimeout)
	}
}

func TestLoadConfigEnvironmentOverridesAndTrimsURLs(t *testing.T) {
	t.Setenv("ADDR", " 127.0.0.1:9000 ")
	t.Setenv("JACKETT_URL", " http://jackett.local:9117/// ")
	t.Setenv("PROWLARR_URL", " http://prowlarr.local:9696/ ")
	t.Setenv("MIN_COUNT", "9")
	t.Setenv("INDEXER_RESULT_CACHE_EXPIRES", "2")
	t.Setenv("CACHE_EXPIRES", "3")
	t.Setenv("HTTP_TIMEOUT_SECONDS", "4")

	cfg := LoadConfig()
	if cfg.Addr != "127.0.0.1:9000" {
		t.Fatalf("Addr = %q", cfg.Addr)
	}
	if cfg.JackettURL != "http://jackett.local:9117" {
		t.Fatalf("JackettURL = %q", cfg.JackettURL)
	}
	if cfg.ProwlarrURL != "http://prowlarr.local:9696" {
		t.Fatalf("ProwlarrURL = %q", cfg.ProwlarrURL)
	}
	if cfg.MinCount != 9 || cfg.IndexerResultCacheTTL != 2*time.Minute || cfg.OffsetCacheTTL != 3*time.Minute || cfg.HTTPTimeout != 4*time.Second {
		t.Fatalf("unexpected config overrides: %+v", cfg)
	}
}

func TestEnvIntFallsBackOnInvalidValue(t *testing.T) {
	t.Setenv("MIN_COUNT", "not-a-number")
	if got := LoadConfig().MinCount; got != 6 {
		t.Fatalf("MinCount with invalid env = %d, want default 6", got)
	}
}
