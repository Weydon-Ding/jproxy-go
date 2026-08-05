package titlesync

import (
	"regexp"
	"strings"

	"jproxy-go/internal/format"
	"jproxy-go/internal/store/sqlite"
)

var yearSuffix = regexp.MustCompile(` \(\d{4}\)$`)

func MapRadarr(values []RadarrMovie, cleanRegex string) ([]sqlite.RadarrTitle, error) {
	rows := make([]sqlite.RadarrTitle, 0)
	ids := map[int64]struct{}{}
	coordinates := map[[2]int64]struct{}{}
	for _, value := range values {
		pathTitle := titleFromPath(value.Path)
		if value.ID <= 0 || value.TMDBID <= 0 || value.Title == "" || value.Path == "" || pathTitle == "" || value.CleanTitle == "" || value.OriginalTitle == "" || value.Year < 0 || value.Year > 9999 {
			return nil, errInvalidRows
		}
		candidates := append([]string{value.Title, pathTitle, pathTitle, value.OriginalTitle}, alternateStrings(value.AlternateTitles)...)
		for sno, title := range candidates {
			if title == "" {
				return nil, errInvalidRows
			}
			id, err := stableID(value.TMDBID, int64(sno))
			if err != nil {
				return nil, err
			}
			coordinate := [2]int64{value.TMDBID, int64(sno)}
			if _, exists := coordinates[coordinate]; exists {
				return nil, errInvalidRows
			}
			coordinates[coordinate] = struct{}{}
			if _, exists := ids[id]; exists {
				return nil, errInvalidRows
			}
			ids[id] = struct{}{}
			clean := format.CleanTitle(title, cleanRegex)
			if sno == 2 {
				clean = value.CleanTitle
			}
			movieID := value.ID
			monitored := sqlite.Unmonitored
			if value.Monitored {
				monitored = sqlite.Monitored
			}
			rows = append(rows, sqlite.RadarrTitle{ID: sqlite.RadarrTitleID(id), TMDBID: value.TMDBID, SNO: int64(sno), MainTitle: value.Title, Title: title, CleanTitle: clean, Year: value.Year, Monitored: monitored, ValidStatus: sqlite.Valid, MovieID: &movieID})
		}
	}
	return rows, nil
}
func alternateStrings(values []RadarrAlternateTitle) []string {
	result := make([]string, len(values))
	for index, value := range values {
		result[index] = value.Title
	}
	return result
}
func titleFromPath(path string) string {
	index := strings.LastIndexAny(path, "/\\")
	if index >= 0 {
		path = path[index+1:]
	}
	return yearSuffix.ReplaceAllString(path, "")
}
