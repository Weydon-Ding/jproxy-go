package qbittorrent

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
)

func (c *Client) Files(ctx context.Context, hash string) ([]string, error) {
	if invalidArgument(hash) {
		return nil, ErrInvalidConfig
	}
	response, err := c.authenticated(ctx, http.MethodGet, "/api/v2/torrents/files?hash="+url.QueryEscape(hash), "")
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	limited := io.LimitReader(response.Body, maxResponseBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, ErrTransport
	}
	if len(data) > maxResponseBytes {
		return nil, ErrResponseTooLarge
	}
	var files []struct {
		Name string `json:"name"`
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	if decoder.Decode(&files) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return nil, ErrMalformedResponse
	}
	names := make([]string, len(files))
	for index, file := range files {
		if strings.TrimSpace(file.Name) == "" {
			return nil, ErrMalformedResponse
		}
		names[index] = file.Name
	}
	return names, nil
}

func (c *Client) Rename(ctx context.Context, hash, name string) error {
	if invalidArgument(hash) || !validRenameName(name) {
		return ErrInvalidConfig
	}
	return c.postMutation(ctx, "/api/v2/torrents/rename", url.Values{"hash": {hash}, "name": {name}})
}

func (c *Client) RenameFile(ctx context.Context, hash, oldPath, newPath string) error {
	if invalidArgument(hash) || !validFilePath(oldPath) || !validFilePath(newPath) {
		return ErrInvalidConfig
	}
	return c.postMutation(ctx, "/api/v2/torrents/renameFile", url.Values{"hash": {hash}, "oldPath": {oldPath}, "newPath": {newPath}})
}

func (c *Client) authenticated(ctx context.Context, method, path, form string) (*http.Response, error) {
	for attempt := 0; attempt < 2; attempt++ {
		cfg, saved, err := c.sessionFor(ctx)
		if err != nil {
			return nil, err
		}
		response, err := c.do(ctx, cfg, method, path, form, saved.cookie)
		if err != nil {
			return nil, err
		}
		if response.StatusCode != http.StatusUnauthorized && response.StatusCode != http.StatusForbidden {
			if response.StatusCode < 200 || response.StatusCode >= 300 {
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

func invalidArgument(value string) bool { return value == "" || containsControl(value) }

func validRenameName(value string) bool {
	return value != "" && strings.TrimSpace(value) != "" && !containsControl(value) && !strings.ContainsAny(value, `/\`) && value != "." && value != ".."
}

func validFilePath(value string) bool {
	if value == "" || containsControl(value) || strings.Contains(value, `\`) || strings.HasPrefix(value, "/") || path.Clean(value) != value {
		return false
	}
	for _, component := range strings.Split(value, "/") {
		if component == "." || component == ".." || component == "" {
			return false
		}
	}
	return true
}
