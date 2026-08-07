package transmission

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"

	"jproxy-go/internal/runtime"
)

type requestResult struct {
	response *http.Response
	err      error
}

func (c *Client) callMutation(ctx context.Context, cfg config, method string, arguments requestArguments, target responseArguments) error {
	body, err := marshalRequest(method, arguments)
	if err != nil {
		return operationError(method, err, 0)
	}
	saved := c.sessionFor(cfg.revision)
	response, err := c.admittedRequest(ctx, cfg, body, saved)
	if err != nil {
		return operationError(method, err, 0)
	}
	if response.StatusCode == http.StatusConflict {
		challenge := response.Header.Get("X-Transmission-Session-Id")
		response.Body.Close()
		if strings.TrimSpace(challenge) == "" {
			return operationError(method, ErrSessionChallenge, http.StatusConflict)
		}
		c.compareAndSwapSession(saved, session{id: challenge, revision: cfg.revision})
		response, err = c.admittedRequest(ctx, cfg, body, session{id: challenge, revision: cfg.revision})
		if err != nil {
			return operationError(method, err, 0)
		}
		if response.StatusCode == http.StatusConflict {
			response.Body.Close()
			return operationError(method, ErrSessionChallenge, http.StatusConflict)
		}
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return operationError(method, ErrAuthentication, response.StatusCode)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return operationError(method, ErrUpstreamStatus, response.StatusCode)
	}
	responseBody, readErr := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if readErr != nil {
		return operationError(method, classifyReadError(ctx, response.Body), response.StatusCode)
	}
	if len(responseBody) > maxResponseBytes {
		return operationError(method, ErrResponseTooLarge, response.StatusCode)
	}
	return operationError(method, decodeResponse(responseBody, target), response.StatusCode)
}

func (c *Client) admittedRequest(ctx context.Context, cfg config, body []byte, saved session) (*http.Response, error) {
	admission, available := c.provider.(runtime.TransmissionMutationAdmission)
	if !available {
		return nil, ErrConfigChanged
	}
	started := make(chan error, 1)
	result := make(chan requestResult, 1)
	err := admission.AdmitTransmissionMutation(cfg.revision, func() {
		go func() {
			response, requestErr := c.requestStarted(ctx, cfg, body, saved, started)
			result <- requestResult{response: response, err: requestErr}
		}()
		<-started
	})
	if errors.Is(err, runtime.ErrTransmissionRevisionStale) {
		return nil, ErrConfigChanged
	}
	if err != nil {
		return nil, err
	}
	select {
	case completed := <-result:
		return completed.response, completed.err
	case <-ctx.Done():
		go func() {
			completed := <-result
			if completed.response != nil {
				completed.response.Body.Close()
			}
		}()
		return nil, ctx.Err()
	}
}
