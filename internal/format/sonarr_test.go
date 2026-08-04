package format

import (
	"strings"
	"testing"
)

func TestSonarrXML_formatsCleanTitleSeasonAndDescriptionTokens(t *testing.T) {
	// Given
	input := "<rss><channel><item><title>The Matrix\nseason 2 1080p</title><description>WEB-DL</description></item></channel></rss>"
	season := 2
	cfg := SonarrConfig{
		Format: "{title} {season} [{quality}] {source}",
		Rules: []Rule{
			{Token: "title", Regex: `^{cleanTitle} season \d+.*$`},
			{Token: "quality", Regex: `.*?(1080p).*`, Replacement: "$1"},
			{Token: "source", Regex: `.* / (WEB-DL)$`, Replacement: "$1"},
		},
		Titles: []SonarrTitle{{MainTitle: "Matrix", Title: "The Matrix", SeasonNumber: &season}},
	}

	// When
	got := SonarrXML(input, cfg)

	// Then
	if !strings.Contains(got, `<title>Matrix S2 [1080p] WEB-DL</title>`) {
		t.Fatalf("SonarrXML() = %s", got)
	}
}

func TestSonarrXML_keepsCompleteTextWhenEpisodeTokenIsUnresolved(t *testing.T) {
	// Given
	input := `<rss><channel><item><title>Show.S02E03.1080p</title><description>WEB-DL</description></item></channel></rss>`
	cfg := SonarrConfig{
		Format: "{title} {episode}",
		Rules:  []Rule{{Token: "title", Regex: `^(.+?)\.S\d+E\d+.*$`, Replacement: "$1"}},
	}

	// When
	got := SonarrXML(input, cfg)

	// Then
	if !strings.Contains(got, `<title>Show.S02E03.1080p / WEB-DL</title>`) {
		t.Fatalf("SonarrXML() = %s", got)
	}
}

func TestSonarrXML_removesSeasonTokenForSentinelSeasonNumbers(t *testing.T) {
	// Given
	tests := []struct {
		name         string
		seasonNumber int
	}{
		{name: "main series season", seasonNumber: 1},
		{name: "unknown season", seasonNumber: -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := `<rss><channel><item><title>Show season 1 episode 2</title></item></channel></rss>`
			cfg := SonarrConfig{
				Format: "{title} {season}{episode}",
				Rules: []Rule{
					{Token: "title", Regex: `^{cleanTitle} season \d+ episode \d+$`},
					{Token: "episode", Regex: `.*episode (\d+).*`, Replacement: "E$1"},
				},
				Titles: []SonarrTitle{{MainTitle: "Show", Title: "Show", SeasonNumber: &tt.seasonNumber}},
			}

			// When
			got := SonarrXML(input, cfg)

			// Then
			if !strings.Contains(got, `<title>Show E2</title>`) {
				t.Fatalf("SonarrXML() = %s, want sentinel season token removed", got)
			}
		})
	}
}

func TestSonarrXML_usesEnabledRulesInStablePriorityOrder(t *testing.T) {
	// Given
	input := `<rss><channel><item><title>Show.S02E03.1080p</title></item></channel></rss>`
	disabled := 0
	cfg := SonarrConfig{
		Format: "{title} {quality} {group}",
		Rules: []Rule{
			{Token: "title", Regex: `^(.+?)\.S\d+E\d+.*$`, Replacement: "disabled", Priority: 0, ValidStatus: &disabled},
			{Token: "title", Regex: `^(.+?)\.S\d+E\d+.*$`, Replacement: "late", Priority: 10},
			{Token: "title", Regex: `^(.+?)\.S\d+E\d+.*$`, Replacement: "early", Priority: 1},
			{Token: "quality", Regex: `.*?(1080p).*`, Replacement: "first", Priority: 5},
			{Token: "quality", Regex: `.*?(1080p).*`, Replacement: "second", Priority: 5},
			{Token: "group", Regex: `.*`, Replacement: "disabled-group", ValidStatus: &disabled},
		},
	}

	// When
	got := SonarrXML(input, cfg)

	// Then
	if !strings.Contains(got, `<title>early first</title>`) || strings.Contains(got, "disabled") || strings.Contains(got, "second") {
		t.Fatalf("SonarrXML() = %s, want enabled rules by stable priority order", got)
	}
}

