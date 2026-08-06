package auth

import (
	"testing"
	"time"

	"jproxy-go/internal/store/sqlite"
)

func TestManager_rejectsExpiredAndWrongAlgorithmClaims(t *testing.T) {
	// Given
	manager, err := NewManager([]byte("0123456789abcdef0123456789abcdef"), time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	role := "ADMIN"
	now := time.Now().Unix()
	expired, err := manager.sign(Claims{IssuedAt: now - 120, ExpiresAt: now - 60, JWTID: "expired", UserID: 1, Username: "admin", Role: role})
	if err != nil {
		t.Fatal(err)
	}
	valid, err := manager.Issue(sqlite.SystemUser{ID: 1, Username: "admin", Role: &role})
	if err != nil {
		t.Fatal(err)
	}

	// When
	_, expiredErr := manager.Verify(expired)
	_, malformedErr := manager.Verify(valid + ".extra")

	// Then
	if expiredErr == nil || malformedErr == nil {
		t.Fatalf("expired=%v malformed=%v", expiredErr, malformedErr)
	}
}

func TestManager_revokesIssuedToken(t *testing.T) {
	// Given
	manager, err := NewManager([]byte("0123456789abcdef0123456789abcdef"), time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	role := "ADMIN"
	token, err := manager.Issue(sqlite.SystemUser{ID: 1, Username: "admin", Role: &role})
	if err != nil {
		t.Fatal(err)
	}
	claims, err := manager.Verify(token)
	if err != nil {
		t.Fatal(err)
	}

	// When
	manager.Revoke(claims)
	_, err = manager.Verify(token)

	// Then
	if err == nil {
		t.Fatal("Verify() error = nil")
	}
}
