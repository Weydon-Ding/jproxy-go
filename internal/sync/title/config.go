package titlesync

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"unicode"

	"jproxy-go/internal/store/sqlite"
)

var errInvalidConfig = errors.New("invalid title sync configuration")

type providerConfig struct {
	baseURL *url.URL
	apiKey  string
	cleanRE string
}

type configSource interface {
	loadSonarr(context.Context) (providerConfig, error)
	loadRadarr(context.Context) (providerConfig, error)
}

type ConfigSource struct{ repository sqlite.SystemConfigRepository }

func NewConfigSource(repository sqlite.SystemConfigRepository) *ConfigSource {
	return &ConfigSource{repository: repository}
}

func (s *ConfigSource) loadSonarr(ctx context.Context) (providerConfig, error) {
	return s.load(ctx, "sonarr", "sonarrUrl", "sonarrApikey")
}

func (s *ConfigSource) loadRadarr(ctx context.Context) (providerConfig, error) {
	return s.load(ctx, "radarr", "radarrUrl", "radarrApikey")
}

func (s *ConfigSource) load(ctx context.Context, provider, urlKey, keyKey string) (providerConfig, error) {
	rows, err := s.repository.List(ctx)
	if err != nil {
		return providerConfig{}, fmt.Errorf("%s config list: %w", provider, err)
	}
	values := map[string]string{}
	for _, row := range rows {
		if row.ValidStatus != sqlite.Valid || row.Value == nil {
			continue
		}
		if row.Key != urlKey && row.Key != keyKey && row.Key != "cleanTitleRegex" {
			continue
		}
		if _, exists := values[row.Key]; exists {
			return providerConfig{}, fmt.Errorf("%s config duplicate %w", provider, errInvalidConfig)
		}
		values[row.Key] = *row.Value
	}
	if _, ok := values[urlKey]; !ok {
		return providerConfig{}, fmt.Errorf("%s config url: %w", provider, errInvalidConfig)
	}
	if _, ok := values[keyKey]; !ok {
		return providerConfig{}, fmt.Errorf("%s config api key: %w", provider, errInvalidConfig)
	}
	if _, ok := values["cleanTitleRegex"]; !ok {
		return providerConfig{}, fmt.Errorf("%s config clean title regex: %w", provider, errInvalidConfig)
	}
	parsed, err := validURL(values[urlKey])
	if err != nil {
		return providerConfig{}, fmt.Errorf("%s config url: %w", provider, errInvalidConfig)
	}
	if values[keyKey] == "" || containsControl(values[keyKey]) {
		return providerConfig{}, fmt.Errorf("%s config api key: %w", provider, errInvalidConfig)
	}
	if _, err := regexp.Compile(values["cleanTitleRegex"]); err != nil {
		return providerConfig{}, fmt.Errorf("%s config clean title regex: %w", provider, errInvalidConfig)
	}
	return providerConfig{baseURL: parsed, apiKey: values[keyKey], cleanRE: values["cleanTitleRegex"]}, nil
}

func validURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "http" && parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || containsControl(raw) {
		return nil, errInvalidConfig
	}
	return parsed, nil
}

func containsControl(value string) bool {
	return strings.IndexFunc(value, unicode.IsControl) >= 0
}
