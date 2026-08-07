package qbittorrent

import (
	"context"
	"errors"
	"net/http"
	"net/url"

	"jproxy-go/internal/runtime"
)

type requestResult struct {
	response *http.Response
	err      error
}

func (c *Client) postMutation(ctx context.Context, path string, values url.Values) error {
	response, err := c.authenticatedMutation(ctx, http.MethodPost, path, values.Encode())
	if err != nil {
		return err
	}
	return response.Body.Close()
}

func (c *Client) authenticatedMutation(ctx context.Context, method, path, form string) (*http.Response, error) {
	for attempt := 0; attempt < 2; attempt++ {
		cfg, saved, err := c.sessionFor(ctx)
		if err != nil {
			return nil, err
		}
		response, err := c.admittedDo(ctx, cfg, method, path, form, saved.cookie)
		if err != nil {
			return nil, err
		}
		if response.StatusCode != http.StatusUnauthorized && response.StatusCode != http.StatusForbidden {
			if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
				response.Body.Close()
				return nil, operationError("request", ErrUpstreamStatus, response.StatusCode)
			}
			return response, nil
		}
		response.Body.Close()
		c.sessionMu.Lock()
		if c.session.revision == saved.revision && c.session.cookie.value == saved.cookie.value {
			c.session = session{}
		}
		c.sessionMu.Unlock()
		if attempt == 1 {
			return nil, operationError("request", ErrAuthentication, response.StatusCode)
		}
		c.loginMu.Lock()
		err = c.login(ctx, true)
		c.loginMu.Unlock()
		if err != nil {
			return nil, err
		}
	}
	return nil, ErrAuthentication
}

func (c *Client) admittedDo(ctx context.Context, cfg config, method, path, form string, saved cookie) (*http.Response, error) {
	admission, available := c.provider.(runtime.QBittorrentMutationAdmission)
	if !available {
		return nil, ErrConfigChanged
	}
	started := make(chan error, 1)
	result := make(chan requestResult, 1)
	err := admission.AdmitQBittorrentMutation(cfg.revision, func() {
		go func() {
			response, requestErr := c.doStarted(ctx, cfg, method, path, form, saved, started)
			result <- requestResult{response: response, err: requestErr}
		}()
		<-started
	})
	if errors.Is(err, runtime.ErrQBittorrentRevisionStale) {
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
