package transmissionconfig

import (
	"errors"
	"strings"
	"testing"
)

func TestNormalizeEndpoint_rejectsOversizedEndpointWithoutLeakingInput(t *testing.T) {
	// Given
	canary := "transmission-endpoint-canary"
	raw := "https://transmission.example/" + strings.Repeat(canary, 1024)

	// When
	_, err := NormalizeEndpoint(raw)

	// Then
	if !errors.Is(err, ErrInvalidEndpoint) {
		t.Fatalf("NormalizeEndpoint() error = %v, want ErrInvalidEndpoint", err)
	}
	assertRedacted(t, err, canary)
}

func TestNormalizeEndpoint_normalizesValidEndpointForms(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "empty endpoint remains empty", raw: "", want: ""},
		{name: "base URL", raw: "https://transmission.example", want: "https://transmission.example/transmission/rpc"},
		{name: "base URL trailing slash", raw: "https://transmission.example/", want: "https://transmission.example/transmission/rpc"},
		{name: "web endpoint", raw: "https://transmission.example/transmission/web", want: "https://transmission.example/transmission/rpc"},
		{name: "web endpoint trailing slash", raw: "https://transmission.example/transmission/web/", want: "https://transmission.example/transmission/rpc"},
		{name: "RPC endpoint", raw: "https://transmission.example/transmission/rpc", want: "https://transmission.example/transmission/rpc"},
		{name: "RPC endpoint trailing slash", raw: "https://transmission.example/transmission/rpc/", want: "https://transmission.example/transmission/rpc"},
		{name: "safe prefix", raw: "https://transmission.example/proxy", want: "https://transmission.example/proxy/transmission/rpc"},
		{name: "safe prefix web endpoint", raw: "https://transmission.example/proxy/transmission/web", want: "https://transmission.example/proxy/transmission/rpc"},
		{name: "safe prefix RPC endpoint", raw: "https://transmission.example/proxy/transmission/rpc", want: "https://transmission.example/proxy/transmission/rpc"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given
			raw := test.raw

			// When
			got, err := NormalizeEndpoint(raw)

			// Then
			if err != nil {
				t.Fatalf("NormalizeEndpoint() error = %v", err)
			}
			if got != test.want {
				t.Fatalf("NormalizeEndpoint() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestNormalizeEndpoint_rejectsUnsafeEndpointForms(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{name: "whitespace is not trimmed", raw: " https://transmission.example"},
		{name: "non HTTP scheme", raw: "ftp://transmission.example"},
		{name: "missing host", raw: "https:///proxy"},
		{name: "userinfo", raw: "https://user:password@transmission.example"},
		{name: "query", raw: "https://transmission.example/?token=secret"},
		{name: "force query", raw: "https://transmission.example/?"},
		{name: "fragment", raw: "https://transmission.example/#secret"},
		{name: "opaque URL", raw: "https:transmission.example"},
		{name: "escaped path", raw: "https://transmission.example/proxy%2Fchild"},
		{name: "backslash", raw: "https://transmission.example/proxy\\child"},
		{name: "control character", raw: "https://transmission.example/proxy\nchild"},
		{name: "dot segment", raw: "https://transmission.example/proxy/./child"},
		{name: "dot dot segment", raw: "https://transmission.example/proxy/../child"},
		{name: "repeated separator", raw: "https://transmission.example/proxy//child"},
		{name: "ambiguous web suffix", raw: "https://transmission.example/proxy/transmission/web/child"},
		{name: "ambiguous RPC suffix", raw: "https://transmission.example/proxy/transmission/rpc/child"},
		{name: "non terminal transmission", raw: "https://transmission.example/proxy/transmission/child"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given
			raw := test.raw

			// When
			_, err := NormalizeEndpoint(raw)

			// Then
			if !errors.Is(err, ErrInvalidEndpoint) {
				t.Fatalf("NormalizeEndpoint() error = %v, want ErrInvalidEndpoint", err)
			}
			assertRedacted(t, err, raw)
		})
	}
}

