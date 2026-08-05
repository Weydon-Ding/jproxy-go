package system

import "testing"

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
	value, err := validateValue("transmissionUrl", "https://example.test/transmission/web/")
	if err != nil || value != "https://example.test/transmission/rpc" {
		t.Fatalf("value=%q err=%v", value, err)
	}
	value, err = validateValue("ruleSyncAuthors", "ALL, one/../two")
	if err != nil || value != "ALL, one/../two" {
		t.Fatalf("authors=%q err=%v", value, err)
	}
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
