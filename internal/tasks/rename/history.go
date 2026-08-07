package rename

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"
)

const maxHistoryBytes = 1 << 20

var (
	ErrInvalidEndpoint  = errors.New("invalid history endpoint")
	ErrHistoryRedirect  = errors.New("history redirect")
	ErrHistoryStatus    = errors.New("history upstream status")
	ErrHistoryBody      = errors.New("invalid history body")
	ErrHistoryTimeout   = errors.New("history timeout")
	ErrHistoryTransport = errors.New("history transport")
)

type HistoryClientOptions struct {
	HTTPClient *http.Client
	Timeout    time.Duration
}

type HistoryClient struct{ http *http.Client }

func NewHistoryClient(options HistoryClientOptions) (*HistoryClient, error) {
	if options.Timeout <= 0 {
		return nil, ErrInvalidEndpoint
	}
	client := http.DefaultClient
	if options.HTTPClient != nil {
		client = options.HTTPClient
	}
	clone := *client
	clone.Timeout = options.Timeout
	clone.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &HistoryClient{http: &clone}, nil
}

func (c *HistoryClient) Fetch(ctx context.Context, endpoint, apiKey string, since time.Time) ([]Event, error) {
	target, err := historyURL(endpoint, apiKey, since)
	if err != nil {
		return nil, err
	}
	requestCtx, cancel := context.WithTimeout(ctx, c.http.Timeout)
	defer cancel()
	request, err := http.NewRequestWithContext(requestCtx, http.MethodGet, target.String(), nil)
	if err != nil {
		return nil, ErrInvalidEndpoint
	}
	response, err := c.http.Do(request)
	if err != nil {
		return nil, historyRequestError(ctx, requestCtx, err)
	}
	defer response.Body.Close()
	if response.StatusCode >= http.StatusMultipleChoices && response.StatusCode < http.StatusBadRequest {
		return nil, ErrHistoryRedirect
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, ErrHistoryStatus
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxHistoryBytes+1))
	if err != nil || len(body) > maxHistoryBytes {
		return nil, ErrHistoryBody
	}
	return decodeHistory(body)
}

func historyURL(raw, apiKey string, since time.Time) (*url.URL, error) {
	if strings.TrimSpace(apiKey) == "" || hasControl(raw) || hasControl(apiKey) {
		return nil, ErrInvalidEndpoint
	}
	base, err := url.Parse(raw)
	if err != nil || (base.Scheme != "http" && base.Scheme != "https") || base.Host == "" || base.User != nil || base.RawQuery != "" || base.Fragment != "" {
		return nil, ErrInvalidEndpoint
	}
	base.Path = strings.TrimRight(base.Path, "/") + "/api/v3/history/since"
	base.RawQuery = url.Values{"eventType": {"1"}, "date": {since.UTC().Format("2006-01-02T15:04:05.000Z")}, "apikey": {apiKey}}.Encode()
	return base, nil
}

func decodeHistory(body []byte) ([]Event, error) {
	var rows []struct {
		SourceTitle string `json:"sourceTitle"`
		DownloadID  string `json:"downloadId"`
		Data        struct {
			TorrentInfoHash string `json:"torrentInfoHash"`
			DownloadClient  string `json:"downloadClient"`
		} `json:"data"`
	}
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	if decoder.Decode(&rows) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return nil, ErrHistoryBody
	}
	events := make([]Event, 0, len(rows))
	for _, row := range rows {
		hash := strings.TrimSpace(row.DownloadID)
		if hash == "" {
			hash = strings.TrimSpace(row.Data.TorrentInfoHash)
		} else {
			hash = strings.ToLower(hash)
		}
		if strings.TrimSpace(row.SourceTitle) == "" || hash == "" || hasControl(row.SourceTitle) || hasControl(hash) {
			continue
		}
		downloadClient := DownloaderQBittorrent
		if strings.EqualFold(row.Data.DownloadClient, "Transmission") {
			downloadClient = DownloaderTransmission
		}
		events = append(events, Event{SourceTitle: row.SourceTitle, Hash: hash, Downloader: downloadClient})
	}
	return events, nil
}

func historyRequestError(parent, request context.Context, err error) error {
	if errors.Is(parent.Err(), context.Canceled) {
		return context.Canceled
	}
	if errors.Is(request.Err(), context.DeadlineExceeded) {
		return ErrHistoryTimeout
	}
	var networkErr net.Error
	if errors.As(err, &networkErr) && networkErr.Timeout() {
		return ErrHistoryTimeout
	}
	return ErrHistoryTransport
}

func hasControl(value string) bool { return strings.IndexFunc(value, unicode.IsControl) >= 0 }