func TestParseEndpoint_rejectsEmptyAndReturnsNormalizedURL(t *testing.T) {
	// Given
	empty := ""

	// When
	_, err := ParseEndpoint(empty)

	// Then
	if !errors.Is(err, ErrInvalidEndpoint) {
		t.Fatalf("ParseEndpoint(\"\") error = %v, want ErrInvalidEndpoint", err)
	}

	// Given
	raw := "https://transmission.example/prefix/transmission/web/"

	// When
	parsed, err := ParseEndpoint(raw)

	// Then
	if err != nil {
		t.Fatalf("ParseEndpoint() error = %v", err)
	}
	if got, want := parsed.String(), "https://transmission.example/prefix/transmission/rpc"; got != want {
		t.Fatalf("ParseEndpoint() = %q, want %q", got, want)
	}
}

func TestValidateCredentials_acceptsJavaCompatibleCredentialShapes(t *testing.T) {
	tests := []struct {
		name     string
		username string
		password string
	}{
		{name: "anonymous", username: "", password: ""},
		{name: "complete credentials", username: "alice", password: "secret"},
		{name: "password without username", username: "", password: "secret"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given
			username, password := test.username, test.password

			// When
			err := ValidateCredentials(username, password)

			// Then
			if err != nil {
				t.Fatalf("ValidateCredentials() error = %v", err)
			}
		})
	}
}

func TestValidateCredentials_rejectsControlCredentialsWithoutLeakingSecrets(t *testing.T) {
	tests := []struct {
		name     string
		username string
		password string
	}{
		{name: "username control", username: "alice\nadmin", password: "secret"},
		{name: "password control", username: "alice", password: "secret\x7f"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given
			username, password := test.username, test.password

			// When
			err := ValidateCredentials(username, password)

			// Then
			if !errors.Is(err, ErrInvalidCredentials) {
				t.Fatalf("ValidateCredentials() error = %v, want ErrInvalidCredentials", err)
			}
			assertRedacted(t, err, username, password)
		})
	}
}

func TestValidateCredentials_rejectsOversizedCredentialsWithoutLeakingSecrets(t *testing.T) {
	for _, test := range []struct {
		name     string
		username string
		password string
	}{
		{name: "username", username: strings.Repeat("transmission-username-canary", 1024), password: "password"},
		{name: "password", username: "username", password: strings.Repeat("transmission-password-canary", 1024)},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Given
			username, password := test.username, test.password

			// When
			err := ValidateCredentials(username, password)

			// Then
			if !errors.Is(err, ErrInvalidCredentials) {
				t.Fatalf("ValidateCredentials() error = %v, want ErrInvalidCredentials", err)
			}
			assertRedacted(t, err, username, password, "transmission-username-canary", "transmission-password-canary")
		})
	}
}

func TestErrors_doNotExposeEndpointOrCredentialCanaries(t *testing.T) {
	// Given
	endpointCanary := "https://alice:secret@private.example/path?token=endpoint-canary"
	usernameCanary := "credential-user-canary"
	passwordCanary := "credential-password-canary"

	// When
	_, endpointErr := NormalizeEndpoint(endpointCanary)
	credentialErr := ValidateCredentials(usernameCanary+"\n", passwordCanary)

	// Then
	assertRedacted(t, endpointErr, endpointCanary, "alice", "secret", "endpoint-canary")
	assertRedacted(t, credentialErr, usernameCanary, passwordCanary)
}

func assertRedacted(t *testing.T, err error, canaries ...string) {
	t.Helper()
	if err == nil {
		t.Fatal("error = nil, want a redacted error")
	}
	for _, canary := range canaries {
		if canary != "" && strings.Contains(err.Error(), canary) {
			t.Fatalf("error %q exposed canary %q", err, canary)
		}
	}
}
