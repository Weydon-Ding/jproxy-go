package rulesync

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"path"
	"time"
)

var errRedirect = errors.New("redirect rejected")

type ClientOptions struct {
	PrimaryURL string
	BackupURL  string
	Timeout    time.Duration
	HTTPClient *http.Client
}

type Client struct {
	primary *url.URL
	backup  *url.URL
	client  *http.Client
	timeout time.Duration
}

func NewClient(options ClientOptions) (Client, error) {
	if options.Timeout <= 0 {
		return Client{}, errInvalidClientOptions
	}
	primary, err := parseBaseURL(options.PrimaryURL)
	if err != nil {
		return Client{}, errInvalidClientOptions
	}
	backup, err := parseOptionalBaseURL(options.BackupURL)
	if err != nil {
		return Client{}, errInvalidClientOptions
	}
	client := options.HTTPClient
	if client == nil {
		client = &http.Client{}
	}
	clone := *client
	clone.CheckRedirect = func(*http.Request, []*http.Request) error { return errRedirect }
	return Client{primary: primary, backup: backup, client: &clone, timeout: options.Timeout}, nil
}

func (c Client) Authors(ctx context.Context) ([]string, error) {
	var authors []string
	if err := c.fetch(ctx, "author.json", &authors); err != nil {
		return nil, err
	}
	if len(authors) == 0 {
		return nil, errors.New("rule sync authors unavailable")
	}
	return normalizeRemoteAuthors(authors)
}

func (c Client) rules(ctx context.Context, domain Domain, author string) ([]remoteRule, error) {
	var rules *[]remoteRule
	if err := c.fetch(ctx, string(domain)+"@"+author+".json", &rules); err != nil {
		return nil, err
	}
	if rules == nil {
		return nil, errors.New("rule sync rules unavailable")
	}
	return *rules, nil
}

func (c Client) fetch(ctx context.Context, resource string, target any) error {
	for _, base := range []*url.URL{c.primary, c.backup} {
		if base == nil {
			continue
		}
		body, err := c.get(ctx, base, resource)
		if err != nil {
			continue
		}
		if err := decodeJSON(body, target); err == nil {
			return nil
		}
	}
	return errors.New("rule sync remote unavailable")
}

func (c Client) get(ctx context.Context, base *url.URL, resource string) ([]byte, error) {
	requestContext, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	endpoint := *base
	endpoint.Path = path.Join(base.Path, resource)
	request, err := http.NewRequestWithContext(requestContext, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, err
	}
	response, err := c.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, errors.New("unexpected response status")
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil || len(body) > maxResponseBytes {
		return nil, errors.New("invalid response body")
	}
	return body, nil
}

func parseBaseURL(raw string) (*url.URL, error) {
	if raw == "" {
		return nil, errors.New("empty URL")
	}
	return parseOptionalBaseURL(raw)
}

func parseOptionalBaseURL(raw string) (*url.URL, error) {
	if raw == "" {
		return nil, nil
	}
	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, errors.New("invalid URL")
	}
	return parsed, nil
}

func decodeJSON(body []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra struct{}
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("multiple JSON values")
	}
	return nil
}

func normalizeRemoteAuthors(authors []string) ([]string, error) {
	result := make([]string, len(authors))
	for index, author := range authors {
		normalized, err := normalizeAuthors(author)
		if err != nil || len(normalized) != 1 || normalized[0] == allAuthors {
			return nil, errInvalidAuthors
		}
		result[index] = normalized[0]
	}
	return result, nil
}
