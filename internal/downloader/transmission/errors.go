package transmission

import "errors"

var (
	ErrInvalidConfig     = errors.New("invalid Transmission configuration")
	ErrAuthentication    = errors.New("Transmission authentication failed")
	ErrUpstreamStatus    = errors.New("Transmission upstream status")
	ErrUpstreamResult    = errors.New("Transmission upstream result")
	ErrTimeout           = errors.New("Transmission request timed out")
	ErrRedirect          = errors.New("Transmission redirect rejected")
	ErrResponseTooLarge  = errors.New("Transmission response too large")
	ErrMalformedResponse = errors.New("Transmission malformed response")
	ErrTransport         = errors.New("Transmission transport failed")
	ErrInvalidPath       = errors.New("invalid Transmission file path")
	ErrUnknownTorrent    = errors.New("Transmission torrent not found")
	ErrSessionChallenge  = errors.New("Transmission session challenge failed")
	ErrRequestTooLarge   = errors.New("Transmission request too large")
)

type HTTPError struct {
	Operation string
	Category  error
	Status    int
}

func (e *HTTPError) Error() string { return "Transmission " + e.Operation + " failed" }
func (e *HTTPError) Is(target error) bool {
	return target == e.Category
}

func operationError(operation string, category error, status int) error {
	if category == nil {
		return nil
	}
	return &HTTPError{Operation: operation, Category: category, Status: status}
}
