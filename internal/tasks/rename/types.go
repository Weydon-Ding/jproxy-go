package rename

import (
	"context"
	"time"

	"jproxy-go/internal/format"
)

type Downloader string

const (
	DownloaderQBittorrent  Downloader = "qbittorrent"
	DownloaderTransmission Downloader = "transmission"
)

type Event struct {
	SourceTitle string
	Hash        string
	Downloader  Downloader
}

type Endpoint struct {
	URL    string
	APIKey string
}

type History interface {
	Fetch(context.Context, string, string, time.Time) ([]Event, error)
}

type TorrentDownloader interface {
	Rename(context.Context, string, string) error
}

type QBittorrent interface {
	TorrentDownloader
	Files(context.Context, string) ([]string, error)
	RenameFile(context.Context, string, string, string) error
}

type Config interface {
	ValueByKey(context.Context, string) (string, error)
}

type FileOptions struct {
	Enabled      bool
	SonarrFormat string
	SonarrRules  []format.Rule
}

const (
	sonarrURLKey    = "sonarrUrl"
	sonarrAPIKeyKey = "sonarrApikey"
	radarrURLKey    = "radarrUrl"
	radarrAPIKeyKey = "radarrApikey"
)
