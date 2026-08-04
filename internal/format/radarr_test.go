package format

import (
	"reflect"
	"strings"
	"testing"
)

func status(value int) *int { return &value }

func TestCleanTitle_matchesJavaFormattingRules(t *testing.T) {
	// Given
	title := "The [A] (Test)+.Movie 2024"

	// When
	got := CleanTitle(title, `\b2024\b`)

	// Then
	if got != "/a/ test movie" {
		t.Fatalf("CleanTitle() = %q, want %q", got, "/a/ test movie")
	}
}

func TestCleanTitle_fallsBackWhenConfiguredRegexRemovesEverything(t *testing.T) {
	// Given
	title := "[A]"

	// When
	got := CleanTitle(title, `.+`)

	// Then
	if got != "/a/" {
		t.Fatalf("CleanTitle() = %q, want %q", got, "/a/")
	}
}

func TestTokenHelpers_replaceUnknownTokensAndApplyJavaStyleOffsets(t *testing.T) {
	// Given
	text := "{season} {unknown}"

	// When
	replaced := ReplaceToken("season", ExecuteOffset("S01E01", 1), text)
	got := RemoveAllTokens(replaced)

	// Then
	if got != "S2E2 " {
		t.Fatalf("token helpers = %q, want %q", got, "S2E2 ")
	}
}

func TestFormatRadarrXML_rewritesMultipleItemsWithDescriptionAndRules(t *testing.T) {
	// Given
	input := `<rss><channel><item><title>Movie.Name.2024.1080p</title><description>WEB-DL</description></item><item><title>Other Movie 2023 720p</title></item></channel></rss>`
	cfg := Config{
		Format: "{title} ({year}) [{resolution}] {unknown}",
		Rules: []Rule{
			{Token: "title", Regex: `^(.+?)(?:\.|\s)\d{4}.*$`, Replacement: "$1"},
			{Token: "year", Regex: `.*?(\d{4}).*`, Replacement: "$1"},
			{Token: "resolution", Regex: `.*?(\d{3,4}p).*`, Replacement: "$1"},
		},
	}

	// When
	got := RadarrXML(input, cfg)

	// Then
	for _, want := range []string{
		"<title>Movie.Name (2024) [1080p]</title>",
		"<title>Other Movie (2023) [720p]</title>",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("formatted XML missing %q: %s", want, got)
		}
	}
}

func TestFormatRadarrXML_appliesLowerPriorityRulesFirst(t *testing.T) {
	// Given
	input := `<rss><channel><item><title>Movie.2024</title></item></channel></rss>`
	cfg := Config{
		Format: "{title}",
		Rules: []Rule{
			{Token: "title", Regex: `^(.+?)\.\d{4}$`, Replacement: "late", Priority: 10},
			{Token: "title", Regex: `^(.+?)\.\d{4}$`, Replacement: "early", Priority: 1},
		},
	}

	// When
	got := RadarrXML(input, cfg)

	// Then
	if !strings.Contains(got, "<title>early</title>") {
		t.Fatalf("RadarrXML() = %s, want lower-priority rule to win", got)
	}
}

func TestFormatRadarrXML_preservesDeclarationOrderForEqualPriority(t *testing.T) {
	// Given
	input := `<rss><channel><item><title>Movie.2024</title></item></channel></rss>`
	cfg := Config{
		Format: "{title}",
		Rules: []Rule{
			{Token: "title", Regex: `^(.+?)\.\d{4}$`, Replacement: "first", Priority: 1},
			{Token: "title", Regex: `^(.+?)\.\d{4}$`, Replacement: "second", Priority: 1},
		},
	}

	// When
	got := RadarrXML(input, cfg)

	// Then
	if !strings.Contains(got, "<title>first</title>") {
		t.Fatalf("RadarrXML() = %s, want first declared equal-priority rule to win", got)
	}
}

