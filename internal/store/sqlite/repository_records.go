package sqlite

type SystemConfig struct {
	ID          SystemConfigID `json:"id"`
	Key         string         `json:"key"`
	Value       *string        `json:"value,omitempty"`
	ValidStatus ValidStatus    `json:"validStatus"`
	CreateTime  *string        `json:"createTime,omitempty"`
	UpdateTime  *string        `json:"updateTime,omitempty"`
}
type SystemUser struct {
	ID                     SystemUserID `json:"id"`
	Username               string       `json:"username"`
	Password               *string      `json:"password,omitempty"`
	Role                   *string      `json:"role,omitempty"`
	ValidStatus            ValidStatus  `json:"validStatus"`
	CreateTime, UpdateTime *string      `json:"-"`
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
type SonarrExample struct {
	Hash                   string
	OriginalText           string
	FormatText             *string
	ValidStatus            ValidStatus
	CreateTime, UpdateTime *string
}
type RadarrExample SonarrExample
type SonarrTitle struct {
	ID                     SonarrTitleID
	TVDBID, SNO            int64
	MainTitle, Title       string
	CleanTitle             *string
	SeasonNumber           int64
	Monitored              MonitoredStatus
	ValidStatus            ValidStatus
	CreateTime, UpdateTime *string
	SeriesID               *int64
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
