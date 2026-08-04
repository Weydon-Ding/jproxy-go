package sqlite

import "errors"

const batchLimit = 200

var ErrBatchTooLarge = errors.New("SQLite repository batch exceeds 200 rows")

type ValidStatus int64

const (
	Invalid ValidStatus = 0
	Valid   ValidStatus = 1
)

type MonitoredStatus int64

const (
	Unmonitored MonitoredStatus = 0
	Monitored   MonitoredStatus = 1
)

type SystemConfigID int64
type SystemUserID int64
type SonarrTitleID int64
type RadarrTitleID int64
type TMDBTitleID int64
type RuleID string
type PageInput struct{ Current, Size int64 }
type PageResult[T any] struct {
	Current, Size, Total int64
	List                 []T
}

func (p PageInput) normalized() PageInput {
	if p.Current < 1 {
		p.Current = 1
	}
	if p.Size < 1 {
		p.Size = 10
	}
	if p.Size > 200 {
		p.Size = 200
	}
	return p
}

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
type RuleFilter struct {
	Page          PageInput
	Token, Remark *string
}
type SonarrTitleFilter struct {
	Page   PageInput
	Title  *string
	TVDBID *int64
}
type RadarrTitleFilter struct {
	Page   PageInput
	Title  *string
	TMDBID *int64
}
type TMDBTitleFilter struct {
	Page   PageInput
	Title  *string
	TVDBID *int64
}
type RadarrTitleBatch struct{ Rows []RadarrTitle }
type RadarrTitleIDs struct{ IDs []RadarrTitleID }
