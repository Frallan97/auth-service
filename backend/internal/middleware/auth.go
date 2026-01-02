package middleware

import (
	"context"
	"crypto/rsa"
	"net/http"
	"strings"

	customJWT "github.com/frans-sjostrom/auth-service/pkg/jwt"
)

type contextKey string

const (
	UserIDKey contextKey = "userID"
	EmailKey  contextKey = "email"
	NameKey   contextKey = "name"
	RoleKey   contextKey = "role"
)

func AuthMiddleware(publicKey *rsa.PublicKey) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "Missing authorization header", http.StatusUnauthorized)
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || parts[0] != "Bearer" {
				http.Error(w, "Invalid authorization header format", http.StatusUnauthorized)
				return
			}

			tokenString := parts[1]
			claims, err := customJWT.ValidateAccessToken(tokenString, publicKey)
			if err != nil {
				http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
			ctx = context.WithValue(ctx, EmailKey, claims.Email)
			ctx = context.WithValue(ctx, NameKey, claims.Name)
			ctx = context.WithValue(ctx, RoleKey, claims.Role)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// AdminMiddleware ensures the authenticated user is an admin
// Must be used after AuthMiddleware
func AdminMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role, ok := r.Context().Value(RoleKey).(string)
			if !ok {
				http.Error(w, "Role not found in context", http.StatusInternalServerError)
				return
			}

			if role != "admin" {
				http.Error(w, "Admin access required", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// IsAdmin checks if the user is an admin
func IsAdmin(ctx context.Context) bool {
	role, ok := ctx.Value(RoleKey).(string)
	return ok && role == "admin"
}

// GetRole returns the user's role from context
func GetRole(ctx context.Context) (string, bool) {
	role, ok := ctx.Value(RoleKey).(string)
	return role, ok
}
