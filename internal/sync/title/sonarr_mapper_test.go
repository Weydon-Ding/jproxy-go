package titlesync

import (
	"reflect"
	"testing"

	"jproxy-go/internal/store/sqlite"
)

func TestMapSonarr_preservesJavaRowOrderAndFields_whenSeriesValid(t *testing.T) {
	// Given
	seriesID := int64(7)
	firstClean := "main"
	secondClean := "-main"
	thirdClean := "alt"
	input := []SonarrSeries{{ID: 7, TVDBID: 12, Title: "The Main", TitleSlug: "the-main", Monitored: true, AlternateTitles: []SonarrAlternateTitle{{Title: "Alt", SceneSeasonNumber: 2}}}}
	expected := []sqlite.SonarrTitle{{ID: 120, TVDBID: 12, SNO: 0, MainTitle: "The Main", Title: "The Main", CleanTitle: &firstClean, SeasonNumber: -1, Monitored: sqlite.Monitored, ValidStatus: sqlite.Valid, SeriesID: &seriesID}, {ID: 121, TVDBID: 12, SNO: 1, MainTitle: "The Main", Title: "The Main", CleanTitle: &secondClean, SeasonNumber: -1, Monitored: sqlite.Monitored, ValidStatus: sqlite.Valid, SeriesID: &seriesID}, {ID: 122, TVDBID: 12, SNO: 2, MainTitle: "The Main", Title: "Alt", CleanTitle: &thirdClean, SeasonNumber: 2, Monitored: sqlite.Monitored, ValidStatus: sqlite.Valid, SeriesID: &seriesID}}

	// When
	rows, err := MapSonarr(input, `the`)

	// Then
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if !reflect.DeepEqual(rows, expected) {
		t.Fatalf("rows=%#v\nexpected=%#v", rows, expected)
	}
}
func TestMapSonarr_rejectsCollisionAndOverflow_whenBatchCannotPersist(t *testing.T) {
	if _, err := MapSonarr([]SonarrSeries{{ID: 1, TVDBID: 2147483647, Title: "a", TitleSlug: "b"}}, ""); err == nil {
		t.Fatal("want overflow")
	}
	if _, err := MapSonarr([]SonarrSeries{{ID: 1, TVDBID: 1, Title: "a", TitleSlug: "b"}, {ID: 2, TVDBID: 1, Title: "c", TitleSlug: "d"}}, ""); err == nil {
		t.Fatal("want collision")
	}
}
