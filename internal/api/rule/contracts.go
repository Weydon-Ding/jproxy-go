package rule

import "jproxy-go/internal/store/sqlite"

const (
	maxBodyBytes = 256 * 1024
	maxBatch     = 200
	primaryID    = "00000000000000000000000000000000"
)

type ruleDTO struct {
	ID          string  `json:"id"`
	Token       string  `json:"token"`
	Priority    int64   `json:"priority"`
	Regex       string  `json:"regex"`
	Replacement string  `json:"replacement"`
	Offset      int64   `json:"offset"`
	Example     string  `json:"example"`
	Remark      *string `json:"remark"`
	Author      *string `json:"author"`
	ValidStatus *int64  `json:"validStatus"`
	CreateTime  *string `json:"createTime"`
	UpdateTime  *string `json:"updateTime"`
}
type pageDTO struct {
	Current  int64     `json:"current"`
	PageSize int64     `json:"pageSize"`
	Total    int64     `json:"total"`
	List     []ruleDTO `json:"list"`
}

func fromRule(v sqlite.SonarrRule) ruleDTO {
	status := int64(v.ValidStatus)
	return ruleDTO{ID: string(v.ID), Token: v.Token, Priority: v.Priority, Regex: v.Regex, Replacement: v.Replacement, Offset: v.Offset, Example: v.Example, Remark: v.Remark, Author: v.Author, ValidStatus: &status, CreateTime: v.CreateTime, UpdateTime: v.UpdateTime}
}
func toRule(v ruleDTO) sqlite.SonarrRule {
	status := sqlite.Valid
	if v.ValidStatus != nil {
		status = sqlite.ValidStatus(*v.ValidStatus)
	}
	return sqlite.SonarrRule{ID: sqlite.RuleID(v.ID), Token: v.Token, Priority: v.Priority, Regex: v.Regex, Replacement: v.Replacement, Offset: v.Offset, Example: v.Example, Remark: v.Remark, Author: v.Author, ValidStatus: status, CreateTime: v.CreateTime, UpdateTime: v.UpdateTime}
}
func ruleIDs(ids []string) sqlite.RuleIDs {
	out := make([]sqlite.RuleID, len(ids))
	for i, id := range ids {
		out[i] = sqlite.RuleID(id)
	}
	return sqlite.RuleIDs{IDs: out}
}
