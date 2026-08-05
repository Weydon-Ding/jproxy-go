package titlesync

import (
	"context"
	"fmt"
)

type SonarrClient struct{ request RequestClient }

func NewSonarrClient(request RequestClient) SonarrClient { return SonarrClient{request: request} }

func (c SonarrClient) Fetch(ctx context.Context, cfg providerConfig) ([]SonarrSeries, error) {
	body, err := c.request.get(ctx, "sonarr", "series", cfg, "api", "v3", "series")
	if err != nil {
		return nil, err
	}
	var values *[]sonarrWire
	if err := decodeSingleJSON(body, &values, "sonarr", "series"); err != nil {
		return nil, err
	}
	if values == nil {
		return nil, &HTTPError{Provider: "sonarr", Operation: "series", Field: "response"}
	}
	result := make([]SonarrSeries, len(*values))
	for index, value := range *values {
		parsed, parseErr := value.parse()
		if parseErr != nil {
			return nil, &HTTPError{Provider: "sonarr", Operation: "series", Field: parseErr.Error()}
		}
		result[index] = parsed
	}
	return result, nil
}

type sonarrWire struct {
	ID              *int64                 `json:"id"`
	TVDBID          *int64                 `json:"tvdbId"`
	Title           *string                `json:"title"`
	TitleSlug       *string                `json:"titleSlug"`
	Monitored       *bool                  `json:"monitored"`
	AlternateTitles *[]sonarrAlternateWire `json:"alternateTitles"`
}
type sonarrAlternateWire struct {
	Title             *string `json:"title"`
	SceneSeasonNumber *int64  `json:"sceneSeasonNumber"`
}

func (w sonarrWire) parse() (SonarrSeries, error) {
	if w.ID == nil || *w.ID <= 0 || *w.ID > 2147483647 {
		return SonarrSeries{}, fmt.Errorf("id")
	}
	if w.TVDBID == nil || *w.TVDBID <= 0 || *w.TVDBID > 2147483647 {
		return SonarrSeries{}, fmt.Errorf("tvdbId")
	}
	if w.Title == nil || *w.Title == "" {
		return SonarrSeries{}, fmt.Errorf("title")
	}
	if w.TitleSlug == nil || *w.TitleSlug == "" {
		return SonarrSeries{}, fmt.Errorf("titleSlug")
	}
	if w.Monitored == nil {
		return SonarrSeries{}, fmt.Errorf("monitored")
	}
	if w.AlternateTitles == nil {
		return SonarrSeries{}, fmt.Errorf("alternateTitles")
	}
	value := SonarrSeries{ID: *w.ID, TVDBID: *w.TVDBID, Title: *w.Title, TitleSlug: *w.TitleSlug, Monitored: *w.Monitored, AlternateTitles: make([]SonarrAlternateTitle, len(*w.AlternateTitles))}
	for index, alternate := range *w.AlternateTitles {
		if alternate.Title == nil || *alternate.Title == "" {
			return SonarrSeries{}, fmt.Errorf("alternateTitles.title")
		}
		if alternate.SceneSeasonNumber == nil || *alternate.SceneSeasonNumber < -1 || *alternate.SceneSeasonNumber > 2147483647 {
			return SonarrSeries{}, fmt.Errorf("alternateTitles.sceneSeasonNumber")
		}
		value.AlternateTitles[index] = SonarrAlternateTitle{Title: *alternate.Title, SceneSeasonNumber: *alternate.SceneSeasonNumber}
	}
	return value, nil
}
