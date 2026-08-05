package titlesync

type SonarrSeries struct {
	ID              int64
	TVDBID          int64
	Title           string
	TitleSlug       string
	Monitored       bool
	AlternateTitles []SonarrAlternateTitle
}

type SonarrAlternateTitle struct {
	Title             string
	SceneSeasonNumber int64
}

type RadarrMovie struct {
	ID              int64
	TMDBID          int64
	Title           string
	Path            string
	CleanTitle      string
	OriginalTitle   string
	Year            int64
	Monitored       bool
	AlternateTitles []RadarrAlternateTitle
}

type RadarrAlternateTitle struct{ Title string }
