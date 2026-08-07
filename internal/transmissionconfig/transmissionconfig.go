package transmissionconfig

import (
	"errors"
	"net/url"
	"strings"
	"unicode"
)

var (
	ErrInvalidEndpoint    = errors.New("invalid Transmission endpoint")
	ErrInvalidCredentials = errors.New("invalid Transmission credentials")
)

const (
	transmissionRPC = "/transmission/rpc"
	transmissionWeb = "/transmission/web"
	MaxValueLength  = 16 * 1024
)

// NormalizeEndpoint converts a safe Transmission base URL into its RPC endpoint.
func NormalizeEndpoint(raw string) (string, error) {
	if raw == "" {
		return "", nil
	}
	endpoint, err := ParseEndpoint(raw)
	if err != nil {
		return "", err
	}
	return endpoint.String(), nil
}

// ParseEndpoint parses and normalizes a non-empty safe Transmission endpoint.
func ParseEndpoint(raw string) (*url.URL, error) {
	if raw == "" || len(raw) > MaxValueLength || hasControl(raw) || strings.Contains(raw, "\\") {
		return nil, ErrInvalidEndpoint
	}
	parsed, err := url.Parse(raw)
	if err != nil || !isSafeEndpoint(parsed) || !isSafePath(parsed.Path, parsed.RawPath) {
		return nil, ErrInvalidEndpoint
	}
	parsed.Path = normalizedPath(parsed.Path)
	parsed.RawPath = ""
	return parsed, nil
}

// ValidateCredentials accepts Java-compatible control-character-free credentials.
func ValidateCredentials(username, password string) error {
	if len(username) > MaxValueLength || len(password) > MaxValueLength || hasControl(username) || hasControl(password) {
		return ErrInvalidCredentials
	}
	return nil
}

func isSafeEndpoint(value *url.URL) bool {
	return (value.Scheme == "http" || value.Scheme == "https") &&
		value.Host != "" &&
		value.Opaque == "" &&
		value.User == nil &&
		value.RawQuery == "" &&
		!value.ForceQuery &&
		value.Fragment == ""
}

func isSafePath(value, rawPath string) bool {
	if rawPath != "" || strings.Contains(value, "\\") || strings.Contains(value, "//") || hasDotSegment(value) {
		return false
	}
	trimmed := strings.TrimSuffix(value, "/")
	if trimmed == "" {
		return value == "" || value == "/"
	}
	if strings.HasSuffix(trimmed, "/transmission") || strings.Contains(trimmed, "/transmission/") {
		return trimmed == transmissionWeb || trimmed == transmissionRPC ||
			strings.HasSuffix(trimmed, transmissionWeb) || strings.HasSuffix(trimmed, transmissionRPC)
	}
	return true
}

func hasDotSegment(value string) bool {
	for _, segment := range strings.Split(value, "/") {
		if segment == "." || segment == ".." {
			return true
		}
	}
	return false
}

func normalizedPath(value string) string {
	trimmed := strings.TrimSuffix(value, "/")
	switch {
	case trimmed == transmissionWeb:
		return transmissionRPC
	case strings.HasSuffix(trimmed, transmissionWeb):
		return strings.TrimSuffix(trimmed, "/web") + "/rpc"
	case trimmed == transmissionRPC || strings.HasSuffix(trimmed, transmissionRPC):
		return trimmed
	case trimmed == "":
		return transmissionRPC
	default:
		return trimmed + transmissionRPC
	}
}

func hasControl(value string) bool {
	return strings.IndexFunc(value, unicode.IsControl) >= 0
}
