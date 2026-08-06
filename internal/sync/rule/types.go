package rulesync

import (
	"errors"
	"strings"
)

const (
	maxResponseBytes = 1024 * 1024
	allAuthors       = "ALL"
)

var (
	errInvalidClientOptions = errors.New("invalid rule sync client options")
	errInvalidAuthors       = errors.New("invalid rule sync authors")
)

type Domain string

const (
	Sonarr Domain = "sonarr"
	Radarr Domain = "radarr"
)

type Failure string

const (
	FailureNone           Failure = ""
	FailureRemote         Failure = "remote"
	FailureInvalidPayload Failure = "invalid_payload"
	FailureTransaction    Failure = "transaction"
)

type AuthorReport struct {
	Author    string
	Committed int
	Failure   Failure
}

type Report struct {
	Domain  Domain
	authors []AuthorReport
}

func (r Report) Authors() []AuthorReport { return append([]AuthorReport(nil), r.authors...) }

func (r Report) Succeeded() bool {
	for _, author := range r.authors {
		if author.Failure != FailureNone {
			return false
		}
	}
	return true
}

func normalizeAuthors(value string) ([]string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	if value == allAuthors {
		return []string{allAuthors}, nil
	}
	parts := strings.Split(value, ",")
	authors := make([]string, len(parts))
	for index, author := range parts {
		author = strings.TrimSpace(author)
		if author == "" || strings.Contains(author, "/") || strings.Contains(author, "\\") {
			return nil, errInvalidAuthors
		}
		authors[index] = author
	}
	return authors, nil
}
