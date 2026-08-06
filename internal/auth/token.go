package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"jproxy-go/internal/store/sqlite"
)

var ErrInvalidToken = errors.New("invalid token")

type Claims struct {
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
	JWTID     string `json:"jti"`
	UserID    int64  `json:"userId"`
	Username  string `json:"username"`
	Role      string `json:"role"`
}

type Manager struct {
	secret      []byte
	expiresIn   time.Duration
	revocations *revocations
}

func NewManager(secret []byte, expiresIn time.Duration) (*Manager, error) {
	if len(secret) < 32 || expiresIn <= 0 {
		return nil, ErrInvalidToken
	}
	return &Manager{secret: append([]byte(nil), secret...), expiresIn: expiresIn, revocations: newRevocations(1024)}, nil
}

func RandomSecret() ([]byte, error) {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, fmt.Errorf("generate auth secret: %w", err)
	}
	return secret, nil
}

func (m *Manager) Issue(user sqlite.SystemUser) (string, error) {
	if user.Role == nil {
		return "", ErrInvalidToken
	}
	jti, err := randomID()
	if err != nil {
		return "", err
	}
	now := time.Now().UTC()
	claims := Claims{IssuedAt: now.Unix(), ExpiresAt: now.Add(m.expiresIn).Unix(), JWTID: jti, UserID: int64(user.ID), Username: user.Username, Role: *user.Role}
	return m.sign(claims)
}

func (m *Manager) Verify(token string) (Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return Claims{}, ErrInvalidToken
	}
	headerBytes, err := decode(parts[0])
	if err != nil {
		return Claims{}, ErrInvalidToken
	}
	var header struct {
		Alg string `json:"alg"`
		Typ string `json:"typ"`
	}
	if json.Unmarshal(headerBytes, &header) != nil || header.Alg != "HS256" || header.Typ != "JWT" {
		return Claims{}, ErrInvalidToken
	}
	signature, err := decode(parts[2])
	if err != nil {
		return Claims{}, ErrInvalidToken
	}
	mac := hmac.New(sha256.New, m.secret)
	_, _ = mac.Write([]byte(parts[0] + "." + parts[1]))
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return Claims{}, ErrInvalidToken
	}
	payload, err := decode(parts[1])
	if err != nil {
		return Claims{}, ErrInvalidToken
	}
	var claims Claims
	if json.Unmarshal(payload, &claims) != nil || claims.JWTID == "" || claims.UserID <= 0 || claims.Username == "" || claims.Role == "" || claims.IssuedAt <= 0 || claims.ExpiresAt <= claims.IssuedAt {
		return Claims{}, ErrInvalidToken
	}
	now := time.Now().Unix()
	if claims.IssuedAt > now || claims.ExpiresAt <= now || m.revocations.contains(claims.JWTID, now) {
		return Claims{}, ErrInvalidToken
	}
	return claims, nil
}

func (m *Manager) Revoke(claims Claims) {
	m.revocations.add(claims.JWTID, claims.ExpiresAt, time.Now().Unix())
}

func (m *Manager) sign(claims Claims) (string, error) {
	header, err := json.Marshal(struct {
		Alg string `json:"alg"`
		Typ string `json:"typ"`
	}{Alg: "HS256", Typ: "JWT"})
	if err != nil {
		return "", fmt.Errorf("marshal token header: %w", err)
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("marshal token claims: %w", err)
	}
	signingInput := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, m.secret)
	_, _ = mac.Write([]byte(signingInput))
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func decode(value string) ([]byte, error) { return base64.RawURLEncoding.DecodeString(value) }

func randomID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("generate token id: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}
