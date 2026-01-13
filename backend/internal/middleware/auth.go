package middleware

import (
	"context"
	"crypto/rsa"
	"net/http"
	"strings"

	"github.com/google/uuid"

	customJWT "github.com/frans-sjostrom/auth-service/pkg/jwt"
)

type contextKey string

const (
	UserIDKey        contextKey = "userID"
	EmailKey         contextKey = "email"
	NameKey          contextKey = "name"
	RoleKey          contextKey = "role"
	IsSuperAdminKey  contextKey = "isSuperAdmin"
	OrganizationsKey contextKey = "organizations"
	CurrentOrgIDKey  contextKey = "currentOrgID"
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
			ctx = context.WithValue(ctx, IsSuperAdminKey, claims.IsSuperAdmin)
			ctx = context.WithValue(ctx, OrganizationsKey, claims.Organizations)
			ctx = context.WithValue(ctx, CurrentOrgIDKey, claims.CurrentOrgID)

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

// IsAdmin checks if the user is an admin (backward compatibility)
func IsAdmin(ctx context.Context) bool {
	role, ok := ctx.Value(RoleKey).(string)
	return ok && role == "admin"
}

// GetRole returns the user's role from context
func GetRole(ctx context.Context) (string, bool) {
	role, ok := ctx.Value(RoleKey).(string)
	return role, ok
}

// SuperAdminMiddleware ensures the authenticated user is a super admin
// Must be used after AuthMiddleware
func SuperAdminMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			isSuperAdmin, ok := r.Context().Value(IsSuperAdminKey).(bool)
			if !ok || !isSuperAdmin {
				http.Error(w, "Super admin access required", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// IsSuperAdmin checks if the user is a super admin
func IsSuperAdmin(ctx context.Context) bool {
	isSuperAdmin, ok := ctx.Value(IsSuperAdminKey).(bool)
	return ok && isSuperAdmin
}

// GetUserOrganizations returns the user's organizations from context
func GetUserOrganizations(ctx context.Context) []customJWT.OrganizationClaim {
	orgs, ok := ctx.Value(OrganizationsKey).([]customJWT.OrganizationClaim)
	if !ok {
		return []customJWT.OrganizationClaim{}
	}
	return orgs
}

// GetCurrentOrgID returns the user's current organization ID from context
func GetCurrentOrgID(ctx context.Context) *uuid.UUID {
	orgID, ok := ctx.Value(CurrentOrgIDKey).(*uuid.UUID)
	if !ok {
		return nil
	}
	return orgID
}

// HasOrgAccess checks if the user has access to the specified organization
func HasOrgAccess(ctx context.Context, orgID uuid.UUID) bool {
	// Super admins have access to all orgs
	if IsSuperAdmin(ctx) {
		return true
	}

	orgs := GetUserOrganizations(ctx)
	for _, org := range orgs {
		if org.ID == orgID {
			return true
		}
	}
	return false
}

// GetOrgRole returns the user's role in the specified organization
func GetOrgRole(ctx context.Context, orgID uuid.UUID) (string, bool) {
	orgs := GetUserOrganizations(ctx)
	for _, org := range orgs {
		if org.ID == orgID {
			return org.Role, true
		}
	}
	return "", false
}

// IsOrgOwnerOrAdmin checks if the user is an owner or admin of the specified organization
func IsOrgOwnerOrAdmin(ctx context.Context, orgID uuid.UUID) bool {
	// Super admins are always considered org admins
	if IsSuperAdmin(ctx) {
		return true
	}

	role, ok := GetOrgRole(ctx, orgID)
	if !ok {
		return false
	}
	return role == "owner" || role == "admin"
}

// IsOrgOwner checks if the user is an owner of the specified organization
func IsOrgOwner(ctx context.Context, orgID uuid.UUID) bool {
	// Super admins are always considered org owners
	if IsSuperAdmin(ctx) {
		return true
	}

	role, ok := GetOrgRole(ctx, orgID)
	return ok && role == "owner"
}
