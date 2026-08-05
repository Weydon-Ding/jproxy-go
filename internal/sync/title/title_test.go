package titlesync

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"jproxy-go/internal/store/sqlite"
)

func TestConfigSource_loadSonarr_readsCurrentActiveRows_whenValuesChange(t *testing.T) {
	// Given
	key := "first-secret"
	rows := []sqlite.SystemConfig{activeConfig("sonarrUrl", "http://example.test/prefix"), activeConfigPointer("sonarrApikey", &key), activeConfig("cleanTitleRegex", `2024`)}
	source := NewConfigSource(configListFake{rows: &rows})

	// When
	first, err := source.loadSonarr(context.Background())
	rows[0] = activeConfig("sonarrUrl", "https://example.test/changed")
	second, secondErr := source.loadSonarr(context.Background())

	// Then
	if err != nil || secondErr != nil || first.baseURL.String() == second.baseURL.String() || second.baseURL.Path != "/changed" {
		t.Fatalf("expected independent current config reads: %v %v", err, secondErr)
	}
}

func TestConfigSource_loadSonarr_rejectsInvalidAndNeverReturnsSecret(t *testing.T) {
	// Given
	secret := "canary-secret"
	rows := []sqlite.SystemConfig{activeConfig("sonarrUrl", "http://example.test?a=b"), activeConfigPointer("sonarrApikey", &secret), activeConfig("cleanTitleRegex", "[")}

	// When
	_, err := NewConfigSource(configListFake{rows: &rows}).loadSonarr(context.Background())

	// Then
	if err == nil || strings.Contains(err.Error(), secret) {
		t.Fatalf("error must be redacted: %v", err)
	}
}

func TestConfigSource_loadRadarr_readsProviderSpecificValues_whenRowsAreActive(t *testing.T) {
	// Given
	rows := []sqlite.SystemConfig{activeConfig("radarrUrl", "https://example.test/radarr"), activeConfig("radarrApikey", "radarr-secret"), activeConfig("cleanTitleRegex", "")}
	source := NewConfigSource(configListFake{rows: &rows})

	// When
	config, err := source.loadRadarr(context.Background())

	// Then
	if err != nil || config.baseURL.Path != "/radarr" || config.apiKey != "radarr-secret" {
		t.Fatalf("unexpected Radarr config: %v", err)
	}
}

func TestSonarrClient_fetch_usesPathPrefixAndEncodedKey_whenUpstreamSucceeds(t *testing.T) {
	// Given
	var gotPath, gotKey string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		gotPath, gotKey = request.URL.Path, request.URL.Query().Get("apikey")
		_, _ = writer.Write([]byte(`[{"id":1,"tvdbId":2,"title":"Main","titleSlug":"slug","monitored":true,"alternateTitles":[]}]`))
	}))
	defer server.Close()
	config, err := providerConfigFor(server.URL+"/base", "a+b&secret")
	if err != nil {
		t.Fatal(err)
	}
	client := NewSonarrClient(newRequestClient(nil, time.Second))

	// When
	values, err := client.Fetch(context.Background(), config)

	// Then
	if err != nil || len(values) != 1 || gotPath != "/base/api/v3/series" || gotKey != "a+b&secret" {
		t.Fatalf("unexpected response/path/key: %#v %q %q %v", values, gotPath, gotKey, err)
	}
}

func TestRadarrClient_fetch_usesMovieEndpoint_whenUpstreamSucceeds(t *testing.T) {
	// Given
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		gotPath = request.URL.Path
		_, _ = writer.Write([]byte(`[{"id":1,"tmdbId":2,"title":"Main","path":"/movies/Main (2024)","cleanTitle":"main","originalTitle":"Original","year":2024,"monitored":false,"alternateTitles":[]}]`))
	}))
	defer server.Close()
	config, err := providerConfigFor(server.URL+"/base", "secret")
	if err != nil {
		t.Fatal(err)
	}
	client := NewRadarrClient(newRequestClient(nil, time.Second))

	// When
	values, err := client.Fetch(context.Background(), config)

	// Then
	if err != nil || len(values) != 1 || gotPath != "/base/api/v3/movie" {
		t.Fatalf("unexpected response/path: %#v %q %v", values, gotPath, err)
	}
}

func TestClients_fetch_rejectsProtocolFailuresWithoutSecretLeak(t *testing.T) {
	for _, body := range []string{"null", `{}`, `[] {}`, `[] []`, strings.Repeat("x", maxResponseBytes+1)} {
		t.Run("invalid response", func(t *testing.T) {
			secret := "do-not-leak"
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) { _, _ = writer.Write([]byte(body)) }))
			defer server.Close()
			config, err := providerConfigFor(server.URL, secret)
			if err != nil {
				t.Fatal(err)
			}
			_, err = NewSonarrClient(newRequestClient(nil, time.Second)).Fetch(context.Background(), config)
			if err == nil || strings.Contains(err.Error(), secret) {
				t.Fatalf("want sanitized rejection: %v", err)
			}
		})
	}
}

func TestRequestClient_get_preservesCancellationAndTimeoutIdentity(t *testing.T) {
	// Given
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) { <-request.Context().Done() }))
	defer server.Close()
	config, err := providerConfigFor(server.URL, "secret")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// When
	_, err = newRequestClient(nil, time.Millisecond).get(ctx, "sonarr", "series", config, "api")

	// Then
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want cancellation identity: %v", err)
	}
}

func TestRequestClient_get_rejectsRedirectWithoutReturningCredentialURL(t *testing.T) {
	// Given
	secret := "redirect-secret"
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, "/other", http.StatusFound)
	}))
	defer server.Close()
	config, err := providerConfigFor(server.URL, secret)
	if err != nil {
		t.Fatal(err)
	}

	// When
	_, err = newRequestClient(nil, time.Second).get(context.Background(), "sonarr", "series", config, "api")

	// Then
	if !errors.Is(err, errRedirect) || strings.Contains(err.Error(), secret) || strings.Contains(err.Error(), server.URL) {
		t.Fatalf("want sanitized redirect rejection: %v", err)
	}
}

type configListFake struct{ rows *[]sqlite.SystemConfig }

func (f configListFake) List(context.Context) ([]sqlite.SystemConfig, error) {
	return append([]sqlite.SystemConfig(nil), (*f.rows)...), nil
}
func (f configListFake) Get(context.Context, sqlite.SystemConfigID) (sqlite.SystemConfig, error) {
	return sqlite.SystemConfig{}, nil
}
func (f configListFake) Upsert(context.Context, sqlite.SystemConfig) error        { return nil }
func (f configListFake) UpsertBatch(context.Context, []sqlite.SystemConfig) error { return nil }
func (f configListFake) ValueByKey(context.Context, string) (string, error)       { return "", nil }
func activeConfig(key, value string) sqlite.SystemConfig                          { return activeConfigPointer(key, &value) }
func activeConfigPointer(key string, value *string) sqlite.SystemConfig {
	return sqlite.SystemConfig{Key: key, Value: value, ValidStatus: sqlite.Valid}
}
func providerConfigFor(rawURL, key string) (providerConfig, error) {
	parsed, err := validURL(rawURL)
	return providerConfig{baseURL: parsed, apiKey: key}, err
}
