package proxy

import (
	"reflect"
	"testing"
)

func TestSearchTitlesForRadarrRemovesTrailingYear(t *testing.T) {
	got := searchTitles("radarr", "Movie Title 2024")
	want := []string{"Movie Title 2024", "Movie Title"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("searchTitles() = %#v, want %#v", got, want)
	}
}

func TestSearchTitlesForRadarrDeduplicatesWhenYearAbsent(t *testing.T) {
	got := searchTitles("radarr", "Movie Title")
	want := []string{"Movie Title"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("searchTitles() = %#v, want %#v", got, want)
	}
}

func TestSearchTitlesForSonarrRemovesTrailingEpisode(t *testing.T) {
	got := searchTitles("sonarr", "Series Title 007")
	want := []string{"Series Title"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("searchTitles() = %#v, want %#v", got, want)
	}
}

func TestUniqueNonEmptyTrimsSkipsEmptyAndDeduplicates(t *testing.T) {
	got := uniqueNonEmpty([]string{" Alpha ", "", "Alpha", "Beta", " Beta "})
	want := []string{"Alpha", "Beta"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("uniqueNonEmpty() = %#v, want %#v", got, want)
	}
}

func TestRemoveSeasonEpisode(t *testing.T) {
	got := removeSeasonEpisode("Series Title S02 08")
	want := "Series Title"
	if got != want {
		t.Fatalf("removeSeasonEpisode() = %q, want %q", got, want)
	}
}
