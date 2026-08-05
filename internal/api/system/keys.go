package system

type Config struct {
	ID    int64
	Key   string
	Value string
}

var configs = []Config{
	{1, "sonarrUrl", ""}, {2, "sonarrApikey", ""}, {3, "sonarrIndexerFormat", "{title}"},
	{5, "sonarrLanguage1", "zh-CN"}, {6, "sonarrLanguage2", "zh-TW"}, {7, "radarrUrl", ""},
	{8, "radarrApikey", ""}, {9, "radarrIndexerFormat", "{title}"}, {10, "jackettUrl", ""},
	{11, "prowlarrUrl", ""}, {12, "qbittorrentUrl", ""}, {13, "transmissionUrl", ""},
	{14, "tmdbUrl", "https://api.themoviedb.org"}, {15, "tmdbApikey", ""}, {16, "cleanTitleRegex", ""},
	{17, "ruleSyncAuthors", "ALL"}, {18, "qbittorrentUsername", ""}, {19, "qbittorrentPassword", ""},
	{21, "transmissionUsername", ""}, {22, "transmissionPassword", ""},
}

func DefaultConfigs() []Config { return append([]Config(nil), configs...) }

func configByID(id int64) (Config, bool) {
	for _, item := range configs {
		if item.ID == id {
			return item, true
		}
	}
	return Config{}, false
}