func TestFormatRadarrXML_usesLegacyDeclarationOrderWhenPriorityIsOmitted(t *testing.T) {
	// Given
	input := `<rss><channel><item><title>Movie.2024</title></item></channel></rss>`
	cfg := Config{
		Format: "{title}",
		Rules: []Rule{
			{Token: "title", Regex: `^(.+?)\.\d{4}$`, Replacement: "first"},
			{Token: "title", Regex: `^(.+?)\.\d{4}$`, Replacement: "second"},
		},
	}

	// When
	got := RadarrXML(input, cfg)

	// Then
	if !strings.Contains(got, "<title>first</title>") {
		t.Fatalf("RadarrXML() = %s, want legacy declaration order", got)
	}
}

func TestFormatRadarrXML_excludesDisabledRulesFromEveryTokenMatch(t *testing.T) {
	// Given
	input := `<rss><channel><item><title>Movie.2024.1080p</title></item></channel></rss>`
	cfg := Config{
		Format: "{title} {year} {resolution}",
		Rules: []Rule{
			{Token: "title", Regex: `^(.+?)\.\d{4}.*$`, Replacement: "disabled-title", ValidStatus: status(0)},
			{Token: "title", Regex: `^(.+?)\.\d{4}.*$`, Replacement: "Movie", ValidStatus: status(1)},
			{Token: "year", Regex: `.*?(\d{4}).*`, Replacement: "disabled-year", ValidStatus: status(0)},
			{Token: "year", Regex: `.*?(\d{4}).*`, Replacement: "$1"},
			{Token: "resolution", Regex: `.*?(1080p).*`, Replacement: "disabled-resolution", ValidStatus: status(0)},
			{Token: "resolution", Regex: `.*?(1080p).*`, Replacement: "$1"},
		},
	}

	// When
	got := RadarrXML(input, cfg)

	// Then
	if !strings.Contains(got, "<title>Movie 2024 1080p</title>") || strings.Contains(got, "disabled-") {
		t.Fatalf("RadarrXML() = %s, want only enabled rules to participate", got)
	}
}

func TestRulesByToken_doesNotMutateCallerRulesWhenOrdering(t *testing.T) {
	// Given
	rules := []Rule{
		{Token: "title", Regex: "first", Priority: 10},
		{Token: "title", Regex: "second", Priority: 1},
	}
	want := append([]Rule(nil), rules...)

	// When
	got := rulesByToken(rules)

	// Then
	if !reflect.DeepEqual(rules, want) || got["title"][0].Regex != "second" {
		t.Fatalf("rules=%#v ordered=%#v, want original unchanged and sorted copy", rules, got["title"])
	}
}

func TestValidateConfig_rejectsInvalidRuleValidStatus(t *testing.T) {
	// Given
	cfg := Config{Format: "{title}", Rules: []Rule{{Token: "title", Regex: `.*`, ValidStatus: status(2)}}}

	// When
	err := ValidateConfig(cfg)

	// Then
	if err == nil {
		t.Fatal("ValidateConfig() error = nil, want invalid validStatus error")
	}
}

func TestFormatRadarrXML_matchesStaticTitleForCleanTitleRule(t *testing.T) {
	// Given
	input := `<rss><channel><item><title>The Matrix 1999 2160p</title></item></channel></rss>`
	cfg := Config{
		Format: "{title} {year} {quality}",
		Rules: []Rule{
			{Token: "title", Regex: `^{cleanTitle} \d{4}.*$`},
			{Token: "year", Regex: `.*?(\d{4}).*`, Replacement: "$1"},
			{Token: "quality", Regex: `.*?(2160p).*`, Replacement: "$1"},
		},
		Titles: []Title{{MainTitle: "Matrix", Title: "The Matrix", Year: 1999}},
	}

	// When
	got := RadarrXML(input, cfg)

	// Then
	if !strings.Contains(got, "<title>Matrix 1999 2160p</title>") {
		t.Fatalf("formatted XML = %s", got)
	}
}

