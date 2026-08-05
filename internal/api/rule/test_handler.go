package rule

import (
	"errors"
	"jproxy-go/internal/format"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

type TestHandler struct{}

func NewTestHandler() *TestHandler { return &TestHandler{} }
func (h *TestHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		w.Header().Set("Allow", "GET")
		writeError(w, 405)
		return
	}
	raw := r.URL.Query()
	offset := int64(0)
	var e error
	if raw.Get("offset") != "" {
		offset, e = strconv.ParseInt(raw.Get("offset"), 10, 64)
	}
	v := ruleDTO{Token: "test", Regex: raw.Get("regex"), Replacement: raw.Get("replacement"), Example: raw.Get("example"), Offset: offset}
	if e != nil || validate(&v) != nil {
		writeError(w, 400)
		return
	}
	re := regexp.MustCompile(strings.ReplaceAll(v.Regex, "{cleanTitle}", "placeholder"))
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	for _, line := range strings.Split(v.Example, "\n") {
		_, _ = w.Write([]byte(format.ExecuteOffset(re.ReplaceAllString(line, v.Replacement), int(v.Offset)) + "\n"))
	}
}

var _ = errors.New
