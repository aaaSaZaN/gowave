package middleware

import (
	"net/http"

	"github.com/go-chi/cors"
)

func Cors() func(http.Handler) http.Handler {
	return cors.Handler(cors.Options{
		AllowOriginFunc: func(r *http.Request, origin string) bool {
			return true
		},
		AllowedMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"},
		AllowedHeaders: []string{
			"*",
			"Accept",
			"Authorization",
			"Content-Type",
			"X-CSRF-Token",
			"X-Requested-With",
			"X-Remnawave-Client-Type",
			"x-remnawave-client-type",
			"X-Remnawave-Real-Ip",
			"x-remnawave-real-ip",
			"X-Forwarded-Proto",
			"x-forwarded-proto",
			"X-Forwarded-For",
			"x-forwarded-for",
		},
		ExposedHeaders:   []string{"Link", "profile-web-page-url", "subscription-userinfo"},
		AllowCredentials: true,
		MaxAge:           300,
	})
}
