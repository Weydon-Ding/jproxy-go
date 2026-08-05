package system

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

const maxValueLength = 16 * 1024

func validateValue(key, value string) (string, error) {
	if len(value) > maxValueLength || strings.ContainsAny(value, "\r\n\x00") {
		return "", fmt.Errorf("invalid config value")
	}
	switch key {
	case "sonarrUrl", "radarrUrl", "jackettUrl", "prowlarrUrl", "qbittorrentUrl", "transmissionUrl", "tmdbUrl":
		return normalizeURL(value, key == "transmissionUrl")
	case "sonarrIndexerFormat", "radarrIndexerFormat":
		if !strings.Contains(value, "{title}") {
			return "", fmt.Errorf("format requires title token")
		}
	case "cleanTitleRegex":
		if _, err := regexp.Compile(value); err != nil {
			return "", fmt.Errorf("invalid regex")
		}
	case "sonarrLanguage1", "sonarrLanguage2":
		if !regexp.MustCompile(`^[A-Za-z]{2,3}(-[A-Za-z0-9]{2,8})*$`).MatchString(value) {
			return "", fmt.Errorf("invalid language")
		}
	case "ruleSyncAuthors":
		return normalizeAuthors(value)
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

func normalizeAuthors(value string) (string, error) {
	items := strings.Split(value, ",")
	seen := make(map[string]bool, len(items))
	for index := range items {
		items[index] = strings.TrimSpace(items[index])
		if items[index] == "" || seen[items[index]] || len(items[index]) > 128 {
			return "", fmt.Errorf("invalid authors")
		}
		seen[items[index]] = true
	}
	return strings.Join(items, ","), nil
}
