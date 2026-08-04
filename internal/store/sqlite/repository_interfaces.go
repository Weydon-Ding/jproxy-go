package sqlite

import "context"

type SystemConfigRepository interface {
	Get(context.Context, SystemConfigID) (SystemConfig, error)
	Upsert(context.Context, SystemConfig) error
	UpsertBatch(context.Context, []SystemConfig) error
	List(context.Context) ([]SystemConfig, error)
	ValueByKey(context.Context, string) (string, error)
}
type SystemUserRepository interface {
	Get(context.Context, SystemUserID) (SystemUser, error)
	Upsert(context.Context, SystemUser) error
	FindByUsername(context.Context, string) (SystemUser, error)
}
type SonarrRuleRepository interface {
	Get(context.Context, RuleID) (SonarrRule, error)
	Upsert(context.Context, SonarrRule) error
	UpsertRemote(context.Context, SonarrRuleInput) error
	Page(context.Context, RuleFilter) (PageResult[SonarrRule], error)
	UpsertBatch(context.Context, SonarrRuleBatch) error
	DeleteBatch(context.Context, RuleIDs) error
	SwitchValidStatus(context.Context, RuleIDs, ValidStatus) error
	Replace(context.Context, SonarrRuleBatch) error
}
type RadarrRuleRepository interface {
	Get(context.Context, RuleID) (RadarrRule, error)
	Upsert(context.Context, RadarrRule) error
	UpsertRemote(context.Context, RadarrRuleInput) error
	Page(context.Context, RuleFilter) (PageResult[RadarrRule], error)
	UpsertBatch(context.Context, RadarrRuleBatch) error
	DeleteBatch(context.Context, RuleIDs) error
	SwitchValidStatus(context.Context, RuleIDs, ValidStatus) error
	Replace(context.Context, RadarrRuleBatch) error
}
type SonarrTitleRepository interface {
	Get(context.Context, SonarrTitleID) (SonarrTitle, error)
	Upsert(context.Context, SonarrTitle) error
	Page(context.Context, SonarrTitleFilter) (PageResult[SonarrTitle], error)
	UpsertBatch(context.Context, SonarrTitleBatch) error
	DeleteBatch(context.Context, SonarrTitleIDs) error
	Replace(context.Context, SonarrTitleBatch) error
	NeedTMDBSync(context.Context) ([]int64, error)
	WithTMDBTitles(context.Context) ([]SonarrTitle, error)
}
type RadarrTitleRepository interface {
	Get(context.Context, RadarrTitleID) (RadarrTitle, error)
	Upsert(context.Context, RadarrTitle) error
	Page(context.Context, RadarrTitleFilter) (PageResult[RadarrTitle], error)
	UpsertBatch(context.Context, RadarrTitleBatch) error
	DeleteBatch(context.Context, RadarrTitleIDs) error
	Replace(context.Context, RadarrTitleBatch) error
}
type TMDBTitleRepository interface {
	Get(context.Context, TMDBTitleID) (TMDBTitle, error)
	Upsert(context.Context, TMDBTitle) error
	Page(context.Context, TMDBTitleFilter) (PageResult[TMDBTitle], error)
	UpsertBatch(context.Context, TMDBTitleBatch) error
	DeleteBatch(context.Context, TMDBTitleIDs) error
	Replace(context.Context, TMDBTitleBatch) error
	FindByTVDBID(context.Context, int64) ([]TMDBTitle, error)
}