func TestFormatRadarrXML_skipsCleanTitleMatchWhenYearDiffers(t *testing.T) {
	// Given
	input := `<rss><channel><item><title>The Matrix 2024 2160p</title></item></channel></rss>`
	cfg := Config{
		Format: "{title} {year}",
		Rules: []Rule{
			{Token: "title", Regex: `^{cleanTitle} \d{4}.*$`},
			{Token: "year", Regex: `.*?(\d{4}).*`, Replacement: "$1"},
		},
		Titles: []Title{{MainTitle: "Matrix", Title: "The Matrix", Year: 1999}},
	}

	// When
	got := RadarrXML(input, cfg)

	// Then
	if got != input {
		t.Fatalf("RadarrXML() = %q, want original %q", got, input)
	}
}

func TestFormatRadarrXML_skipsShortEnglishCleanTitleInsideLongerTitle(t *testing.T) {
	// Given
	input := `<rss><channel><item><title>It Follows 2014 1080p</title></item></channel></rss>`
	cfg := Config{
		Format: "{title} {year}",
		Rules: []Rule{
			{Token: "title", Regex: `^{cleanTitle}.*$`},
			{Token: "year", Regex: `.*?(\d{4}).*`, Replacement: "$1"},
		},
		Titles: []Title{{MainTitle: "It", Title: "It", Year: 2014}},
	}

	// When
	got := RadarrXML(input, cfg)

	// Then
	if got != input {
		t.Fatalf("RadarrXML() = %q, want original %q", got, input)
	}
}

func TestFormatRadarrXML_returnsOriginalWhenStaticCleanTitleContainsRegexMeta(t *testing.T) {
	// Given
	input := `<rss><channel><item><title>Movie 2024</title></item></channel></rss>`
	cfg := Config{
		Format: "{title}",
		Rules:  []Rule{{Token: "title", Regex: `^{cleanTitle} \d{4}$`}},
		Titles: []Title{{MainTitle: "Movie", CleanTitle: "(", Year: 2024}},
	}

	// When
	got := RadarrXML(input, cfg)

	// Then
	if got != input {
		t.Fatalf("RadarrXML() = %q, want original %q", got, input)
	}
}

func TestFormatRadarrXML_returnsOriginalForUnsupportedInputs(t *testing.T) {
	tests := []struct {
		name  string
		input string
		cfg   Config
	}{
		{name: "blank", input: "", cfg: Config{Format: "{title}"}},
		{name: "no item", input: `<rss><channel></channel></rss>`, cfg: Config{Format: "{title}"}},
		{name: "blank format", input: `<rss><channel><item><title>Movie</title></item></channel></rss>`, cfg: Config{}},
		{name: "missing title token", input: `<rss><channel><item><title>Movie</title></item></channel></rss>`, cfg: Config{Format: "{year}"}},
		{name: "invalid XML", input: `<rss><channel>`, cfg: Config{Format: "{title}"}},
		{name: "unmatched title", input: `<rss><channel><item><title>Movie</title></item></channel></rss>`, cfg: Config{Format: "{title} {year}", Rules: []Rule{{Token: "year", Regex: `.*`, Replacement: "2024"}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When
			got := RadarrXML(tt.input, tt.cfg)

			// Then
			if got != tt.input {
				t.Fatalf("RadarrXML() = %q, want original %q", got, tt.input)
			}
		})
	}
}

func TestFormatRadarrXML_rewritesOnlyDirectItemTitleAndDescription(t *testing.T) {
	// Given
	input := `<rss><channel><item><title>Movie.2024.1080p</title><details><title>Nested.2024.720p</title><description>nested</description></details><description>WEB-DL</description></item></channel></rss>`
	cfg := Config{
		Format: "{title} [{source}]",
		Rules: []Rule{
			{Token: "title", Regex: `^(.+?)\.\d{4}.*$`, Replacement: "$1"},
			{Token: "source", Regex: `.* / (WEB-DL).*`, Replacement: "$1"},
		},
	}

	// When
	got := RadarrXML(input, cfg)

	// Then
	if !strings.Contains(got, `<title>Movie [WEB-DL]</title>`) || !strings.Contains(got, `<details><title>Nested.2024.720p</title><description>nested</description></details>`) {
		t.Fatalf("RadarrXML() = %s", got)
	}
}
