package authtoken

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type Claims struct {
	UserID uuid.UUID `json:"sub"`
	Role   string    `json:"role"`
	jwt.RegisteredClaims
}

var ErrInvalidToken = errors.New("authtoken: invalid or expired token")

// Generate emite un token sin sesion asociada. Los tokens de login usan GenerateWithSession.
func Generate(secret string, ttl time.Duration, userID uuid.UUID, role string) (string, error) {
	return GenerateWithSession(secret, ttl, userID, role, uuid.Nil)
}

// GenerateWithSession emite un token ligado a una sesion (jti). El middleware puede revocarla.
func GenerateWithSession(secret string, ttl time.Duration, userID uuid.UUID, role string, sessionID uuid.UUID) (string, error) {
	claims := Claims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        sessionIDClaim(sessionID),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
			Issuer:    "corelog-api",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// SessionID devuelve la sesion del token, o uuid.Nil si el token no tiene sesion.
func (c *Claims) SessionID() uuid.UUID {
	id, err := uuid.Parse(c.ID)
	if err != nil {
		return uuid.Nil
	}
	return id
}

func sessionIDClaim(id uuid.UUID) string {
	if id == uuid.Nil {
		return ""
	}
	return id.String()
}

func Parse(secret, tokenString string) (*Claims, error) {
	claims := &Claims{}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return []byte(secret), nil
	})
	if err != nil || !token.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}
