package titlesync

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

const maxResponseBytes = 1024 * 1024

var errRedirect = errors.New("redirect rejected")

type HTTPError struct {
	Provider  string
	Operation string
	Status    int
	Field     string
	err       error
}

func (e *HTTPError) Error() string {
	if e.Status != 0 {
		return fmt.Sprintf("%s %s status %d", e.Provider, e.Operation, e.Status)
	}
	if e.Field != "" {
		return fmt.Sprintf("%s %s field %s", e.Provider, e.Operation, e.Field)
	}
	return fmt.Sprintf("%s %s failed", e.Provider, e.Operation)
}

func (e *HTTPError) Unwrap() error { return e.err }

type RequestClient struct {
	client  *http.Client
	timeout time.Duration
}

func newRequestClient(client *http.Client, timeout time.Duration) RequestClient {
	if client == nil {
		client = &http.Client{}
	}
	clone := *client
	clone.CheckRedirect = func(*http.Request, []*http.Request) error { return errRedirect }
	return RequestClient{client: &clone, timeout: timeout}
}

// NewRequestClient creates a bounded title-sync request client that rejects redirects.
func NewRequestClient(client *http.Client, timeout time.Duration) RequestClient {
	return newRequestClient(client, timeout)
}

func (c RequestClient) get(ctx context.Context, provider, operation string, cfg providerConfig, parts ...string) ([]byte, error) {
	requestContext, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	endpoint := *cfg.baseURL.JoinPath(parts...)
	endpoint.RawPath = ""
	query := endpoint.Query()
	query.Set("apikey", cfg.apiKey)
	endpoint.RawQuery = query.Encode()
	request, err := http.NewRequestWithContext(requestContext, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, &HTTPError{Provider: provider, Operation: operation, Field: "request", err: err}
	}
	response, err := c.client.Do(request)
	if err != nil {
		if errors.Is(requestContext.Err(), context.Canceled) || errors.Is(requestContext.Err(), context.DeadlineExceeded) {
			return nil, &HTTPError{Provider: provider, Operation: operation, err: requestContext.Err()}
		}
		if errors.Is(err, errRedirect) {
			return nil, &HTTPError{Provider: provider, Operation: operation, err: errRedirect}
		}
		return nil, &HTTPError{Provider: provider, Operation: operation}
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, &HTTPError{Provider: provider, Operation: operation, Status: response.StatusCode}
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return nil, &HTTPError{Provider: provider, Operation: operation, err: err}
	}
	if len(body) > maxResponseBytes {
		return nil, &HTTPError{Provider: provider, Operation: operation, Field: "body size"}
	}
	return body, nil
}

func decodeSingleJSON(body []byte, target any, provider, operation string) error {
	decoder := json.NewDecoder(bytes.NewReader(body))
	if err := decoder.Decode(target); err != nil {
		return &HTTPError{Provider: provider, Operation: operation, Field: "json", err: err}
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return &HTTPError{Provider: provider, Operation: operation, Field: "json"}
	}
	return nil
}
