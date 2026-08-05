package titlesync

import (
	"testing"
)

func TestMapSonarr_preservesJavaRowOrderAndFields_whenSeriesValid(t *testing.T) {
	rows, err := MapSonarr([]SonarrSeries{{ID: 7, TVDBID: 12, Title: "The Main", TitleSlug: "the-main", Monitored: true, AlternateTitles: []SonarrAlternateTitle{{Title: "Alt", SceneSeasonNumber: 2}}}}, `the`)
	if err != nil || len(rows) != 3 || rows[0].ID != 120 || rows[1].Title != "The Main" || *rows[1].CleanTitle != "-main" || rows[2].SeasonNumber != 2 || rows[2].ID != 122 || rows[0].SeriesID == nil || *rows[0].SeriesID != 7 {
		t.Fatalf("rows=%#v err=%v", rows, err)
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
