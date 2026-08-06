package qbittorrent

import "errors"

var (
	ErrInvalidConfig     = errors.New("invalid qBittorrent configuration")
	ErrAuthentication    = errors.New("qBittorrent authentication failed")
	ErrUpstreamStatus    = errors.New("qBittorrent upstream status")
	ErrTimeout           = errors.New("qBittorrent request timed out")
	ErrRedirect          = errors.New("qBittorrent redirect rejected")
	ErrResponseTooLarge  = errors.New("qBittorrent response too large")
	ErrMalformedResponse = errors.New("qBittorrent malformed response")
	ErrTransport         = errors.New("qBittorrent transport failed")
)

type HTTPError struct {
	Operation string
	Category  error
	Status    int
}

func (e *HTTPError) Error() string        { return "qBittorrent " + e.Operation + " failed" }
func (e *HTTPError) Is(target error) bool { return target == e.Category }

func operationError(operation string, category error, status int) error {
	return &HTTPError{Operation: operation, Category: category, Status: status}
}
