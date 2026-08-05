package system

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"
)

const remoteBodyLimit = 64 * 1024

const (
	defaultVersionURL      = "https://api.github.com/repos/LuckyPuppy514/jproxy/releases/latest"
	defaultAuthorURL       = "https://raw.githubusercontent.com/LuckyPuppy514/jproxy/main/src/main/resources/rule/author.json"
	defaultAuthorBackupURL = "https://github.rn.lckp.top/LuckyPuppy514/jproxy/main/src/main/resources/rule/author.json"
)

func (h *Handler) authorList(ctx context.Context) []string {
	primary, backup := h.options.AuthorURL, h.options.AuthorBackupURL
	if primary == "" {
		primary = defaultAuthorURL
	}
	if backup == "" {
		backup = defaultAuthorBackupURL
	}
	for _, source := range []string{primary, backup} {
		if authors, ok := fetchAuthors(ctx, source); ok {
			return authors
		}
	}
	return []string{"LuckyPuppy514"}
}

func fetchAuthors(ctx context.Context, source string) ([]string, bool) {
	var authors []string
	if !fetchJSON(ctx, source, &authors) || len(authors) == 0 {
		return nil, false
	}
	for _, author := range authors {
		if strings.TrimSpace(author) == "" {
			return nil, false
		}
	}
	return authors, true
}

func fetchVersion(ctx context.Context, source string) (string, bool) {
	var payload struct {
		TagName string `json:"tag_name"`
	}
	if !fetchJSON(ctx, source, &payload) || strings.TrimSpace(payload.TagName) == "" {
		return "", false
	}
	return strings.TrimPrefix(payload.TagName, "v"), true
}

func fetchJSON(ctx context.Context, source string, target any) bool {
	if source == "" {
		return false
	}
	attemptCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(attemptCtx, http.MethodGet, source, nil)
	if err != nil {
		return false
	}
	client := &http.Client{CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(request)
	if err != nil || response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		if response != nil {
			_ = response.Body.Close()
		}
		return false
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, remoteBodyLimit+1))
	if err != nil || len(body) > remoteBodyLimit {
		return false
	}
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.DisallowUnknownFields()
	return decoder.Decode(target) == nil && decoder.Decode(&struct{}{}) == io.EOF
}
