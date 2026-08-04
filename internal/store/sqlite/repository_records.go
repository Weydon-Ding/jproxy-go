package sqlite

type SystemConfig struct {
	ID                     SystemConfigID
	Key                    string
	Value                  *string
	ValidStatus            ValidStatus
	CreateTime, UpdateTime *string
}
type SystemUser struct {
	ID                     SystemUserID
	Username               string
	Password, Role         *string
	ValidStatus            ValidStatus
	CreateTime, UpdateTime *string
}
type SonarrRule struct {
	ID                     RuleID
	Token                  string
	Priority               int64
	Regex, Replacement     string
	Offset                 int64
	Example                string
	Remark, Author         *string
	ValidStatus            ValidStatus
	CreateTime, UpdateTime *string
}
type RadarrRule SonarrRule
type SonarrTitle struct {
	ID                           SonarrTitleID
	TVDBID, SNO                  int64
	MainTitle, Title, CleanTitle string
	SeasonNumber                 int64
	Monitored                    MonitoredStatus
	ValidStatus                  ValidStatus
	CreateTime, UpdateTime       *string
	SeriesID                     *int64
}
type RadarrTitle struct {
	ID                           RadarrTitleID
	TMDBID, SNO                  int64
	MainTitle, Title, CleanTitle string
	Year                         int64
	Monitored                    MonitoredStatus
	ValidStatus                  ValidStatus
	CreateTime, UpdateTime       *string
	MovieID                      *int64
}
type TMDBTitle struct {
	ID                     TMDBTitleID
	TVDBID                 int64
	TMDBID                 *int64
	Language, Title        string
	ValidStatus            ValidStatus
	CreateTime, UpdateTime *string
}
