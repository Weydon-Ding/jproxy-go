package format

import "testing"

func TestExampleProjection_matchesItemFormatting_whenTitleRulesApply(t *testing.T) {
	sonarr := SonarrConfig{Format: "{title} {episode}", Rules: []Rule{{Token: "title", Regex: "^(.*)$", Replacement: "$1"}}}
	if got := SonarrExample("Series", sonarr); got != "Series" {
		t.Fatalf("sonarr=%q", got)
	}
	radarr := Config{Format: "{title} {year}", Rules: []Rule{{Token: "title", Regex: "{cleanTitle} (\\d{4})", Replacement: ""}, {Token: "year", Regex: ".*(\\d{4}).*", Replacement: "$1"}}, Titles: []Title{{MainTitle: "Movie", Title: "Movie", Year: 2024}}}
	if got := RadarrExample("Movie 2024", radarr); got != "Movie 2024" {
		t.Fatalf("radarr=%q", got)
	}
}
