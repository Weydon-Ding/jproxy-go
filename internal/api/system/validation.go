package system

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"jproxy-go/internal/transmissionconfig"
)

const maxValueLength = transmissionconfig.MaxValueLength

func validateValue(key, value string) (string, error) {
	if len(value) > maxValueLength || hasControl(value) {
		return "", fmt.Errorf("invalid config value")
	}
	switch key {
	case "transmissionUrl":
		return transmissionconfig.NormalizeEndpoint(value)
	case "sonarrUrl", "radarrUrl", "jackettUrl", "prowlarrUrl", "qbittorrentUrl", "tmdbUrl":
		return normalizeURL(value)
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

func normalizeURL(value string) (string, error) {
	if value == "" {
		return value, nil
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "http" && parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" {
		return "", fmt.Errorf("invalid URL")
	}
	value = strings.TrimRight(value, "/")
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
