package middleware

import (
	"context"
	"testing"

	customJWT "github.com/frans-sjostrom/auth-service/pkg/jwt"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestIsSuperAdmin(t *testing.T) {
	tests := []struct {
		name     string
		ctx      context.Context
		expected bool
	}{
		{
			name:     "Super admin context",
			ctx:      context.WithValue(context.Background(), IsSuperAdminKey, true),
			expected: true,
		},
		{
			name:     "Non-super admin context",
			ctx:      context.WithValue(context.Background(), IsSuperAdminKey, false),
			expected: false,
		},
		{
			name:     "Missing key in context",
			ctx:      context.Background(),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsSuperAdmin(tt.ctx)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsAdmin(t *testing.T) {
	tests := []struct {
		name     string
		ctx      context.Context
		expected bool
	}{
		{
			name:     "Admin role",
			ctx:      context.WithValue(context.Background(), RoleKey, "admin"),
			expected: true,
		},
		{
			name:     "User role",
			ctx:      context.WithValue(context.Background(), RoleKey, "user"),
			expected: false,
		},
		{
			name:     "Missing role in context",
			ctx:      context.Background(),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsAdmin(tt.ctx)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetRole(t *testing.T) {
	tests := []struct {
		name         string
		ctx          context.Context
		expectedRole string
		expectedOk   bool
	}{
		{
			name:         "Admin role exists",
			ctx:          context.WithValue(context.Background(), RoleKey, "admin"),
			expectedRole: "admin",
			expectedOk:   true,
		},
		{
			name:         "User role exists",
			ctx:          context.WithValue(context.Background(), RoleKey, "user"),
			expectedRole: "user",
			expectedOk:   true,
		},
		{
			name:         "Role missing from context",
			ctx:          context.Background(),
			expectedRole: "",
			expectedOk:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			role, ok := GetRole(tt.ctx)
			assert.Equal(t, tt.expectedRole, role)
			assert.Equal(t, tt.expectedOk, ok)
		})
	}
}

func TestGetUserOrganizations(t *testing.T) {
	org1 := customJWT.OrganizationClaim{
		ID:   uuid.New(),
		Slug: "org-1",
		Name: "Organization 1",
		Role: "owner",
	}
	org2 := customJWT.OrganizationClaim{
		ID:   uuid.New(),
		Slug: "org-2",
		Name: "Organization 2",
		Role: "member",
	}

	tests := []struct {
		name     string
		ctx      context.Context
		expected []customJWT.OrganizationClaim
	}{
		{
			name:     "Organizations exist",
			ctx:      context.WithValue(context.Background(), OrganizationsKey, []customJWT.OrganizationClaim{org1, org2}),
			expected: []customJWT.OrganizationClaim{org1, org2},
		},
		{
			name:     "Empty organizations",
			ctx:      context.WithValue(context.Background(), OrganizationsKey, []customJWT.OrganizationClaim{}),
			expected: []customJWT.OrganizationClaim{},
		},
		{
			name:     "Organizations missing from context",
			ctx:      context.Background(),
			expected: []customJWT.OrganizationClaim{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetUserOrganizations(tt.ctx)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetCurrentOrgID(t *testing.T) {
	orgID := uuid.New()

	tests := []struct {
		name     string
		ctx      context.Context
		expected *uuid.UUID
	}{
		{
			name:     "Current org ID exists",
			ctx:      context.WithValue(context.Background(), CurrentOrgIDKey, &orgID),
			expected: &orgID,
		},
		{
			name:     "Current org ID is nil",
			ctx:      context.WithValue(context.Background(), CurrentOrgIDKey, (*uuid.UUID)(nil)),
			expected: nil,
		},
		{
			name:     "Current org ID missing from context",
			ctx:      context.Background(),
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetCurrentOrgID(tt.ctx)
			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				assert.Equal(t, *tt.expected, *result)
			}
		})
	}
}

func TestHasOrgAccess(t *testing.T) {
	org1ID := uuid.New()
	org2ID := uuid.New()
	org3ID := uuid.New()

	org1 := customJWT.OrganizationClaim{
		ID:   org1ID,
		Slug: "org-1",
		Name: "Organization 1",
		Role: "owner",
	}
	org2 := customJWT.OrganizationClaim{
		ID:   org2ID,
		Slug: "org-2",
		Name: "Organization 2",
		Role: "member",
	}

	tests := []struct {
		name     string
		ctx      context.Context
		orgID    uuid.UUID
		expected bool
	}{
		{
			name: "Super admin has access to any org",
			ctx: func() context.Context {
				ctx := context.WithValue(context.Background(), IsSuperAdminKey, true)
				return context.WithValue(ctx, OrganizationsKey, []customJWT.OrganizationClaim{})
			}(),
			orgID:    org3ID,
			expected: true,
		},
		{
			name: "User has access to owned org",
			ctx: func() context.Context {
				ctx := context.WithValue(context.Background(), IsSuperAdminKey, false)
				return context.WithValue(ctx, OrganizationsKey, []customJWT.OrganizationClaim{org1, org2})
			}(),
			orgID:    org1ID,
			expected: true,
		},
		{
			name: "User has access to member org",
			ctx: func() context.Context {
				ctx := context.WithValue(context.Background(), IsSuperAdminKey, false)
				return context.WithValue(ctx, OrganizationsKey, []customJWT.OrganizationClaim{org1, org2})
			}(),
			orgID:    org2ID,
			expected: true,
		},
		{
			name: "User does not have access to org",
			ctx: func() context.Context {
				ctx := context.WithValue(context.Background(), IsSuperAdminKey, false)
				return context.WithValue(ctx, OrganizationsKey, []customJWT.OrganizationClaim{org1, org2})
			}(),
			orgID:    org3ID,
			expected: false,
		},
		{
			name: "User has no organizations",
			ctx: func() context.Context {
				ctx := context.WithValue(context.Background(), IsSuperAdminKey, false)
				return context.WithValue(ctx, OrganizationsKey, []customJWT.OrganizationClaim{})
			}(),
			orgID:    org1ID,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HasOrgAccess(tt.ctx, tt.orgID)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetOrgRole(t *testing.T) {
	org1ID := uuid.New()
	org2ID := uuid.New()
	org3ID := uuid.New()

	org1 := customJWT.OrganizationClaim{
		ID:   org1ID,
		Slug: "org-1",
		Name: "Organization 1",
		Role: "owner",
	}
	org2 := customJWT.OrganizationClaim{
		ID:   org2ID,
		Slug: "org-2",
		Name: "Organization 2",
		Role: "member",
	}

	ctx := context.WithValue(context.Background(), OrganizationsKey, []customJWT.OrganizationClaim{org1, org2})

	tests := []struct {
		name         string
		orgID        uuid.UUID
		expectedRole string
		expectedOk   bool
	}{
		{
			name:         "Get owner role",
			orgID:        org1ID,
			expectedRole: "owner",
			expectedOk:   true,
		},
		{
			name:         "Get member role",
			orgID:        org2ID,
			expectedRole: "member",
			expectedOk:   true,
		},
		{
			name:         "Org not found",
			orgID:        org3ID,
			expectedRole: "",
			expectedOk:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			role, ok := GetOrgRole(ctx, tt.orgID)
			assert.Equal(t, tt.expectedRole, role)
			assert.Equal(t, tt.expectedOk, ok)
		})
	}
}

func TestIsOrgOwnerOrAdmin(t *testing.T) {
	org1ID := uuid.New()
	org2ID := uuid.New()
	org3ID := uuid.New()
	org4ID := uuid.New()

	org1 := customJWT.OrganizationClaim{
		ID:   org1ID,
		Slug: "org-1",
		Name: "Organization 1",
		Role: "owner",
	}
	org2 := customJWT.OrganizationClaim{
		ID:   org2ID,
		Slug: "org-2",
		Name: "Organization 2",
		Role: "admin",
	}
	org3 := customJWT.OrganizationClaim{
		ID:   org3ID,
		Slug: "org-3",
		Name: "Organization 3",
		Role: "member",
	}

	tests := []struct {
		name     string
		ctx      context.Context
		orgID    uuid.UUID
		expected bool
	}{
		{
			name: "Super admin is considered owner/admin",
			ctx: func() context.Context {
				ctx := context.WithValue(context.Background(), IsSuperAdminKey, true)
				return context.WithValue(ctx, OrganizationsKey, []customJWT.OrganizationClaim{})
			}(),
			orgID:    org4ID,
			expected: true,
		},
		{
			name: "User is owner",
			ctx: func() context.Context {
				ctx := context.WithValue(context.Background(), IsSuperAdminKey, false)
				return context.WithValue(ctx, OrganizationsKey, []customJWT.OrganizationClaim{org1, org2, org3})
			}(),
			orgID:    org1ID,
			expected: true,
		},
		{
			name: "User is admin",
			ctx: func() context.Context {
				ctx := context.WithValue(context.Background(), IsSuperAdminKey, false)
				return context.WithValue(ctx, OrganizationsKey, []customJWT.OrganizationClaim{org1, org2, org3})
			}(),
			orgID:    org2ID,
			expected: true,
		},
		{
			name: "User is only member",
			ctx: func() context.Context {
				ctx := context.WithValue(context.Background(), IsSuperAdminKey, false)
				return context.WithValue(ctx, OrganizationsKey, []customJWT.OrganizationClaim{org1, org2, org3})
			}(),
			orgID:    org3ID,
			expected: false,
		},
		{
			name: "User not in org",
			ctx: func() context.Context {
				ctx := context.WithValue(context.Background(), IsSuperAdminKey, false)
				return context.WithValue(ctx, OrganizationsKey, []customJWT.OrganizationClaim{org1, org2, org3})
			}(),
			orgID:    org4ID,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsOrgOwnerOrAdmin(tt.ctx, tt.orgID)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsOrgOwner(t *testing.T) {
	org1ID := uuid.New()
	org2ID := uuid.New()
	org3ID := uuid.New()

	org1 := customJWT.OrganizationClaim{
		ID:   org1ID,
		Slug: "org-1",
		Name: "Organization 1",
		Role: "owner",
	}
	org2 := customJWT.OrganizationClaim{
		ID:   org2ID,
		Slug: "org-2",
		Name: "Organization 2",
		Role: "admin",
	}

	tests := []struct {
		name     string
		ctx      context.Context
		orgID    uuid.UUID
		expected bool
	}{
		{
			name: "Super admin is considered owner",
			ctx: func() context.Context {
				ctx := context.WithValue(context.Background(), IsSuperAdminKey, true)
				return context.WithValue(ctx, OrganizationsKey, []customJWT.OrganizationClaim{})
			}(),
			orgID:    org3ID,
			expected: true,
		},
		{
			name: "User is owner",
			ctx: func() context.Context {
				ctx := context.WithValue(context.Background(), IsSuperAdminKey, false)
				return context.WithValue(ctx, OrganizationsKey, []customJWT.OrganizationClaim{org1, org2})
			}(),
			orgID:    org1ID,
			expected: true,
		},
		{
			name: "User is admin but not owner",
			ctx: func() context.Context {
				ctx := context.WithValue(context.Background(), IsSuperAdminKey, false)
				return context.WithValue(ctx, OrganizationsKey, []customJWT.OrganizationClaim{org1, org2})
			}(),
			orgID:    org2ID,
			expected: false,
		},
		{
			name: "User not in org",
			ctx: func() context.Context {
				ctx := context.WithValue(context.Background(), IsSuperAdminKey, false)
				return context.WithValue(ctx, OrganizationsKey, []customJWT.OrganizationClaim{org1, org2})
			}(),
			orgID:    org3ID,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsOrgOwner(tt.ctx, tt.orgID)
			assert.Equal(t, tt.expected, result)
		})
	}
}
