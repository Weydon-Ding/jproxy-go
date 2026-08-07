package system

import (
	"strings"
	"testing"

	"jproxy-go/internal/store/sqlite"
)

func TestValidateRows_acceptsAllFixedIDsAndKeys(t *testing.T) {
	rows := append([]Config(nil), configs...)
	if len(rows) != 20 {
		t.Fatalf("fixed rows=%d", len(rows))
	}
}

func TestValidateValue_rejectsBoundaryValues(t *testing.T) {
	cases := []struct{ name, key, value string }{
		{"url_userinfo", "sonarrUrl", "http://user@example.test"}, {"url_fragment", "sonarrUrl", "http://example.test/#secret"},
		{"url_control", "sonarrUrl", "http://example.test/\x01"}, {"format_token", "sonarrIndexerFormat", "plain"},
		{"regex", "cleanTitleRegex", "["},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if _, err := validateValue(testCase.key, testCase.value); err == nil {
				t.Fatal("expected error")
			}
		})
	}
	for _, key := range []string{"sonarrUrl", "radarrUrl", "jackettUrl", "prowlarrUrl", "qbittorrentUrl", "tmdbUrl"} {
		for _, value := range []string{"http://example.test/?url-query-canary", "http://example.test/?"} {
			t.Run(key+value, func(t *testing.T) {
				// Given
				managementURL := value

				// When
				_, err := validateValue(key, managementURL)

				// Then
				if err == nil || strings.Contains(err.Error(), "url-query-canary") {
					t.Fatalf("validateValue(%q, %q) error = %v", key, managementURL, err)
				}
			})
		}
	}
	value, err := validateValue("transmissionUrl", "https://example.test/transmission/web/")
	if err != nil || value != "https://example.test/transmission/rpc" {
		t.Fatalf("value=%q err=%v", value, err)
	}
	value, err = validateValue("ruleSyncAuthors", "ALL, one/../two")
	if err != nil || value != "ALL, one/../two" {
		t.Fatalf("authors=%q err=%v", value, err)
	}
}

func TestValidateValue_normalizesOnlyTransmissionEndpoints(t *testing.T) {
	for _, test := range []struct {
		name  string
		key   string
		value string
		want  string
	}{
		{name: "Transmission base URL", key: "transmissionUrl", value: "https://transmission.test", want: "https://transmission.test/transmission/rpc"},
		{name: "Transmission web URL", key: "transmissionUrl", value: "https://transmission.test/transmission/web/", want: "https://transmission.test/transmission/rpc"},
		{name: "Transmission RPC URL", key: "transmissionUrl", value: "https://transmission.test/transmission/rpc/", want: "https://transmission.test/transmission/rpc"},
		{name: "Transmission prefix URL", key: "transmissionUrl", value: "https://transmission.test/prefix", want: "https://transmission.test/prefix/transmission/rpc"},
		{name: "generic URL", key: "jackettUrl", value: "https://jackett.test/prefix", want: "https://jackett.test/prefix"},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Given
			key, value := test.key, test.value

			// When
			got, err := validateValue(key, value)

			// Then
			if err != nil || got != test.want {
				t.Fatalf("validateValue(%q, %q) = %q, %v; want %q, nil", key, value, got, err, test.want)
			}
		})
	}
}

func TestValidateValue_rejectsUnsafeTransmissionEndpointsWithoutLeakingInput(t *testing.T) {
	for _, value := range []string{
		"https://transmission.test/?token=endpoint-canary",
		"https://transmission.test/?",
		"https://user:password@transmission.test",
		"https://transmission.test/#endpoint-canary",
		"https://transmission.test/control\npath",
	} {
		t.Run("rejects unsafe endpoint", func(t *testing.T) {
			// Given
			raw := value

			// When
			_, err := validateValue("transmissionUrl", raw)

			// Then
			if err == nil || strings.Contains(err.Error(), raw) || strings.Contains(err.Error(), "endpoint-canary") || strings.Contains(err.Error(), "password") {
				t.Fatalf("validateValue() error = %v", err)
			}
		})
	}
}

func TestValidateRows_validatesTransmissionCredentialPairsAfterNormalization(t *testing.T) {
	for _, test := range []struct {
		name     string
		username string
		password string
		wantErr  bool
	}{
		{name: "anonymous", username: "", password: ""},
		{name: "complete credentials", username: "transmission-user", password: "transmission-password"},
		{name: "username only", username: "transmission-user", password: "", wantErr: true},
		{name: "password only", username: "", password: "transmission-password", wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Given
			rows := systemConfigRows(t, "https://transmission.test/transmission/web/", test.username, test.password)

			// When
			err := validateRows(rows)

			// Then
			if (err != nil) != test.wantErr {
				t.Fatalf("validateRows() error = %v, wantErr %t", err, test.wantErr)
			}
			if err == nil && configValue(rows, "transmissionUrl") != "https://transmission.test/transmission/rpc" {
				t.Fatalf("Transmission URL = %q", configValue(rows, "transmissionUrl"))
			}
			if err != nil && ((test.username != "" && strings.Contains(err.Error(), test.username)) || (test.password != "" && strings.Contains(err.Error(), test.password))) {
				t.Fatalf("validateRows() error leaked credential: %v", err)
			}
		})
	}
}

func systemConfigRows(t *testing.T, transmissionURL, username, password string) []sqlite.SystemConfig {
	t.Helper()
	rows := make([]sqlite.SystemConfig, 0, len(configs))
	for _, config := range configs {
		value := config.Value
		switch config.Key {
		case "transmissionUrl":
			value = transmissionURL
		case "transmissionUsername":
			value = username
		case "transmissionPassword":
			value = password
		}
		rows = append(rows, sqlite.SystemConfig{ID: sqlite.SystemConfigID(config.ID), Key: config.Key, Value: &value})
	}
	return rows
}

func TestValidateValue_matchesJavaFormatterTokensAndCompatibility(t *testing.T) {
	for _, testCase := range []struct{ key, value string }{
		{"sonarrIndexerFormat", "{title} {season}"}, {"sonarrIndexerFormat", "{title} {episode}"},
		{"radarrIndexerFormat", "{title}"}, {"radarrIndexerFormat", "{year}"},
	} {
		if _, err := validateValue(testCase.key, testCase.value); err == nil {
			t.Fatalf("accepted invalid %s=%q", testCase.key, testCase.value)
		}
	}
	for _, testCase := range []struct{ key, value string }{
		{"sonarrIndexerFormat", "{title} {season} {episode}"}, {"radarrIndexerFormat", "{title} {year}"},
		{"sonarrLanguage1", "legacy_language"}, {"ruleSyncAuthors", "ALL,one,one/path"},
	} {
		if _, err := validateValue(testCase.key, testCase.value); err != nil {
			t.Fatalf("rejected Java-compatible %s=%q: %v", testCase.key, testCase.value, err)
		}
	}
	if _, err := validateValue("ruleSyncAuthors", "bad\x01"); err == nil {
		t.Fatal("accepted control character")
	}
	if _, err := validateValue("sonarrLanguage1", string(make([]byte, maxValueLength+1))); err == nil {
		t.Fatal("accepted oversized value")
	}
}
