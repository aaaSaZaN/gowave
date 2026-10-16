package auth

import (
	"time"

	"gowave/internal/middleware"

	"github.com/golang-jwt/jwt/v5"
)

func GenerateToken(uuid string, username string, role string, appSecret string, lifetimeHours int) (string, error) {
	claims := middleware.TokenClaims{
		UUID:     uuid,
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(lifetimeHours) * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(appSecret))
}
