package models

import (
	"time"

	"github.com/google/uuid"
)

const (
	RoleUser  = "user"
	RoleAdmin = "admin"
)

// Organization roles
const (
	OrgRoleOwner  = "owner"
	OrgRoleAdmin  = "admin"
	OrgRoleMember = "member"
	OrgRoleViewer = "viewer"
)

type User struct {
	ID           uuid.UUID  `json:"id" db:"id"`
	Email        string     `json:"email" db:"email"`
	GoogleID     *string    `json:"google_id,omitempty" db:"google_id"`
	Name         string     `json:"name" db:"name"`
	AvatarURL    *string    `json:"avatar_url,omitempty" db:"avatar_url"`
	Role         string     `json:"role" db:"role"`
	IsSuperAdmin bool       `json:"is_super_admin" db:"is_super_admin"`
	IsActive     bool       `json:"is_active" db:"is_active"`
	CreatedAt    time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at" db:"updated_at"`
	DeletedAt    *time.Time `json:"deleted_at,omitempty" db:"deleted_at"`
}

// IsAdmin returns true if the user has the admin role
func (u *User) IsAdmin() bool {
	return u.Role == RoleAdmin
}

// IsSuperAdminUser returns true if the user is a super admin
func (u *User) IsSuperAdminUser() bool {
	return u.IsSuperAdmin
}

type RefreshToken struct {
	ID        uuid.UUID  `json:"id" db:"id"`
	UserID    uuid.UUID  `json:"user_id" db:"user_id"`
	TokenHash string     `json:"-" db:"token_hash"`
	ExpiresAt time.Time  `json:"expires_at" db:"expires_at"`
	CreatedAt time.Time  `json:"created_at" db:"created_at"`
	RevokedAt *time.Time `json:"revoked_at,omitempty" db:"revoked_at"`
}

type AuthAuditLog struct {
	ID        uuid.UUID  `json:"id" db:"id"`
	UserID    *uuid.UUID `json:"user_id,omitempty" db:"user_id"`
	Action    string     `json:"action" db:"action"`
	IPAddress *string    `json:"ip_address,omitempty" db:"ip_address"`
	UserAgent *string    `json:"user_agent,omitempty" db:"user_agent"`
	CreatedAt time.Time  `json:"created_at" db:"created_at"`
}

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
}

type GoogleUserInfo struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
}

// Application represents a registered app using this auth service
// Replaces the old AllowedOrigin model
type Application struct {
	ID           uuid.UUID  `json:"id" db:"id"`
	Name         string     `json:"name" db:"name"`
	Slug         string     `json:"slug" db:"slug"`
	Origin       string     `json:"origin" db:"origin"`
	Description  *string    `json:"description,omitempty" db:"description"`
	RedirectURIs []string   `json:"redirect_uris,omitempty" db:"redirect_uris"`
	IsActive     bool       `json:"is_active" db:"is_active"`
	CreatedAt    time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at" db:"updated_at"`
	CreatedBy    *uuid.UUID `json:"created_by,omitempty" db:"created_by"`
}

// Organization represents a business entity/team
type Organization struct {
	ID          uuid.UUID  `json:"id" db:"id"`
	Name        string     `json:"name" db:"name"`
	Slug        string     `json:"slug" db:"slug"`
	Description *string    `json:"description,omitempty" db:"description"`
	IsActive    bool       `json:"is_active" db:"is_active"`
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at" db:"updated_at"`
	CreatedBy   *uuid.UUID `json:"created_by,omitempty" db:"created_by"`
}

// UserOrganization represents a user's membership in an organization with a role
type UserOrganization struct {
	ID             uuid.UUID `json:"id" db:"id"`
	UserID         uuid.UUID `json:"user_id" db:"user_id"`
	OrganizationID uuid.UUID `json:"organization_id" db:"organization_id"`
	Role           string    `json:"role" db:"role"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time `json:"updated_at" db:"updated_at"`
}

// UserApplicationLogin tracks which users log into which applications
type UserApplicationLogin struct {
	ID             uuid.UUID  `json:"id" db:"id"`
	UserID         uuid.UUID  `json:"user_id" db:"user_id"`
	ApplicationID  uuid.UUID  `json:"application_id" db:"application_id"`
	OrganizationID *uuid.UUID `json:"organization_id,omitempty" db:"organization_id"`
	IPAddress      *string    `json:"ip_address,omitempty" db:"ip_address"`
	UserAgent      *string    `json:"user_agent,omitempty" db:"user_agent"`
	CreatedAt      time.Time  `json:"created_at" db:"created_at"`
}

// OrganizationApplication controls which organizations can access which applications
type OrganizationApplication struct {
	ID             uuid.UUID `json:"id" db:"id"`
	OrganizationID uuid.UUID `json:"organization_id" db:"organization_id"`
	ApplicationID  uuid.UUID `json:"application_id" db:"application_id"`
	IsActive       bool      `json:"is_active" db:"is_active"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
}

// Backward compatibility alias
type AllowedOrigin = Application
