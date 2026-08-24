package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const EmailKey contextKey = "email"

// JWTAuth validates the JWT token from the "token" cookie.
func JWTAuth(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie("token")
			if err != nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			token, err := jwt.Parse(cookie.Value, func(t *jwt.Token) (any, error) {
				return []byte(secret), nil
			}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithLeeway(30*time.Second))

			if err != nil || !token.Valid {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			email, _ := claims["email"].(string)
			sub, _ := claims["sub"].(string)
			// Defense-in-depth: reject malformed tokens where neither email nor sub is set.
			if email == "" && sub == "" {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			userIdentifier := email
			if userIdentifier == "" {
				userIdentifier = sub
			}
			ctx := context.WithValue(r.Context(), EmailKey, userIdentifier)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
