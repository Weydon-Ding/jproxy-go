package sqlite

import "strings"

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
	if p.Size > batchLimit {
		p.Size = batchLimit
	}
	return p
}

type RuleFilter struct {
	Page          PageInput
	Token, Remark *string
}
type ExampleFilter struct {
	Page         PageInput
	OriginalText *string
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

func (f RuleFilter) normalized() RuleFilter {
	f.Page = f.Page.normalized()
	f.Token = trimFilter(f.Token)
	f.Remark = trimFilter(f.Remark)
	return f
}
func (f ExampleFilter) normalized() ExampleFilter {
	f.Page = f.Page.normalized()
	f.OriginalText = trimFilter(f.OriginalText)
	return f
}
func (f SonarrTitleFilter) normalized() SonarrTitleFilter {
	f.Page = f.Page.normalized()
	f.Title = trimFilter(f.Title)
	return f
}
func (f RadarrTitleFilter) normalized() RadarrTitleFilter {
	f.Page = f.Page.normalized()
	f.Title = trimFilter(f.Title)
	return f
}
func (f TMDBTitleFilter) normalized() TMDBTitleFilter {
	f.Page = f.Page.normalized()
	f.Title = trimFilter(f.Title)
	return f
}
func trimFilter(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}
