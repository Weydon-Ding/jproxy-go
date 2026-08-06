package proxy

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"
)

const transmissionProxyLimit = 8 << 20

func (s *Server) handleTransmission(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet && request.Method != http.MethodPost {
		writer.Header().Set("Allow", "GET, POST")
		writeTransmissionError(writer, http.StatusMethodNotAllowed)
		return
	}
	if !transmissionPath(request.URL, request.URL.EscapedPath()) {
		writeTransmissionError(writer, http.StatusBadRequest)
		return
	}
	endpoint, err := transmissionEndpoint(s.provider.Snapshot().TransmissionURL)
	if err != nil {
		writeTransmissionError(writer, http.StatusServiceUnavailable)
		return
	}
	body, err := readLimited(request.Body, transmissionProxyLimit)
	if err != nil {
		writeTransmissionError(writer, http.StatusRequestEntityTooLarge)
		return
	}
	timeout := s.cfg.HTTPTimeout
	if timeout <= 0 {
		timeout = time.Second
	}
	ctx, cancel := context.WithTimeout(request.Context(), timeout)
	defer cancel()
	target := *endpoint
	target.RawQuery = request.URL.RawQuery
	upstream, err := http.NewRequestWithContext(ctx, request.Method, target.String(), bytes.NewReader(body))
	if err != nil {
		writeTransmissionError(writer, http.StatusBadGateway)
		return
	}
	copyEndToEndHeaders(upstream.Header, request.Header)
	upstream.Host = ""
	client := *s.client
	client.Timeout = timeout + time.Second
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	response, err := client.Do(upstream)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			writeTransmissionError(writer, http.StatusGatewayTimeout)
		} else {
			writeTransmissionError(writer, http.StatusBadGateway)
		}
		return
	}
	defer response.Body.Close()
	if response.StatusCode >= http.StatusMultipleChoices && response.StatusCode < http.StatusBadRequest {
		writeTransmissionError(writer, http.StatusBadGateway)
		return
	}
	responseBody, err := readLimited(response.Body, transmissionProxyLimit)
	if err != nil {
		writeTransmissionError(writer, http.StatusBadGateway)
		return
	}
	copyEndToEndHeaders(writer.Header(), response.Header)
	writer.WriteHeader(response.StatusCode)
	_, _ = writer.Write(responseBody)
}

func transmissionEndpoint(raw string) (*url.URL, error) {
	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil || parsed.ForceQuery || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Path != "/transmission/rpc" || parsed.RawPath != "" || strings.IndexFunc(raw, unicode.IsControl) >= 0 {
		return nil, errors.New("invalid URL")
	}
	return parsed, nil
}

func transmissionPath(value *url.URL, escaped string) bool {
	const sonarr = "/sonarr/transmission/"
	const radarr = "/radarr/transmission/"
	suffix := strings.TrimPrefix(escaped, sonarr)
	if suffix == escaped {
		suffix = strings.TrimPrefix(escaped, radarr)
	}
	return suffix != escaped && (suffix == "transmission/rpc" || suffix == "transmission/rpc/") && !strings.Contains(value.Path, "\\") && !strings.Contains(value.Path, "/../") && !strings.Contains(value.Path, "/./") && !strings.Contains(strings.ToLower(escaped), "%2e") && strings.IndexFunc(value.Path, unicode.IsControl) < 0
}

func writeTransmissionError(writer http.ResponseWriter, status int) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_, _ = writer.Write([]byte(`{"error":"request failed"}`))
}
