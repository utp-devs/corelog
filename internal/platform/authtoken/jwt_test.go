package authtoken

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestJWT(t *testing.T) {
	secret := "super-secret-key-at-least-32-chars-long"
	userID := uuid.New()
	role := "admin"

	t.Run("Generate and Parse valid token", func(t *testing.T) {
		token, err := Generate(secret, time.Hour, userID, role)
		if err != nil {
			t.Fatalf("failed to generate token: %v", err)
		}

		claims, err := Parse(secret, token)
		if err != nil {
			t.Fatalf("failed to parse token: %v", err)
		}

		if claims.UserID != userID {
			t.Errorf("got userID %v, want %v", claims.UserID, userID)
		}
		if claims.Role != role {
			t.Errorf("got role %q, want %q", claims.Role, role)
		}
	})

	t.Run("Parse fails with wrong secret", func(t *testing.T) {
		token, err := Generate(secret, time.Hour, userID, role)
		if err != nil {
			t.Fatalf("failed to generate token: %v", err)
		}

		_, err = Parse("wrong-secret-key-that-does-not-match", token)
		if !errors.Is(err, ErrInvalidToken) {
			t.Errorf("expected ErrInvalidToken, got %v", err)
		}
	})

	t.Run("Parse fails with expired token", func(t *testing.T) {
		token, err := Generate(secret, -time.Hour, userID, role)
		if err != nil {
			t.Fatalf("failed to generate token: %v", err)
		}

		_, err = Parse(secret, token)
		if !errors.Is(err, ErrInvalidToken) {
			t.Errorf("expected ErrInvalidToken for expired token, got %v", err)
		}
	})

	t.Run("Parse fails with malformed token", func(t *testing.T) {
		_, err := Parse(secret, "invalid.jwt.token")
		if !errors.Is(err, ErrInvalidToken) {
			t.Errorf("expected ErrInvalidToken for malformed token, got %v", err)
		}
	})
}