func TestSonarrXML_allowsCleanTitleNextToSeasonAndRejectsOtherEnglishWords(t *testing.T) {
	season := 2
	cfg := SonarrConfig{
		Format: "{title} {season}",
		Rules:  []Rule{{Token: "title", Regex: `^{cleanTitle}.*$`}},
		Titles: []SonarrTitle{{MainTitle: "It", Title: "It", SeasonNumber: &season}},
	}
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "season is an allowed suffix", input: `<rss><channel><item><title>It season 2</title></item></channel></rss>`, want: `<title>It S2</title>`},
		{name: "word suffix is rejected", input: `<rss><channel><item><title>It Follows season 2</title></item></channel></rss>`, want: `<title>It Follows season 2</title>`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When
			got := SonarrXML(tt.input, cfg)

			// Then
			if !strings.Contains(got, tt.want) {
				t.Fatalf("SonarrXML() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestValidateSonarrConfig_rejectsInvalidStaticTitlesAndRules(t *testing.T) {
	season := 1
	tests := []struct {
		name string
		cfg  SonarrConfig
	}{
		{name: "missing title token", cfg: SonarrConfig{Format: "{season}"}},
		{name: "invalid clean title regex", cfg: SonarrConfig{Format: "{title}", CleanTitleRegex: "["}},
		{name: "invalid rule status", cfg: SonarrConfig{Format: "{title}", Rules: []Rule{{Token: "title", Regex: ".*", ValidStatus: status(2)}}}},
		{name: "blank main title", cfg: SonarrConfig{Format: "{title}", Titles: []SonarrTitle{{Title: "Show", SeasonNumber: &season}}}},
		{name: "missing alternate title", cfg: SonarrConfig{Format: "{title}", Titles: []SonarrTitle{{MainTitle: "Show", SeasonNumber: &season}}}},
		{name: "missing season", cfg: SonarrConfig{Format: "{title}", Titles: []SonarrTitle{{MainTitle: "Show", Title: "Show"}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When
			err := ValidateSonarrConfig(tt.cfg)

			// Then
			if err == nil {
				t.Fatal("ValidateSonarrConfig() error = nil")
			}
		})
	}
}

func TestSonarrXML_returnsOriginalForUnsupportedInputs(t *testing.T) {
	// Given
	input := `<rss><channel><item><title>Show</title></item></channel></rss>`
	noTitleRules := SonarrConfig{Format: "{title}", Rules: []Rule{{Token: "season", Regex: `.*`, Replacement: "S1"}}}

	// When
	got := SonarrXML(input, noTitleRules)

	// Then
	if got != input {
		t.Fatalf("SonarrXML() = %q, want %q", got, input)
	}
}

func TestSonarrXML_rewritesOnlyDirectChannelItemTitles(t *testing.T) {
	// Given
	input := `<rss><channel><item><title>Show.S02E03</title><details><title>Nested.S02E03</title></details></item></channel><item><title>Outside.S02E03</title></item></rss>`
	cfg := SonarrConfig{Format: "{title}", Rules: []Rule{{Token: "title", Regex: `^(.+?)\.S\d+E\d+$`, Replacement: "$1"}}}

	// When
	got := SonarrXML(input, cfg)

	// Then
	if !strings.Contains(got, `<channel><item><title>Show</title><details><title>Nested.S02E03</title></details></item></channel><item><title>Outside.S02E03</title></item>`) {
		t.Fatalf("SonarrXML() = %s", got)
	}
}
