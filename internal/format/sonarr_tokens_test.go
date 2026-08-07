package format

import "testing"

func TestFormatTokens_matchesJavaSonarrFormatting(t *testing.T) {
	// Given
	rules := []Rule{{Token: "season", Regex: `.*S(\d+).*`, Replacement: "S$1"}, {Token: "episode", Regex: `.*E(\d+).*`, Replacement: "E$1"}}

	// When
	got := FormatTokens("Show.S02E03.1080p", "{season}{episode}", rules)

	// Then
	if got != "S02E03" {
		t.Fatalf("FormatTokens() = %q", got)
	}
}

func TestFormatTokens_returnsOriginalTextWhenEpisodeIsUnresolved(t *testing.T) {
	// Given
	rules := []Rule{{Token: "season", Regex: `.*S(\d+).*`, Replacement: "S$1"}}

	// When
	got := FormatTokens("Show.S02.1080p", "{season}{episode}", rules)

	// Then
	if got != "Show.S02.1080p" {
		t.Fatalf("FormatTokens() = %q", got)
	}
}
