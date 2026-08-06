package app

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"jproxy-go/internal/config"
	"jproxy-go/internal/runtime"
	"jproxy-go/internal/store/sqlite"
)

func TestRootHandler_protectsManagementButKeepsHealthAndProxyPublic_whenLoginEnabled(t *testing.T) {
	// Given
	store, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "auth.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	password := legacyPassword("password")
	role := "ADMIN"
	if err := store.Repositories().SystemUsers.Upsert(context.Background(), sqlite.SystemUser{ID: 1, Username: "admin", Password: &password, Role: &role, ValidStatus: sqlite.Valid}); err != nil {
		t.Fatal(err)
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) { _, _ = writer.Write([]byte("proxy")) }))
	t.Cleanup(upstream.Close)
	cfg := config.Config{JackettURL: upstream.URL, ProwlarrURL: upstream.URL, HTTPTimeout: time.Second, Database: config.DatabaseConfig{Enabled: true}, Auth: config.AuthConfig{LoginEnabled: true, JWTSecret: "0123456789abcdef0123456789abcdef", TokenExpiresMinutes: 60}}
	server := httptest.NewServer(rootHandler(cfg, runtime.NewStaticProvider(sqlite.Snapshot{}), store))
	t.Cleanup(server.Close)

	// When
	protected := rootRequest(t, server.URL, http.MethodGet, "/api/system/config/query", "", "")
	health := rootRequest(t, server.URL, http.MethodGet, "/health", "", "")
	proxy := rootRequest(t, server.URL, http.MethodGet, "/sonarr/jackett/api", "", "")
	login := rootRequest(t, server.URL, http.MethodPost, "/api/system/user/login", `{"username":"admin","password":"password"}`, "")
	var token string
	if err := json.NewDecoder(login.Body).Decode(&token); err != nil {
		t.Fatal(err)
	}
	authorized := rootRequest(t, server.URL, http.MethodGet, "/api/system/config/query", "", token)

	// Then
	if protected.Code != http.StatusUnauthorized || health.Code != http.StatusOK || proxy.Code != http.StatusOK || login.Code != http.StatusOK || token == "" || authorized.Code != http.StatusOK {
		t.Fatalf("statuses protected=%d health=%d proxy=%d login=%d authorized=%d", protected.Code, health.Code, proxy.Code, login.Code, authorized.Code)
	}
}

func TestRootHandler_issuesAnonymousTokenAndKeepsManagementCompatible_whenLoginDisabled(t *testing.T) {
	// Given
	store, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "anonymous.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	cfg := config.Config{Database: config.DatabaseConfig{Enabled: true}, Auth: config.AuthConfig{TokenExpiresMinutes: 60}}
	server := httptest.NewServer(rootHandler(cfg, runtime.NewStaticProvider(sqlite.Snapshot{}), store))
	t.Cleanup(server.Close)

	// When
	login := rootRequest(t, server.URL, http.MethodPost, "/api/system/user/login", "{}", "")
	status := rootRequest(t, server.URL, http.MethodGet, "/api/system/user/isLoginEnabled", "", "")
	management := rootRequest(t, server.URL, http.MethodGet, "/api/system/config/query", "", "")

	// Then
	if login.Code != http.StatusOK || status.Code != http.StatusOK || string(bytes.TrimSpace(status.Body.Bytes())) != "false" || management.Code != http.StatusOK {
		t.Fatalf("statuses login=%d status=%d management=%d", login.Code, status.Code, management.Code)
	}
}

func rootRequest(t *testing.T, base, method, path, body, token string) *httptest.ResponseRecorder {
	t.Helper()
	request, err := http.NewRequest(method, base+path, bytes.NewBufferString(body))
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	recorder := httptest.NewRecorder()
	recorder.Code = response.StatusCode
	_, _ = recorder.Body.ReadFrom(response.Body)
	return recorder
}

func legacyPassword(value string) string {
	sum := md5.Sum([]byte(value))
	return string(bytes.ToUpper([]byte(hex.EncodeToString(sum[:]))))
}
