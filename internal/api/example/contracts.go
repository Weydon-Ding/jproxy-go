package example

type saveDTO struct {
	OriginalText string `json:"originalText"`
}

type exampleDTO struct {
	Hash         string `json:"hash"`
	OriginalText string `json:"originalText"`
	FormatText   string `json:"formatText"`
	ValidStatus  int64  `json:"validStatus"`
	CreateTime   string `json:"createTime,omitempty"`
	UpdateTime   string `json:"updateTime,omitempty"`
}

type pageDTO struct {
	Current  int64        `json:"current"`
	PageSize int64        `json:"pageSize"`
	Total    int64        `json:"total"`
	List     []exampleDTO `json:"list"`
}
