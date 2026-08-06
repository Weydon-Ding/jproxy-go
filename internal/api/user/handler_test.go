package user

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

	"jproxy-go/internal/auth"
	"jproxy-go/internal/store/sqlite"
)

func TestHandler_loginUpgradesLegacyPassword_thenUpdatesAndRevokes(t *testing.T) {
	// Given
	store := testStore(t)
	legacy := stringsToUpperMD5("old-password")
	role := "ADMIN"
	if err := store.Repositories().SystemUsers.Upsert(context.Background(), sqlite.SystemUser{ID: 7, Username: "admin", Password: &legacy, Role: &role, ValidStatus: sqlite.Valid}); err != nil {
		t.Fatal(err)
	}
	manager, err := auth.NewManager([]byte("0123456789abcdef0123456789abcdef"), time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	handler := userRoutes(store, manager, true)

	// When
	token := requestToken(t, handler, `{"username":"admin","password":"old-password"}`)
	info := request(handler, http.MethodGet, "/api/system/user/info", "", token)
	updated := request(handler, http.MethodPost, "/api/system/user/update", `{"username":"renamed","password":"new-password"}`, token)
	logout := request(handler, http.MethodPost, "/api/system/user/logout", "", token)
	denied := request(handler, http.MethodGet, "/api/system/user/info", "", token)
	newToken := requestToken(t, handler, `{"username":"renamed","password":"new-password"}`)

	// Then
	if info.Code != http.StatusOK || !bytes.Contains(info.Body.Bytes(), []byte(`"password":"******"`)) || updated.Code != http.StatusOK || logout.Code != http.StatusOK || denied.Code != http.StatusUnauthorized || newToken == "" {
		t.Fatalf("status info=%d update=%d logout=%d denied=%d", info.Code, updated.Code, logout.Code, denied.Code)
	}
	row, err := store.Repositories().SystemUsers.FindByUsername(context.Background(), "renamed")
	if err != nil || row.Password == nil || len(*row.Password) != 60 {
		t.Fatalf("row=%#v err=%v", row, err)
	}
}

func TestHandler_disabledLoginIssuesAnonymousToken_andForbidsUpdate(t *testing.T) {
	// Given
	store := testStore(t)
	manager, err := auth.NewManager([]byte("0123456789abcdef0123456789abcdef"), time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	handler := userRoutes(store, manager, false)

	// When
	token := requestToken(t, handler, "{}")
	update := request(handler, http.MethodPost, "/api/system/user/update", `{}`, token)

	// Then
	if token == "" || update.Code != http.StatusUnauthorized {
		t.Fatalf("token=%q status=%d", token, update.Code)
	}
}

func TestManager_rejectsMalformedExpiredAndWrongAlgorithmTokens(t *testing.T) {
	// Given
	manager, err := auth.NewManager([]byte("0123456789abcdef0123456789abcdef"), time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	role := "ADMIN"
	token, err := manager.Issue(sqlite.SystemUser{ID: 1, Username: "admin", Role: &role})
	if err != nil {
		t.Fatal(err)
	}

	// When
	_, malformedErr := manager.Verify("one.two")
	_, tamperedErr := manager.Verify(token + "x")

	// Then
	if malformedErr == nil || tamperedErr == nil {
		t.Fatalf("malformed=%v tampered=%v", malformedErr, tamperedErr)
	}
}

func testStore(t *testing.T) *sqlite.Store {
	store, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "user.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}
func stringsToUpperMD5(value string) string {
	digest := md5.Sum([]byte(value))
	return string(bytes.ToUpper([]byte(hex.EncodeToString(digest[:]))))
}
func requestToken(t *testing.T, handler http.Handler, body string) string {
	recorder := request(handler, http.MethodPost, "/api/system/user/login", body, "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("login=%d", recorder.Code)
	}
	var token string
	if err := json.NewDecoder(recorder.Body).Decode(&token); err != nil {
		t.Fatal(err)
	}
	return token
}
func request(handler http.Handler, method, url, body, token string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, url, bytes.NewBufferString(body))
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

func userRoutes(store *sqlite.Store, manager *auth.Manager, loginEnabled bool) http.Handler {
	users := NewHandler(Options{Store: store, Tokens: manager, LoginEnabled: loginEnabled})
	routes := http.NewServeMux()
	routes.Handle("/api/system/user/login", users)
	routes.Handle("/api/system/user/isLoginEnabled", users)
	routes.Handle("/api/system/user/", auth.Require(manager, users))
	return routes
}
