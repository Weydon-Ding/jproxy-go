package user

import (
	"crypto/md5"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"jproxy-go/internal/auth"
	"jproxy-go/internal/store/sqlite"
)

const maxBodyBytes = 64 * 1024

type Options struct {
	Store        Store
	Tokens       *auth.Manager
	LoginEnabled bool
}

type Store interface{ Repositories() sqlite.Repositories }

type Handler struct{ options Options }

type credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func NewHandler(options Options) *Handler { return &Handler{options: options} }

func (h *Handler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	switch request.Method + " " + request.URL.Path {
	case http.MethodPost + " /api/system/user/login":
		h.login(writer, request)
	case http.MethodGet + " /api/system/user/info":
		h.info(writer, request)
	case http.MethodPost + " /api/system/user/update":
		h.update(writer, request)
	case http.MethodPost + " /api/system/user/logout":
		h.logout(writer, request)
	case http.MethodGet + " /api/system/user/isLoginEnabled":
		writeJSON(writer, http.StatusOK, h.options.LoginEnabled)
	default:
		http.NotFound(writer, request)
	}
}

func (h *Handler) login(writer http.ResponseWriter, request *http.Request) {
	if !h.options.LoginEnabled {
		h.anonymous(writer)
		return
	}
	var input credentials
	if decodeJSON(request, &input) != nil || input.Username == "" || input.Password == "" {
		writeError(writer, http.StatusUnauthorized)
		return
	}
	row, err := h.options.Store.Repositories().SystemUsers.FindByUsername(request.Context(), input.Username)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(writer, http.StatusUnauthorized)
			return
		}
		writeError(writer, http.StatusUnauthorized)
		return
	}
	if !validCredentials(input.Password, row) {
		writeError(writer, http.StatusUnauthorized)
		return
	}
	if legacyMD5(row.Password) {
		hash, hashErr := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
		if hashErr != nil || h.options.Store.Repositories().SystemUsers.Upsert(request.Context(), withPassword(row, string(hash))) != nil {
			writeError(writer, http.StatusInternalServerError)
			return
		}
	}
	h.issue(writer, row)
}

func (h *Handler) anonymous(writer http.ResponseWriter) {
	role := "ANONYMOUS"
	h.issue(writer, sqlite.SystemUser{ID: 1, Username: "Anonymous", Role: &role, ValidStatus: sqlite.Valid})
}

func (h *Handler) issue(writer http.ResponseWriter, row sqlite.SystemUser) {
	token, err := h.options.Tokens.Issue(row)
	if err != nil {
		writeError(writer, http.StatusInternalServerError)
		return
	}
	writeJSON(writer, http.StatusOK, token)
}

func (h *Handler) info(writer http.ResponseWriter, request *http.Request) {
	claims, ok := auth.ClaimsFromContext(request.Context())
	if !ok {
		writeError(writer, http.StatusUnauthorized)
		return
	}
	if claims.Role == "ANONYMOUS" {
		masked := "******"
		writeJSON(writer, http.StatusOK, sqlite.SystemUser{ID: sqlite.SystemUserID(claims.UserID), Username: claims.Username, Password: &masked, Role: &claims.Role, ValidStatus: sqlite.Valid})
		return
	}
	row, err := h.options.Store.Repositories().SystemUsers.Get(request.Context(), sqlite.SystemUserID(claims.UserID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(writer, http.StatusUnauthorized)
			return
		}
		writeError(writer, http.StatusUnauthorized)
		return
	}
	masked := "******"
	row.Password = &masked
	writeJSON(writer, http.StatusOK, row)
}

func (h *Handler) update(writer http.ResponseWriter, request *http.Request) {
	if !h.options.LoginEnabled {
		writeError(writer, http.StatusUnauthorized)
		return
	}
	claims, ok := auth.ClaimsFromContext(request.Context())
	if !ok {
		writeError(writer, http.StatusUnauthorized)
		return
	}
	var input sqlite.SystemUser
	if decodeJSON(request, &input) != nil {
		writeError(writer, http.StatusBadRequest)
		return
	}
	row, err := h.options.Store.Repositories().SystemUsers.Get(request.Context(), sqlite.SystemUserID(claims.UserID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(writer, http.StatusUnauthorized)
			return
		}
		writeError(writer, http.StatusUnauthorized)
		return
	}
	if input.Username != "" {
		row.Username = input.Username
	}
	if input.Password != nil && *input.Password != "" {
		hash, hashErr := bcrypt.GenerateFromPassword([]byte(*input.Password), bcrypt.DefaultCost)
		if hashErr != nil {
			writeError(writer, http.StatusInternalServerError)
			return
		}
		row.Password = stringPointer(string(hash))
	}
	if err := h.options.Store.Repositories().SystemUsers.Upsert(request.Context(), row); err != nil {
		writeError(writer, http.StatusInternalServerError)
		return
	}
	masked := "******"
	row.Password = &masked
	writeJSON(writer, http.StatusOK, row)
}

func (h *Handler) logout(writer http.ResponseWriter, request *http.Request) {
	claims, ok := auth.ClaimsFromContext(request.Context())
	if !ok {
		writeError(writer, http.StatusUnauthorized)
		return
	}
	h.options.Tokens.Revoke(claims)
	writer.WriteHeader(http.StatusOK)
}

func validCredentials(password string, row sqlite.SystemUser) bool {
	if row.ValidStatus != sqlite.Valid || row.Password == nil {
		return false
	}
	if legacyMD5(row.Password) {
		digest := md5.Sum([]byte(password))
		return subtle.ConstantTimeCompare([]byte(strings.ToUpper(hex.EncodeToString(digest[:]))), []byte(*row.Password)) == 1
	}
	return bcrypt.CompareHashAndPassword([]byte(*row.Password), []byte(password)) == nil
}

func legacyMD5(password *string) bool {
	return password != nil && len(*password) == 32 && isUpperHex(*password)
}

func isUpperHex(value string) bool {
	for _, character := range value {
		if !(character >= '0' && character <= '9' || character >= 'A' && character <= 'F') {
			return false
		}
	}
	return true
}

func withPassword(user sqlite.SystemUser, password string) sqlite.SystemUser {
	user.Password = &password
	return user
}
func stringPointer(value string) *string { return &value }

func decodeJSON(request *http.Request, target any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(nil, request.Body, maxBodyBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("expected one JSON value")
	}
	return nil
}
func writeError(writer http.ResponseWriter, status int) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_, _ = writer.Write([]byte(`{"error":"request failed"}`))
}
func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}
