package rulesync

import (
	"errors"
	"regexp"
	"strings"

	"jproxy-go/internal/store/sqlite"
)

var errInvalidPayload = errors.New("invalid rule payload")

type remoteRule struct {
	ID          *string `json:"id"`
	Token       *string `json:"token"`
	Priority    *int64  `json:"priority"`
	Regex       *string `json:"regex"`
	Replacement *string `json:"replacement"`
	Offset      *int64  `json:"offset"`
	Example     *string `json:"example"`
	Remark      *string `json:"remark"`
	Author      *string `json:"author"`
	ValidStatus *int64  `json:"validStatus"`
}

func (r remoteRule) sonarr(author string) (sqlite.SonarrRuleInput, error) {
	value, err := r.value(author)
	if err != nil {
		return sqlite.SonarrRuleInput{}, err
	}
	return sqlite.SonarrRuleInput{Rule: value, ValidStatus: remoteStatus(*r.ValidStatus)}, nil
}

func (r remoteRule) radarr(author string) (sqlite.RadarrRuleInput, error) {
	value, err := r.value(author)
	if err != nil {
		return sqlite.RadarrRuleInput{}, err
	}
	return sqlite.RadarrRuleInput{Rule: sqlite.RadarrRule(value), ValidStatus: remoteStatus(*r.ValidStatus)}, nil
}

func (r remoteRule) value(author string) (sqlite.SonarrRule, error) {
	if r.ID == nil || r.Token == nil || r.Priority == nil || r.Regex == nil || r.Replacement == nil || r.Offset == nil || r.Example == nil || r.Remark == nil || r.Author == nil || r.ValidStatus == nil {
		return sqlite.SonarrRule{}, errInvalidPayload
	}
	if *r.ID == "" || *r.Token == "" || *r.Regex == "" || *r.Example == "" || *r.Remark == "" || *r.Author != author || *r.ValidStatus != int64(sqlite.Valid) && *r.ValidStatus != int64(sqlite.Invalid) || *r.Priority < -2147483648 || *r.Priority > 2147483647 || *r.Offset < -2147483648 || *r.Offset > 2147483647 {
		return sqlite.SonarrRule{}, errInvalidPayload
	}
	if !validToken(*r.Token) {
		return sqlite.SonarrRule{}, errInvalidPayload
	}
	re, err := regexp.Compile(strings.ReplaceAll(*r.Regex, "{cleanTitle}", "placeholder"))
	if err != nil || !validReplacement(*r.Replacement, re.NumSubexp()) {
		return sqlite.SonarrRule{}, errInvalidPayload
	}
	return sqlite.SonarrRule{ID: sqlite.RuleID(*r.ID), Token: *r.Token, Priority: *r.Priority, Regex: *r.Regex, Replacement: *r.Replacement, Offset: *r.Offset, Example: *r.Example, Remark: r.Remark, Author: r.Author}, nil
}

func validToken(token string) bool {
	switch token {
	case "title", "season", "episode", "language", "resolution", "quality", "group", "cleanTitle", "year":
		return true
	default:
		return false
	}
}

func validReplacement(value string, groups int) bool {
	for index := 0; index+1 < len(value); index++ {
		if value[index] == '$' && value[index+1] >= '0' && value[index+1] <= '9' && int(value[index+1]-'0') > groups {
			return false
		}
	}
	return true
}

func remoteStatus(status int64) *sqlite.ValidStatus {
	if status == int64(sqlite.Valid) {
		return nil
	}
	value := sqlite.ValidStatus(status)
	return &value
}
