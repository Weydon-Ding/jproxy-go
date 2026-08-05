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
		{"regex", "cleanTitleRegex", "["}, {"language", "sonarrLanguage1", "invalid_language"},
		{"duplicate_authors", "ruleSyncAuthors", "one,one"}, {"all_with_other", "ruleSyncAuthors", "ALL,one"},
		{"author_path", "ruleSyncAuthors", "../secret"},
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
	value, err = validateValue("ruleSyncAuthors", "one, two")
	if err != nil || value != "one,two" {
		t.Fatalf("authors=%q err=%v", value, err)
	}
}
