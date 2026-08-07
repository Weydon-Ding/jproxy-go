package rename

import "context"

type LoginDownloader interface {
	Login(context.Context) error
}

type LoginService struct {
	QBittorrent  LoginDownloader
	Transmission LoginDownloader
}

func (s LoginService) Startup(ctx context.Context) {
	if s.QBittorrent != nil {
		_ = s.QBittorrent.Login(ctx)
	}
	if s.Transmission != nil {
		_ = s.Transmission.Login(ctx)
	}
}

func (s LoginService) RefreshQBittorrent(ctx context.Context) {
	if s.QBittorrent != nil {
		_ = s.QBittorrent.Login(ctx)
	}
}
