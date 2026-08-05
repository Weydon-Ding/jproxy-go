package title

import "jproxy-go/internal/store/sqlite"

const (
	maxBodyBytes = 256 * 1024
	maxBatch     = 200
)

type pageDTO[T any] struct {
	Current  int64 `json:"current"`
	PageSize int64 `json:"pageSize"`
	Total    int64 `json:"total"`
	List     []T   `json:"list"`
}

type sonarrTitleDTO struct {
	ID           int64   `json:"id"`
	TVDBID       int64   `json:"tvdbId"`
	SNO          int64   `json:"sno"`
	MainTitle    string  `json:"mainTitle"`
	Title        string  `json:"title"`
	CleanTitle   *string `json:"cleanTitle,omitempty"`
	SeasonNumber int64   `json:"seasonNumber"`
	Monitored    int64   `json:"monitored"`
	ValidStatus  int64   `json:"validStatus"`
	CreateTime   *string `json:"createTime,omitempty"`
	UpdateTime   *string `json:"updateTime,omitempty"`
	SeriesID     *int64  `json:"seriesId,omitempty"`
}

type radarrTitleDTO struct {
	ID          int64   `json:"id"`
	TMDBID      int64   `json:"tmdbId"`
	SNO         int64   `json:"sno"`
	MainTitle   string  `json:"mainTitle"`
	Title       string  `json:"title"`
	CleanTitle  string  `json:"cleanTitle"`
	Year        int64   `json:"year"`
	Monitored   int64   `json:"monitored"`
	ValidStatus int64   `json:"validStatus"`
	CreateTime  *string `json:"createTime,omitempty"`
	UpdateTime  *string `json:"updateTime,omitempty"`
	MovieID     *int64  `json:"movieId,omitempty"`
}

type tmdbTitleDTO struct {
	ID          *int64  `json:"id,omitempty"`
	TVDBID      int64   `json:"tvdbId"`
	TMDBID      *int64  `json:"tmdbId"`
	Language    string  `json:"language"`
	Title       string  `json:"title"`
	CleanTitle  string  `json:"cleanTitle,omitempty"`
	ValidStatus int64   `json:"validStatus"`
	CreateTime  *string `json:"createTime,omitempty"`
	UpdateTime  *string `json:"updateTime,omitempty"`
}

func fromSonarr(value sqlite.SonarrTitle) sonarrTitleDTO {
	return sonarrTitleDTO{ID: int64(value.ID), TVDBID: value.TVDBID, SNO: value.SNO, MainTitle: value.MainTitle, Title: value.Title, CleanTitle: value.CleanTitle, SeasonNumber: value.SeasonNumber, Monitored: int64(value.Monitored), ValidStatus: int64(value.ValidStatus), CreateTime: value.CreateTime, UpdateTime: value.UpdateTime, SeriesID: value.SeriesID}
}

func fromRadarr(value sqlite.RadarrTitle) radarrTitleDTO {
	return radarrTitleDTO{ID: int64(value.ID), TMDBID: value.TMDBID, SNO: value.SNO, MainTitle: value.MainTitle, Title: value.Title, CleanTitle: value.CleanTitle, Year: value.Year, Monitored: int64(value.Monitored), ValidStatus: int64(value.ValidStatus), CreateTime: value.CreateTime, UpdateTime: value.UpdateTime, MovieID: value.MovieID}
}

func fromTMDB(value sqlite.TMDBTitle, cleanTitle string) tmdbTitleDTO {
	id := int64(value.ID)
	return tmdbTitleDTO{ID: &id, TVDBID: value.TVDBID, TMDBID: value.TMDBID, Language: value.Language, Title: value.Title, CleanTitle: cleanTitle, ValidStatus: int64(value.ValidStatus), CreateTime: value.CreateTime, UpdateTime: value.UpdateTime}
}
