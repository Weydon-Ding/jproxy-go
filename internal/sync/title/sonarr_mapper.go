package titlesync

import (
	"jproxy-go/internal/format"
	"jproxy-go/internal/store/sqlite"
)

func MapSonarr(values []SonarrSeries, cleanRegex string) ([]sqlite.SonarrTitle, error) {
	rows := make([]sqlite.SonarrTitle, 0)
	ids := map[int64]struct{}{}
	coordinates := map[[2]int64]struct{}{}
	for _, value := range values {
		if value.ID <= 0 || value.TVDBID <= 0 || value.Title == "" || value.TitleSlug == "" {
			return nil, errInvalidRows
		}
		candidates := append([]SonarrAlternateTitle{{Title: value.Title, SceneSeasonNumber: -1}, {Title: value.TitleSlug, SceneSeasonNumber: -1}}, value.AlternateTitles...)
		for sno, candidate := range candidates {
			if candidate.Title == "" {
				return nil, errInvalidRows
			}
			id, err := stableID(value.TVDBID, int64(sno))
			if err != nil {
				return nil, err
			}
			coordinate := [2]int64{value.TVDBID, int64(sno)}
			if _, exists := coordinates[coordinate]; exists {
				return nil, errInvalidRows
			}
			coordinates[coordinate] = struct{}{}
			if _, exists := ids[id]; exists {
				return nil, errInvalidRows
			}
			ids[id] = struct{}{}
			clean := format.CleanTitle(candidate.Title, cleanRegex)
			title := value.Title
			if sno >= 2 {
				title = candidate.Title
			}
			seriesID := value.ID
			monitored := sqlite.Unmonitored
			if value.Monitored {
				monitored = sqlite.Monitored
			}
			rows = append(rows, sqlite.SonarrTitle{ID: sqlite.SonarrTitleID(id), TVDBID: value.TVDBID, SNO: int64(sno), MainTitle: value.Title, Title: title, CleanTitle: &clean, SeasonNumber: candidate.SceneSeasonNumber, Monitored: monitored, ValidStatus: sqlite.Valid, SeriesID: &seriesID})
		}
	}
	return rows, nil
}
