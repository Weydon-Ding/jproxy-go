package tmdb

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"

	"jproxy-go/internal/store/sqlite"
)

const maxResponseBytes = 1024 * 1024

var errRedirect = errors.New("redirect rejected")
var errInvalidConfig = errors.New("invalid TMDB configuration")

type HTTPError struct {
	Status int
	Field  string
	err    error
}

func (errorValue *HTTPError) Error() string {
	if errorValue.Status != 0 {
		return fmt.Sprintf("tmdb find status %d", errorValue.Status)
	}
	if errorValue.Field != "" {
		return fmt.Sprintf("tmdb find field %s", errorValue.Field)
	}
	return "tmdb find failed"
}

func (errorValue *HTTPError) Unwrap() error { return errorValue.err }

type Config struct {
	BaseURL string
	APIKey  string
}

type Alias struct {
	TVDBID   int64
	TMDBID   *int64
	Language string
	Title    string
}

type Client struct {
	client  *http.Client
	timeout time.Duration
}

func NewClient(client *http.Client, timeout time.Duration) Client {
	if client == nil {
		client = &http.Client{}
	}
	clone := *client
	clone.CheckRedirect = func(*http.Request, []*http.Request) error { return errRedirect }
	return Client{client: &clone, timeout: timeout}
}

func (client Client) Find(ctx context.Context, config Config, tvdbID int64, language string) (Alias, bool, error) {
	endpoint, err := findEndpoint(config, tvdbID, language)
	if err != nil {
		return Alias{}, false, err
	}
	requestContext, cancel := context.WithTimeout(ctx, client.timeout)
	defer cancel()
	request, err := http.NewRequestWithContext(requestContext, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return Alias{}, false, &HTTPError{Field: "request", err: err}
	}
	response, err := client.client.Do(request)
	if err != nil {
		return Alias{}, false, client.requestError(requestContext, err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return Alias{}, false, &HTTPError{Status: response.StatusCode}
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return Alias{}, false, &HTTPError{err: err}
	}
	if len(body) > maxResponseBytes {
		return Alias{}, false, &HTTPError{Field: "body size"}
	}
	return decodeAlias(body, tvdbID, language)
}

func (client Client) requestError(requestContext context.Context, err error) error {
	if errors.Is(requestContext.Err(), context.Canceled) || errors.Is(err, context.Canceled) {
		return &HTTPError{err: context.Canceled}
	}
	if errors.Is(requestContext.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
		return &HTTPError{err: context.DeadlineExceeded}
	}
	if errors.Is(err, errRedirect) {
		return &HTTPError{err: errRedirect}
	}
	return &HTTPError{}
}

func findEndpoint(config Config, tvdbID int64, language string) (*url.URL, error) {
	if err := javaInteger(tvdbID); err != nil {
		return nil, err
	}
	if config.APIKey == "" || containsControl(config.APIKey) || language == "" || containsControl(language) {
		return nil, errInvalidConfig
	}
	baseURL, err := url.Parse(config.BaseURL)
	if err != nil || baseURL.Scheme != "http" && baseURL.Scheme != "https" || baseURL.Host == "" || baseURL.User != nil || baseURL.RawQuery != "" || baseURL.Fragment != "" || containsControl(config.BaseURL) {
		return nil, errInvalidConfig
	}
	endpoint := baseURL.JoinPath("3", "find", fmt.Sprint(tvdbID))
	query := endpoint.Query()
	query.Set("api_key", config.APIKey)
	query.Set("language", language)
	query.Set("external_source", "tvdb_id")
	endpoint.RawQuery = query.Encode()
	return endpoint, nil
}

func decodeAlias(body []byte, tvdbID int64, language string) (Alias, bool, error) {
	var response struct {
		TVResults json.RawMessage `json:"tv_results"`
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	if err := decoder.Decode(&response); err != nil {
		return Alias{}, false, &HTTPError{Field: "json", err: err}
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return Alias{}, false, &HTTPError{Field: "json"}
	}
	if response.TVResults == nil {
		return Alias{}, false, &HTTPError{Field: "tv_results"}
	}
	var results []struct {
		ID   *int64 `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(response.TVResults, &results); err != nil {
		return Alias{}, false, &HTTPError{Field: "tv_results", err: err}
	}
	if results == nil {
		return Alias{}, false, &HTTPError{Field: "tv_results"}
	}
	if len(results) == 0 {
		return Alias{}, false, nil
	}
	if results[0].ID == nil || results[0].Name == "" {
		return Alias{}, false, &HTTPError{Field: "tv_results"}
	}
	if err := javaInteger(*results[0].ID); err != nil {
		return Alias{}, false, &HTTPError{Field: "tv_results", err: err}
	}
	return Alias{TVDBID: tvdbID, TMDBID: results[0].ID, Language: language, Title: results[0].Name}, true, nil
}

func javaInteger(value int64) error {
	if value < -2147483648 || value > 2147483647 {
		return fmt.Errorf("%d: %w", value, sqlite.ErrJavaIntegerRange)
	}
	return nil
}

func containsControl(value string) bool { return strings.IndexFunc(value, unicode.IsControl) >= 0 }
