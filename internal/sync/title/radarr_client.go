package titlesync

import (
	"context"
	"fmt"
)

type RadarrClient struct{ request RequestClient }

func NewRadarrClient(request RequestClient) RadarrClient { return RadarrClient{request: request} }
func (c RadarrClient) Fetch(ctx context.Context, cfg providerConfig) ([]RadarrMovie, error) {
	body, err := c.request.get(ctx, "radarr", "movie", cfg, "api", "v3", "movie")
	if err != nil {
		return nil, err
	}
	var values *[]radarrWire
	if err := decodeSingleJSON(body, &values, "radarr", "movie"); err != nil {
		return nil, err
	}
	if values == nil {
		return nil, &HTTPError{Provider: "radarr", Operation: "movie", Field: "response"}
	}
	result := make([]RadarrMovie, len(*values))
	for index, value := range *values {
		parsed, parseErr := value.parse()
		if parseErr != nil {
			return nil, &HTTPError{Provider: "radarr", Operation: "movie", Field: parseErr.Error()}
		}
		result[index] = parsed
	}
	return result, nil
}

type radarrWire struct {
	ID              *int64                 `json:"id"`
	TMDBID          *int64                 `json:"tmdbId"`
	Title           *string                `json:"title"`
	Path            *string                `json:"path"`
	CleanTitle      *string                `json:"cleanTitle"`
	OriginalTitle   *string                `json:"originalTitle"`
	Year            *int64                 `json:"year"`
	Monitored       *bool                  `json:"monitored"`
	AlternateTitles *[]radarrAlternateWire `json:"alternateTitles"`
}
type radarrAlternateWire struct {
	Title *string `json:"title"`
}

func (w radarrWire) parse() (RadarrMovie, error) {
	if w.ID == nil || *w.ID <= 0 || *w.ID > 2147483647 {
		return RadarrMovie{}, fmt.Errorf("id")
	}
	if w.TMDBID == nil || *w.TMDBID <= 0 || *w.TMDBID > 2147483647 {
		return RadarrMovie{}, fmt.Errorf("tmdbId")
	}
	if w.Title == nil || *w.Title == "" {
		return RadarrMovie{}, fmt.Errorf("title")
	}
	if w.Path == nil || *w.Path == "" {
		return RadarrMovie{}, fmt.Errorf("path")
	}
	if w.CleanTitle == nil || *w.CleanTitle == "" {
		return RadarrMovie{}, fmt.Errorf("cleanTitle")
	}
	if w.OriginalTitle == nil || *w.OriginalTitle == "" {
		return RadarrMovie{}, fmt.Errorf("originalTitle")
	}
	if w.Year == nil || *w.Year < 0 || *w.Year > 9999 {
		return RadarrMovie{}, fmt.Errorf("year")
	}
	if w.Monitored == nil || w.AlternateTitles == nil {
		return RadarrMovie{}, fmt.Errorf("array")
	}
	value := RadarrMovie{ID: *w.ID, TMDBID: *w.TMDBID, Title: *w.Title, Path: *w.Path, CleanTitle: *w.CleanTitle, OriginalTitle: *w.OriginalTitle, Year: *w.Year, Monitored: *w.Monitored, AlternateTitles: make([]RadarrAlternateTitle, len(*w.AlternateTitles))}
	for index, alternate := range *w.AlternateTitles {
		if alternate.Title == nil || *alternate.Title == "" {
			return RadarrMovie{}, fmt.Errorf("alternateTitles.title")
		}
		value.AlternateTitles[index] = RadarrAlternateTitle{Title: *alternate.Title}
	}
	return value, nil
}
