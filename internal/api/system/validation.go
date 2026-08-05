package system

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

const maxValueLength = 16 * 1024

func validateValue(key, value string) (string, error) {
	if len(value) > maxValueLength || hasControl(value) {
		return "", fmt.Errorf("invalid config value")
	}
	switch key {
	case "sonarrUrl", "radarrUrl", "jackettUrl", "prowlarrUrl", "qbittorrentUrl", "transmissionUrl", "tmdbUrl":
		return normalizeURL(value, key == "transmissionUrl")
	case "sonarrIndexerFormat", "radarrIndexerFormat":
		if key == "sonarrIndexerFormat" && (!strings.Contains(value, "{title}") || !strings.Contains(value, "{season}") || !strings.Contains(value, "{episode}")) {
			return "", fmt.Errorf("invalid Sonarr format")
		}
		if key == "radarrIndexerFormat" && (!strings.Contains(value, "{title}") || !strings.Contains(value, "{year}")) {
			return "", fmt.Errorf("invalid Radarr format")
		}
	case "cleanTitleRegex":
		if _, err := regexp.Compile(value); err != nil {
			return "", fmt.Errorf("invalid regex")
		}
	}
	return value, nil
}

func normalizeURL(value string, transmission bool) (string, error) {
	if value == "" {
		return value, nil
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "http" && parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" {
		return "", fmt.Errorf("invalid URL")
	}
	value = strings.TrimRight(value, "/")
	if transmission {
		value = strings.TrimSuffix(value, "/transmission/web")
		if !strings.HasSuffix(value, "/transmission/rpc") {
			value += "/transmission/rpc"
		}
	}
	return value, nil
}

func hasControl(value string) bool {
	for _, character := range value {
		if character < 0x20 || character == 0x7f {
			return true
		}
	}
	return false
}
