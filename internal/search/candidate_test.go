package search_test

import (
	"reflect"
	"slices"
	"testing"

	"jproxy-go/internal/search"
)

func TestSonarrCandidates_areDeterministicAndDeduplicated_whenRowsAreJoinedWithAliases(t *testing.T) {
	// Given
	rows := []search.SonarrRow{
		{MainTitle: "The Example", Title: "Example Alias", SeasonNumber: -1, Monitored: true},
		{MainTitle: "The Example", Title: "The Example", CleanTitle: "the example", SeasonNumber: 0, Monitored: true},
		{MainTitle: "The Example", Title: "Example Alias", SeasonNumber: -1, Monitored: true},
		{MainTitle: "Other", Title: "Other", CleanTitle: "other", SeasonNumber: 3},
		{MainTitle: "The Example", Title: "", SeasonNumber: -1, Monitored: true},
	}
	reversed := slices.Clone(rows)
	slices.Reverse(reversed)
	want := []search.Candidate{
		{Query: "Example Alias", MainTitle: "The Example", SeasonNumber: -1},
		{Query: "The Example", MainTitle: "The Example", SeasonNumber: 0},
		{Query: "Other", MainTitle: "Other", SeasonNumber: 3},
	}

	// When
	forward := search.SonarrCandidates(rows)
	backward := search.SonarrCandidates(reversed)

	// Then
	if !reflect.DeepEqual(forward, want) || !reflect.DeepEqual(backward, want) {
		t.Fatalf("forward=%+v backward=%+v want=%+v", forward, backward, want)
	}
}

func TestSonarrCandidates_preserveSeasonMetadata_whenSeasonIsMinusOneZeroOrPositive(t *testing.T) {
	// Given
	rows := []search.SonarrRow{
		{MainTitle: "Show", Title: "Show All", SeasonNumber: -1},
		{MainTitle: "Show", Title: "Show Specials", SeasonNumber: 0},
		{MainTitle: "Show", Title: "Show Second", SeasonNumber: 2},
	}

	// When
	got := search.SonarrCandidates(rows)

	// Then
	want := []search.Candidate{
		{Query: "Show Specials", MainTitle: "Show", SeasonNumber: 0},
		{Query: "Show Second", MainTitle: "Show", SeasonNumber: 2},
		{Query: "Show All", MainTitle: "Show", SeasonNumber: -1},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got=%+v want=%+v", got, want)
	}
}

func TestRadarrCandidates_areDeterministicAndAddOnlyMissingYearVariants(t *testing.T) {
	// Given
	rows := []search.RadarrRow{
		{TMDBID: 20, SNO: 1, MainTitle: "Movie", Title: "Movie", Year: 2024, Monitored: true},
		{TMDBID: 20, SNO: 0, MainTitle: "Movie", Title: "Movie 2024", Year: 2024, Monitored: true},
		{TMDBID: 20, SNO: 2, MainTitle: "Movie", Title: "Movie", Year: 2024, Monitored: true},
		{TMDBID: 30, SNO: 0, MainTitle: "Other", Title: "", Year: 2020},
	}
	reversed := slices.Clone(rows)
	slices.Reverse(reversed)
	want := []search.Candidate{
		{Query: "Movie 2024", MainTitle: "Movie", Year: 2024},
		{Query: "Movie", MainTitle: "Movie", Year: 2024},
	}

	// When
	forward := search.RadarrCandidates(rows)
	backward := search.RadarrCandidates(reversed)

	// Then
	if !reflect.DeepEqual(forward, want) || !reflect.DeepEqual(backward, want) {
		t.Fatalf("forward=%+v backward=%+v want=%+v", forward, backward, want)
	}
}

func TestCandidateBuilders_doNotMutateInputs(t *testing.T) {
	// Given
	sonarr := []search.SonarrRow{{MainTitle: "Show", Title: "Alias", CleanTitle: "alias", SeasonNumber: 1, Monitored: true}}
	radarr := []search.RadarrRow{{TMDBID: 1, SNO: 1, MainTitle: "Movie", Title: "Movie", CleanTitle: "movie", Year: 2024, Monitored: true}}
	sonarrBefore := slices.Clone(sonarr)
	radarrBefore := slices.Clone(radarr)

	// When
	_ = search.SonarrCandidates(sonarr)
	_ = search.RadarrCandidates(radarr)

	// Then
	if !reflect.DeepEqual(sonarr, sonarrBefore) || !reflect.DeepEqual(radarr, radarrBefore) {
		t.Fatalf("sonarr=%+v radarr=%+v", sonarr, radarr)
	}
}
