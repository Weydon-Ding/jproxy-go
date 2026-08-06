package proxy

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"
)

const qbittorrentProxyLimit = 8 << 20

func (s *Server) handleQBittorrent(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet && request.Method != http.MethodPost {
		writer.Header().Set("Allow", "GET, POST")
		writeQBittorrentError(writer, http.StatusMethodNotAllowed)
		return
	}
	path, ok := qbittorrentPath(request.URL, request.URL.EscapedPath())
	if !ok {
		writeQBittorrentError(writer, http.StatusBadRequest)
		return
	}
	base, err := qbittorrentBaseURL(s.provider.Snapshot().QBittorrentURL)
	if err != nil {
		writeQBittorrentError(writer, http.StatusServiceUnavailable)
		return
	}
	body, err := readLimited(request.Body, qbittorrentProxyLimit)
	if err != nil {
		writeQBittorrentError(writer, http.StatusRequestEntityTooLarge)
		return
	}
	timeout := s.cfg.HTTPTimeout
	if timeout <= 0 {
		timeout = time.Second
	}
	ctx, cancel := context.WithTimeout(request.Context(), timeout)
	defer cancel()
	target := *base
	target.Path = strings.TrimRight(target.Path, "/") + path
	target.RawQuery = request.URL.RawQuery
	upstream, err := http.NewRequestWithContext(ctx, request.Method, target.String(), bytes.NewReader(body))
	if err != nil {
		writeQBittorrentError(writer, http.StatusBadGateway)
		return
	}
	copyEndToEndHeaders(upstream.Header, request.Header)
	upstream.Host = ""
	client := *s.client
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	response, err := client.Do(upstream)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			writeQBittorrentError(writer, http.StatusGatewayTimeout)
		} else {
			writeQBittorrentError(writer, http.StatusBadGateway)
		}
		return
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 && response.StatusCode < 400 {
		writeQBittorrentError(writer, http.StatusBadGateway)
		return
	}
	responseBody, err := readLimited(response.Body, qbittorrentProxyLimit)
	if err != nil {
		writeQBittorrentError(writer, http.StatusBadGateway)
		return
	}
	copyEndToEndHeaders(writer.Header(), response.Header)
	writer.WriteHeader(response.StatusCode)
	_, _ = writer.Write(responseBody)
}

func qbittorrentBaseURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || strings.IndexFunc(raw, unicode.IsControl) >= 0 {
		return nil, errors.New("invalid URL")
	}
	return parsed, nil
}

func qbittorrentPath(value *url.URL, escaped string) (string, bool) {
	const sonarr = "/sonarr/qbittorrent/"
	const radarr = "/radarr/qbittorrent/"
	prefixed := strings.TrimPrefix(escaped, sonarr)
	if prefixed == escaped {
		prefixed = strings.TrimPrefix(escaped, radarr)
	}
	if prefixed == escaped || prefixed == "" || strings.Contains(prefixed, "\\") || !strings.HasPrefix(prefixed, "api/v2/") || strings.Contains(value.Path, "/../") || strings.Contains(value.Path, "/./") || strings.Contains(strings.ToLower(escaped), "%2e") {
		return "", false
	}
	return "/" + prefixed, true
}

func readLimited(body io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(body, limit+1))
	if err != nil || int64(len(data)) > limit {
		return nil, errors.New("body limit")
	}
	return data, nil
}

func copyEndToEndHeaders(destination, source http.Header) {
	connection := map[string]bool{}
	for _, value := range source.Values("Connection") {
		for _, name := range strings.Split(value, ",") {
			connection[http.CanonicalHeaderKey(strings.TrimSpace(name))] = true
		}
	}
	for name, values := range source {
		canonical := http.CanonicalHeaderKey(name)
		if canonical == "Host" || connection[canonical] || hopHeader(canonical) {
			continue
		}
		for _, value := range values {
			destination.Add(canonical, value)
		}
	}
}

func hopHeader(name string) bool {
	switch name {
	case "Connection", "Keep-Alive", "Proxy-Authenticate", "Proxy-Authorization", "Proxy-Connection", "Te", "Trailer", "Transfer-Encoding", "Upgrade":
		return true
	}
	return false
}
func writeQBittorrentError(writer http.ResponseWriter, status int) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_, _ = writer.Write([]byte(`{"error":"request failed"}`))
}
