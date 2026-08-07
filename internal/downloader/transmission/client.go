package transmission

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"jproxy-go/internal/runtime"
)

type Options struct {
	Provider   runtime.Provider
	HTTPClient *http.Client
	Timeout    time.Duration
}

type Client struct {
	provider  runtime.Provider
	http      *http.Client
	sessionMu sync.Mutex
	session   session
}

type session struct {
	id       string
	revision uint64
}

func New(options Options) (*Client, error) {
	if options.Provider == nil || options.Timeout <= 0 {
		return nil, ErrInvalidConfig
	}
	client := http.DefaultClient
	if options.HTTPClient != nil {
		client = options.HTTPClient
	}
	clone := *client
	clone.Timeout = options.Timeout
	clone.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	transport := client.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	clone.Transport = signalingRoundTripper{next: transport}
	return &Client{provider: options.Provider, http: &clone}, nil
}

func (c *Client) Login(ctx context.Context) error {
	return c.callCurrent(ctx, "session-get", sessionGetArguments{Fields: []string{"version"}}, &sessionGetResponse{})
}

func (c *Client) callCurrent(ctx context.Context, method string, arguments requestArguments, target responseArguments) error {
	cfg, err := c.currentConfig()
	if err != nil {
		return err
	}
	return c.call(ctx, cfg, method, arguments, target)
}

func (c *Client) call(ctx context.Context, cfg config, method string, arguments requestArguments, target responseArguments) error {
	body, err := marshalRequest(method, arguments)
	if err != nil {
		return operationError(method, err, 0)
	}
	saved := c.sessionFor(cfg.revision)
	response, requestErr := c.request(ctx, cfg, body, saved)
	if requestErr != nil {
		return operationError(method, requestErr, 0)
	}
	if response.StatusCode == http.StatusConflict {
		challenge := response.Header.Get("X-Transmission-Session-Id")
		response.Body.Close()
		if strings.TrimSpace(challenge) == "" {
			return operationError(method, ErrSessionChallenge, http.StatusConflict)
		}
		if !c.configUnchanged(cfg) {
			return operationError(method, ErrConfigChanged, http.StatusConflict)
		}
		c.compareAndSwapSession(saved, session{id: challenge, revision: cfg.revision})
		response, requestErr = c.request(ctx, cfg, body, session{id: challenge, revision: cfg.revision})
		if requestErr != nil {
			return operationError(method, requestErr, 0)
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

func (c *Client) sessionFor(revision uint64) session {
	c.sessionMu.Lock()
	defer c.sessionMu.Unlock()
	if c.session.revision == revision {
		return c.session
	}
	return session{}
}

func (c *Client) compareAndSwapSession(current, next session) {
	c.sessionMu.Lock()
	if c.session == current {
		c.session = next
	}
	c.sessionMu.Unlock()
}

func (c *Client) request(ctx context.Context, cfg config, body []byte, saved session) (*http.Response, error) {
	return c.requestStarted(ctx, cfg, body, saved, nil)
}

func (c *Client) requestStarted(ctx context.Context, cfg config, body []byte, saved session, started chan<- error) (*http.Response, error) {
	requestCtx, cancel := context.WithTimeout(ctx, c.http.Timeout)
	request, err := http.NewRequestWithContext(requestCtx, http.MethodPost, cfg.endpoint.String(), bytes.NewReader(body))
	if err != nil {
		cancel()
		if started != nil {
			started <- ErrInvalidConfig
		}
		return nil, ErrInvalidConfig
	}
	request.Header.Set("Content-Type", "application/json")
	if cfg.username != "" {
		request.SetBasicAuth(cfg.username, cfg.password)
	}
	if saved.id != "" {
		request.Header.Set("X-Transmission-Session-Id", saved.id)
	}
	if started != nil {
		request = request.WithContext(context.WithValue(request.Context(), mutationStartKey{}, started))
	}
	response, err := c.http.Do(request)
	if err != nil {
		cancel()
		if errors.Is(ctx.Err(), context.Canceled) {
			return nil, context.Canceled
		}
		if errors.Is(requestCtx.Err(), context.DeadlineExceeded) {
			return nil, ErrTimeout
		}
		if errors.Is(err, http.ErrUseLastResponse) {
			return nil, ErrRedirect
		}
		var networkErr net.Error
		if errors.As(err, &networkErr) && networkErr.Timeout() {
			return nil, ErrTimeout
		}
		return nil, ErrTransport
	}
	if response.StatusCode >= http.StatusMultipleChoices && response.StatusCode < http.StatusBadRequest {
		response.Body.Close()
		cancel()
		return nil, ErrRedirect
	}
	response.Body = cancelOnClose{ReadCloser: response.Body, cancel: cancel, requestCtx: requestCtx}
	return response, nil
}

type cancelOnClose struct {
	io.ReadCloser
	cancel     context.CancelFunc
	requestCtx context.Context
}

func (body cancelOnClose) Close() error {
	err := body.ReadCloser.Close()
	body.cancel()
	return err
}

func classifyReadError(ctx context.Context, body io.ReadCloser) error {
	if errors.Is(ctx.Err(), context.Canceled) {
		return context.Canceled
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return ErrTimeout
	}
	if tracked, ok := body.(cancelOnClose); ok && errors.Is(tracked.requestCtx.Err(), context.DeadlineExceeded) {
		return ErrTimeout
	}
	return ErrTransport
}
