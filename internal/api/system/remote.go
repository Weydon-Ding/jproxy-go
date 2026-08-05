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
	defaultAuthorURL       = "https://raw.githubusercontent.com/LuckyPuppy514/jproxy/master/rule"
	defaultAuthorBackupURL = "https://raw.githubusercontent.com/LuckyPuppy514/jproxy/main/rule"
)

func (h *Handler) authorList() []string {
	primary, backup := h.options.AuthorURL, h.options.AuthorBackupURL
	if primary == "" {
		primary = defaultAuthorURL
	}
	if backup == "" {
		backup = defaultAuthorBackupURL
	}
	for _, source := range []string{primary, backup} {
		if authors, ok := fetchAuthors(source); ok {
			return authors
		}
	}
	return []string{"LuckyPuppy514"}
}

func fetchAuthors(source string) ([]string, bool) {
	if source == "" {
		return nil, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	client := &http.Client{Timeout: 3 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(source, "/")+"/author.json", nil)
	if err != nil {
		return nil, false
	}
	response, err := client.Do(request)
	if err != nil || response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		if response != nil {
			response.Body.Close()
		}
		return nil, false
	}
	defer response.Body.Close()
	decoder := json.NewDecoder(io.LimitReader(response.Body, remoteBodyLimit+1))
	var authors []string
	if decoder.Decode(&authors) != nil || decoder.Decode(&struct{}{}) != io.EOF || len(authors) == 0 {
		return nil, false
	}
	for _, author := range authors {
		if strings.TrimSpace(author) == "" {
			return nil, false
		}
	}
	return authors, true
}

func fetchVersion(source string) (string, bool) {
	if source == "" {
		return "", false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	client := &http.Client{Timeout: 3 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
	if err != nil {
		return "", false
	}
	response, err := client.Do(request)
	if err != nil || response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		if response != nil {
			response.Body.Close()
		}
		return "", false
	}
	defer response.Body.Close()
	decoder := json.NewDecoder(io.LimitReader(response.Body, remoteBodyLimit+1))
	var payload struct {
		TagName string `json:"tag_name"`
	}
	if decoder.Decode(&payload) != nil || decoder.Decode(&struct{}{}) != io.EOF || strings.TrimSpace(payload.TagName) == "" {
		return "", false
	}
	return strings.TrimPrefix(payload.TagName, "v"), true
}
