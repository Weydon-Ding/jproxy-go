package qbittorrent

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
	"unicode"

	"jproxy-go/internal/runtime"
)

const maxResponseBytes = 1 << 20

type Options struct {
	Provider   runtime.Provider
	HTTPClient *http.Client
	Timeout    time.Duration
}

type Client struct {
	provider  runtime.Provider
	http      *http.Client
	sessionMu sync.Mutex
	loginMu   sync.Mutex
	session   session
}

type session struct {
	cookie   cookie
	revision uint64
}
type cookie struct{ name, value string }
type config struct {
	base               *url.URL
	username, password string
	revision           uint64
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
	c.loginMu.Lock()
	defer c.loginMu.Unlock()
	return c.login(ctx, false)
}

func (c *Client) currentConfig() (config, error) {
	snapshot := c.provider.Snapshot()
	if snapshot.QBittorrentURL == "" || snapshot.QBittorrentUsername == "" || snapshot.QBittorrentPassword == "" || containsControl(snapshot.QBittorrentURL) || containsControl(snapshot.QBittorrentUsername) || containsControl(snapshot.QBittorrentPassword) {
		return config{}, ErrInvalidConfig
	}
	parsed, err := url.Parse(snapshot.QBittorrentURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return config{}, ErrInvalidConfig
	}
	return config{base: parsed, username: snapshot.QBittorrentUsername, password: snapshot.QBittorrentPassword, revision: snapshot.QBittorrentRevision}, nil
}

func containsControl(value string) bool { return strings.IndexFunc(value, unicode.IsControl) >= 0 }

func (c *Client) sessionFor(ctx context.Context) (config, session, error) {
	cfg, err := c.currentConfig()
	if err != nil {
		return config{}, session{}, err
	}
	c.sessionMu.Lock()
	current := c.session
	c.sessionMu.Unlock()
	if current.cookie.value != "" && current.revision == cfg.revision {
		return cfg, current, nil
	}
	c.loginMu.Lock()
	defer c.loginMu.Unlock()
	if err := c.login(ctx, true); err != nil {
		return config{}, session{}, err
	}
	cfg, err = c.currentConfig()
	if err != nil {
		return config{}, session{}, err
	}
	c.sessionMu.Lock()
	current = c.session
	c.sessionMu.Unlock()
	if current.cookie.value == "" || current.revision != cfg.revision {
		return config{}, session{}, ErrAuthentication
	}
	return cfg, current, nil
}

func (c *Client) login(ctx context.Context, reuse bool) error {
	for {
		cfg, err := c.currentConfig()
		if err != nil {
			return err
		}
		if reuse {
			c.sessionMu.Lock()
			current := c.session
			c.sessionMu.Unlock()
			if current.cookie.value != "" && current.revision == cfg.revision {
				return nil
			}
		}
		values := url.Values{"username": {cfg.username}, "password": {cfg.password}}
		response, err := c.do(ctx, cfg, http.MethodPost, "/api/v2/auth/login", values.Encode(), cookie{})
		if err != nil {
			return err
		}
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			response.Body.Close()
			return operationError("login", ErrAuthentication, response.StatusCode)
		}
		var saved session
		for _, received := range response.Cookies() {
			if received.Name == "SID" && received.Value != "" {
				saved = session{cookie: cookie{name: received.Name, value: received.Value}, revision: cfg.revision}
				break
			}
			if saved.cookie.value == "" && strings.HasPrefix(received.Name, "QBT_SID_") && received.Value != "" {
				saved = session{cookie: cookie{name: received.Name, value: received.Value}, revision: cfg.revision}
			}
		}
		response.Body.Close()
		if saved.cookie.value == "" {
			return operationError("login", ErrAuthentication, response.StatusCode)
		}
		latest, configErr := c.currentConfig()
		if configErr != nil {
			return configErr
		}
		if latest.revision != cfg.revision {
			continue
		}
		c.sessionMu.Lock()
		c.session = saved
		c.sessionMu.Unlock()
		return nil
	}
}

func (c *Client) do(ctx context.Context, cfg config, method, path, form string, saved cookie) (*http.Response, error) {
	return c.doStarted(ctx, cfg, method, path, form, saved, nil)
}

func (c *Client) doStarted(ctx context.Context, cfg config, method, path, form string, saved cookie, started chan<- error) (*http.Response, error) {
	relative, err := url.Parse(path)
	if err != nil {
		notifyStart(started, ErrInvalidConfig)
		return nil, ErrInvalidConfig
	}
	requestCtx, cancel := context.WithTimeout(ctx, c.http.Timeout)
	target := *cfg.base
	target.Path = strings.TrimRight(target.Path, "/") + relative.Path
	target.RawQuery = relative.RawQuery
	var body *strings.Reader
	if form != "" {
		body = strings.NewReader(form)
	} else {
		body = strings.NewReader("")
	}
	req, err := http.NewRequestWithContext(requestCtx, method, target.String(), body)
	if err != nil {
		cancel()
		notifyStart(started, ErrInvalidConfig)
		return nil, ErrInvalidConfig
	}
	if form != "" {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if saved.value != "" {
		req.AddCookie(&http.Cookie{Name: saved.name, Value: saved.value})
	}
	if started != nil {
		req = req.WithContext(context.WithValue(req.Context(), mutationStartKey{}, started))
	}
	response, err := c.http.Do(req)
	if err != nil {
		cancel()
		notifyStart(started, ErrTransport)
		if errors.Is(ctx.Err(), context.Canceled) {
			return nil, context.Canceled
		}
		if errors.Is(requestCtx.Err(), context.DeadlineExceeded) {
			return nil, ErrTimeout
		}
		var networkError net.Error
		if errors.As(err, &networkError) && networkError.Timeout() {
			return nil, ErrTimeout
		}
		return nil, ErrTransport
	}
	if response.StatusCode >= 300 && response.StatusCode < 400 {
		response.Body.Close()
		cancel()
		return nil, ErrRedirect
	}
	response.Body = cancelOnClose{ReadCloser: response.Body, cancel: cancel}
	return response, nil
}

func notifyStart(started chan<- error, err error) {
	if started == nil {
		return
	}
	select {
	case started <- err:
	default:
	}
}

type cancelOnClose struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (body cancelOnClose) Close() error { err := body.ReadCloser.Close(); body.cancel(); return err }
