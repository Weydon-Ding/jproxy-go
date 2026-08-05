package rule

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

func validate(v *ruleDTO) error {
	v.Token = strings.TrimSpace(v.Token)
	v.Regex = strings.TrimSpace(v.Regex)
	v.Example = strings.TrimSpace(v.Example)
	if v.Token == "" || v.Regex == "" || v.Example == "" || v.Priority < -2147483648 || v.Priority > 2147483647 || v.Offset < -2147483648 || v.Offset > 2147483647 {
		return errors.New("invalid")
	}
	if v.ValidStatus != nil && *v.ValidStatus != 0 && *v.ValidStatus != 1 {
		return errors.New("status")
	}
	re, err := regexp.Compile(strings.ReplaceAll(v.Regex, "{cleanTitle}", "placeholder"))
	if err != nil {
		return errors.New("regex")
	}
	for i := 0; i < len(v.Replacement); i++ {
		if v.Replacement[i] == '$' && i+1 < len(v.Replacement) && v.Replacement[i+1] >= '0' && v.Replacement[i+1] <= '9' {
			n := int(v.Replacement[i+1] - '0')
			if n > re.NumSubexp() {
				return errors.New("replacement")
			}
		}
	}
	_ = re.ReplaceAllString(v.Example, v.Replacement)
	return nil
}
func page(r *http.Request) (int64, int64, error) {
	cur, size := int64(1), int64(10)
	var err error
	if raw := r.URL.Query().Get("current"); raw != "" {
		cur, err = strconv.ParseInt(raw, 10, 64)
		if err != nil || cur < 1 {
			return 0, 0, errors.New("page")
		}
	}
	if raw := r.URL.Query().Get("pageSize"); raw != "" {
		size, err = strconv.ParseInt(raw, 10, 64)
		if err != nil || size < 1 || size > maxBatch {
			return 0, 0, errors.New("page")
		}
	}
	return cur, size, nil
}
func id() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}
func primary(ids []string) bool {
	for _, id := range ids {
		if id == primaryID {
			return true
		}
	}
	return false
}
