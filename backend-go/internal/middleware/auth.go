package middleware

import (
	"context"
	"gowave/internal/contracts"
	"gowave/internal/database"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
)

type contextKey string

const AdminContextKey contextKey = "admin"

type TokenClaims struct {
	UUID     string `json:"uuid"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

func Auth(appSecret string, db ...*gorm.DB) func(http.Handler) http.Handler {
	var connection *gorm.DB
	if len(db) > 0 {
		connection = db[0]
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
				http.Error(w, `{"error":"Unauthorized"}`, http.StatusUnauthorized)
				return
			}

			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")

			token, err := jwt.ParseWithClaims(tokenStr, &TokenClaims{}, func(token *jwt.Token) (interface{}, error) {
				if token.Method != jwt.SigningMethodHS256 {
					return nil, jwt.ErrSignatureInvalid
				}
				return []byte(appSecret), nil
			}, jwt.WithExpirationRequired(), jwt.WithValidMethods([]string{"HS256"}))

			if err != nil || !token.Valid {
				http.Error(w, `{"error":"Unauthorized"}`, http.StatusUnauthorized)
				return
			}

			claims, ok := token.Claims.(*TokenClaims)
			if !ok {
				http.Error(w, `{"error":"Unauthorized"}`, http.StatusUnauthorized)
				return
			}

			if connection == nil || claims.UUID == "" || (claims.Role != "API" && claims.Role != "ADMIN") {
				http.Error(w, `{"error":"Unauthorized"}`, http.StatusUnauthorized)
				return
			}
			if claims.Role == "API" {
				var record database.ApiToken
				if err := connection.Where("uuid = ?", claims.UUID).First(&record).Error; err != nil || !record.ExpireAt.After(time.Now()) {
					http.Error(w, `{"error":"Unauthorized"}`, http.StatusUnauthorized)
					return
				}
				endpoints, _ := contracts.Lookup(r.Method, r.URL.Path)
				allowed := false
				for _, endpoint := range endpoints {
					if contracts.Allows(record.Scopes, endpoint) {
						allowed = true
						break
					}
				}
				if contracts.AdminOnly(r.URL.Path) || !allowed {
					http.Error(w, `{"error":"Forbidden"}`, http.StatusForbidden)
					return
				}
			} else {
				var admin database.Admin
				if err := connection.Where("uuid = ? AND role = ?", claims.UUID, "ADMIN").First(&admin).Error; err != nil {
					http.Error(w, `{"error":"Unauthorized"}`, http.StatusUnauthorized)
					return
				}
			}

			ctx := context.WithValue(r.Context(), AdminContextKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
