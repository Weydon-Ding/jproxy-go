package titlesync

import "testing"

func TestMapRadarr_preservesPathAndCloneOrder_whenMovieValid(t *testing.T) {
	rows, err := MapRadarr([]RadarrMovie{{ID: 7, TMDBID: 12, Title: "Main", Path: `C:\Movies\Path Title (2024)`, CleanTitle: "upstream", OriginalTitle: "Original", Year: 2024, AlternateTitles: []RadarrAlternateTitle{{Title: "Alt"}}}}, "")
	if err != nil || len(rows) != 5 || rows[1].Title != "Path Title" || rows[2].Title != "Path Title" || rows[2].CleanTitle != "upstream" || rows[4].ID != 124 || rows[0].MovieID == nil || *rows[0].MovieID != 7 {
		t.Fatalf("rows=%#v err=%v", rows, err)
	}
}

func TestMapRadarr_rejectsCollisionAndInvalidPath_whenBatchCannotPersist(t *testing.T) {
	base := RadarrMovie{ID: 1, TMDBID: 1, Title: "a", Path: "x", CleanTitle: "c", OriginalTitle: "o"}
	if _, err := MapRadarr([]RadarrMovie{base, {ID: 2, TMDBID: 1, Title: "b", Path: "y", CleanTitle: "d", OriginalTitle: "p"}}, ""); err == nil {
		t.Fatal("want collision")
	}
	base.Path = "/"
	if _, err := MapRadarr([]RadarrMovie{base}, ""); err == nil {
		t.Fatal("want path rejection")
	}
}
