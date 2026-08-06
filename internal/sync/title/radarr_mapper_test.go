package titlesync

import (
	"reflect"
	"testing"

	"jproxy-go/internal/store/sqlite"
)

func TestMapRadarr_preservesPathAndCloneOrder_whenMovieValid(t *testing.T) {
	// Given
	movieID := int64(7)
	input := []RadarrMovie{{ID: 7, TMDBID: 12, Title: "Main", Path: `C:\Movies\Path Title (2024)`, CleanTitle: "upstream", OriginalTitle: "Original", Year: 2024, Monitored: true, AlternateTitles: []RadarrAlternateTitle{{Title: "Alt"}}}}
	expected := []sqlite.RadarrTitle{{ID: 120, TMDBID: 12, SNO: 0, MainTitle: "Main", Title: "Main", CleanTitle: "main", Year: 2024, Monitored: sqlite.Monitored, ValidStatus: sqlite.Valid, MovieID: &movieID}, {ID: 121, TMDBID: 12, SNO: 1, MainTitle: "Main", Title: "Path Title", CleanTitle: "path title", Year: 2024, Monitored: sqlite.Monitored, ValidStatus: sqlite.Valid, MovieID: &movieID}, {ID: 122, TMDBID: 12, SNO: 2, MainTitle: "Main", Title: "Path Title", CleanTitle: "upstream", Year: 2024, Monitored: sqlite.Monitored, ValidStatus: sqlite.Valid, MovieID: &movieID}, {ID: 123, TMDBID: 12, SNO: 3, MainTitle: "Main", Title: "Original", CleanTitle: "original", Year: 2024, Monitored: sqlite.Monitored, ValidStatus: sqlite.Valid, MovieID: &movieID}, {ID: 124, TMDBID: 12, SNO: 4, MainTitle: "Main", Title: "Alt", CleanTitle: "alt", Year: 2024, Monitored: sqlite.Monitored, ValidStatus: sqlite.Valid, MovieID: &movieID}}

	// When
	rows, err := MapRadarr(input, "")

	// Then
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if !reflect.DeepEqual(rows, expected) {
		t.Fatalf("rows=%#v\nexpected=%#v", rows, expected)
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
