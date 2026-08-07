package transmission

import (
	"net/url"

	"jproxy-go/internal/transmissionconfig"
)

type config struct {
	endpoint           *url.URL
	username, password string
	revision           uint64
}

func (c *Client) currentConfig() (config, error) {
	snapshot := c.provider.Snapshot()
	endpoint, err := transmissionconfig.ParseEndpoint(snapshot.TransmissionURL)
	if err != nil || transmissionconfig.ValidateCredentials(snapshot.TransmissionUsername, snapshot.TransmissionPassword) != nil {
		return config{}, ErrInvalidConfig
	}
	return config{endpoint: endpoint, username: snapshot.TransmissionUsername, password: snapshot.TransmissionPassword, revision: snapshot.TransmissionRevision}, nil
}

func (c *Client) configUnchanged(cfg config) bool {
	return c.provider.Snapshot().TransmissionRevision == cfg.revision
}
