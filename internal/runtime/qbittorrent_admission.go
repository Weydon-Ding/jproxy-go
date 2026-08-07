package runtime

import "errors"

var ErrQBittorrentRevisionStale = errors.New("qBittorrent configuration revision is stale")

// QBittorrentMutationAdmission atomically verifies the qBittorrent
// configuration revision and begins a mutation request.
type QBittorrentMutationAdmission interface {
	AdmitQBittorrentMutation(uint64, func()) error
}

// AdmitQBittorrentMutation holds the publication lock only while it compares
// the revision and synchronously starts the request. Response I/O must happen
// after the callback returns, outside this lock.
func (p *provider) AdmitQBittorrentMutation(revision uint64, begin func()) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.value.Load().QBittorrentRevision != revision {
		return ErrQBittorrentRevisionStale
	}
	begin()
	return nil
}
