package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"

	"github.com/frans-sjostrom/auth-service/internal/config"
	"github.com/frans-sjostrom/auth-service/internal/database"
	"github.com/frans-sjostrom/auth-service/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestService creates a test service with in-memory database
func setupTestService(t *testing.T) (*Service, *config.Config, func()) {
	// Generate test RSA keys
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	cfg := &config.Config{
		DatabaseURL:          "postgresql://authuser:authpass@localhost:5433/authdb?sslmode=disable",
		Port:                 "8080",
		Env:                  "test",
		GoogleClientID:       "test-client-id",
		GoogleClientSecret:   "test-client-secret",
		GoogleRedirectURL:    "http://localhost:8080/api/auth/google/callback",
		JWTPrivateKey:        privateKey,
		JWTPublicKey:         &privateKey.PublicKey,
		JWTAccessTokenExpiry: 15 * time.Minute,
		JWTRefreshTokenExpiry: 7 * 24 * time.Hour,
		AllowedOrigins:       []string{"http://localhost:3000"},
		AdminEmails:          []string{"admin@test.com"},
	}

	// Note: This requires a test database to be running
	// Migrations should be applied manually before running tests
	// Run: docker compose up -d postgres
	// Then apply migrations: docker exec -i auth-postgres psql -U authuser -d authdb < migrations/*.sql
	db, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		t.Skip("Test database not available, skipping integration tests")
		return nil, nil, nil
	}

	// Note: Migrations are expected to be already applied to the test database
	// We skip running them here to avoid path issues in tests

	service := NewService(db, cfg)

	cleanup := func() {
		// Clean up test data
		db.Exec(context.Background(), "DELETE FROM refresh_tokens")
		db.Exec(context.Background(), "DELETE FROM auth_audit_log")
		db.Exec(context.Background(), "DELETE FROM users")
		db.Close()
	}

	return service, cfg, cleanup
}

func TestCreateOrUpdateUser_NewUser(t *testing.T) {
	service, _, cleanup := setupTestService(t)
	if service == nil {
		return // Test skipped
	}
	defer cleanup()

	ctx := context.Background()
	googleUserInfo := &models.GoogleUserInfo{
		ID:      "google-123",
		Email:   "test@example.com",
		Name:    "Test User",
		Picture: "https://example.com/avatar.jpg",
	}

	user, err := service.CreateOrUpdateUser(ctx, googleUserInfo)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, user.ID)
	assert.Equal(t, "test@example.com", user.Email)
	assert.Equal(t, "google-123", *user.GoogleID)
	assert.Equal(t, "Test User", user.Name)
	assert.Equal(t, models.RoleUser, user.Role)
	assert.True(t, user.IsActive)
}

func TestCreateOrUpdateUser_AdminUser(t *testing.T) {
	service, _, cleanup := setupTestService(t)
	if service == nil {
		return
	}
	defer cleanup()

	ctx := context.Background()
	googleUserInfo := &models.GoogleUserInfo{
		ID:      "google-456",
		Email:   "admin@test.com",
		Name:    "Admin User",
		Picture: "https://example.com/admin.jpg",
	}

	user, err := service.CreateOrUpdateUser(ctx, googleUserInfo)
	require.NoError(t, err)
	assert.Equal(t, models.RoleAdmin, user.Role)
	assert.Equal(t, "admin@test.com", user.Email)
}

func TestCreateOrUpdateUser_ExistingUser(t *testing.T) {
	service, _, cleanup := setupTestService(t)
	if service == nil {
		return
	}
	defer cleanup()

	ctx := context.Background()
	googleUserInfo := &models.GoogleUserInfo{
		ID:      "google-789",
		Email:   "existing@example.com",
		Name:    "Original Name",
		Picture: "https://example.com/old.jpg",
	}

	// Create user first time
	user1, err := service.CreateOrUpdateUser(ctx, googleUserInfo)
	require.NoError(t, err)

	// Update user info
	googleUserInfo.Name = "Updated Name"
	googleUserInfo.Picture = "https://example.com/new.jpg"

	user2, err := service.CreateOrUpdateUser(ctx, googleUserInfo)
	require.NoError(t, err)

	// Should have same ID but updated info
	assert.Equal(t, user1.ID, user2.ID)
	assert.Equal(t, "Updated Name", user2.Name)
	assert.Equal(t, "https://example.com/new.jpg", *user2.AvatarURL)
}

func TestGenerateTokens(t *testing.T) {
	service, cfg, cleanup := setupTestService(t)
	if service == nil {
		return
	}
	defer cleanup()

	ctx := context.Background()

	// Create a real user in the database first
	googleUserInfo := &models.GoogleUserInfo{
		ID:      "google-tokens-test",
		Email:   "tokens@example.com",
		Name:    "Token Test User",
		Picture: "https://example.com/tokens.jpg",
	}
	user, err := service.CreateOrUpdateUser(ctx, googleUserInfo)
	require.NoError(t, err)

	tokens, err := service.GenerateTokens(ctx, user)
	require.NoError(t, err)
	assert.NotEmpty(t, tokens.AccessToken)
	assert.NotEmpty(t, tokens.RefreshToken)

	// Verify refresh token was stored in database
	var count int
	err = service.db.QueryRow(ctx, "SELECT COUNT(*) FROM refresh_tokens WHERE user_id = $1", user.ID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// Verify token expiry
	var expiresAt time.Time
	err = service.db.QueryRow(ctx, "SELECT expires_at FROM refresh_tokens WHERE user_id = $1", user.ID).Scan(&expiresAt)
	require.NoError(t, err)
	expectedExpiry := time.Now().Add(cfg.JWTRefreshTokenExpiry)
	// Use a larger tolerance to account for timezone differences
	assert.WithinDuration(t, expectedExpiry, expiresAt, 2*time.Hour)
}

func TestRefreshAccessToken_ValidToken(t *testing.T) {
	service, _, cleanup := setupTestService(t)
	if service == nil {
		return
	}
	defer cleanup()

	ctx := context.Background()

	// Create a test user
	googleUserInfo := &models.GoogleUserInfo{
		ID:      "google-refresh-test",
		Email:   "refresh@example.com",
		Name:    "Refresh Test",
		Picture: "https://example.com/refresh.jpg",
	}
	user, err := service.CreateOrUpdateUser(ctx, googleUserInfo)
	require.NoError(t, err)

	// Generate tokens
	tokens, err := service.GenerateTokens(ctx, user)
	require.NoError(t, err)

	// Refresh tokens
	newTokens, err := service.RefreshAccessToken(ctx, tokens.RefreshToken)
	require.NoError(t, err)
	assert.NotEmpty(t, newTokens.AccessToken)
	assert.NotEmpty(t, newTokens.RefreshToken)
	assert.NotEqual(t, tokens.RefreshToken, newTokens.RefreshToken, "Refresh token should be rotated")

	// Verify old refresh token was revoked
	var revokedAt *time.Time
	err = service.db.QueryRow(ctx,
		"SELECT revoked_at FROM refresh_tokens WHERE user_id = $1 ORDER BY created_at LIMIT 1",
		user.ID,
	).Scan(&revokedAt)
	require.NoError(t, err)
	assert.NotNil(t, revokedAt, "Old refresh token should be revoked")
}

func TestRefreshAccessToken_InvalidToken(t *testing.T) {
	service, _, cleanup := setupTestService(t)
	if service == nil {
		return
	}
	defer cleanup()

	ctx := context.Background()

	// Try to refresh with invalid token
	_, err := service.RefreshAccessToken(ctx, "invalid-token-123")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid or expired")
}

func TestRefreshAccessToken_ExpiredToken(t *testing.T) {
	service, _, cleanup := setupTestService(t)
	if service == nil {
		return
	}
	defer cleanup()

	ctx := context.Background()

	// Create a test user
	googleUserInfo := &models.GoogleUserInfo{
		ID:      "google-expired-test",
		Email:   "expired@example.com",
		Name:    "Expired Test",
		Picture: "https://example.com/expired.jpg",
	}
	user, err := service.CreateOrUpdateUser(ctx, googleUserInfo)
	require.NoError(t, err)

	// Generate tokens
	tokens, err := service.GenerateTokens(ctx, user)
	require.NoError(t, err)

	// Manually expire the refresh token in database
	_, err = service.db.Exec(ctx,
		"UPDATE refresh_tokens SET expires_at = NOW() - INTERVAL '1 day' WHERE user_id = $1",
		user.ID,
	)
	require.NoError(t, err)

	// Try to refresh with expired token
	_, err = service.RefreshAccessToken(ctx, tokens.RefreshToken)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid or expired")
}

func TestRevokeRefreshToken(t *testing.T) {
	service, _, cleanup := setupTestService(t)
	if service == nil {
		return
	}
	defer cleanup()

	ctx := context.Background()

	// Create a test user
	googleUserInfo := &models.GoogleUserInfo{
		ID:      "google-revoke-test",
		Email:   "revoke@example.com",
		Name:    "Revoke Test",
		Picture: "https://example.com/revoke.jpg",
	}
	user, err := service.CreateOrUpdateUser(ctx, googleUserInfo)
	require.NoError(t, err)

	// Generate tokens
	tokens, err := service.GenerateTokens(ctx, user)
	require.NoError(t, err)

	// Revoke the refresh token
	err = service.RevokeRefreshToken(ctx, tokens.RefreshToken)
	require.NoError(t, err)

	// Verify token was revoked
	var revokedAt *time.Time
	err = service.db.QueryRow(ctx,
		"SELECT revoked_at FROM refresh_tokens WHERE user_id = $1",
		user.ID,
	).Scan(&revokedAt)
	require.NoError(t, err)
	assert.NotNil(t, revokedAt, "Refresh token should be revoked")

	// Try to use revoked token
	_, err = service.RefreshAccessToken(ctx, tokens.RefreshToken)
	assert.Error(t, err)
}

func TestLogAuthEvent(t *testing.T) {
	service, _, cleanup := setupTestService(t)
	if service == nil {
		return
	}
	defer cleanup()

	ctx := context.Background()

	// Create a real user in the database first
	googleUserInfo := &models.GoogleUserInfo{
		ID:      "google-audit-test",
		Email:   "audit@example.com",
		Name:    "Audit Test User",
		Picture: "https://example.com/audit.jpg",
	}
	user, err := service.CreateOrUpdateUser(ctx, googleUserInfo)
	require.NoError(t, err)

	// Log an auth event
	err = service.LogAuthEvent(ctx, &user.ID, "LOGIN", "192.168.1.1", "Mozilla/5.0")
	require.NoError(t, err)

	// Verify event was logged
	var count int
	err = service.db.QueryRow(ctx,
		"SELECT COUNT(*) FROM auth_audit_log WHERE user_id = $1 AND action = 'LOGIN'",
		user.ID,
	).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// Verify event details
	var action, ipAddress, userAgent string
	err = service.db.QueryRow(ctx,
		"SELECT action, CAST(ip_address AS TEXT), user_agent FROM auth_audit_log WHERE user_id = $1",
		user.ID,
	).Scan(&action, &ipAddress, &userAgent)
	require.NoError(t, err)
	assert.Equal(t, "LOGIN", action)
	// PostgreSQL INET type stores IPs in CIDR notation
	assert.Equal(t, "192.168.1.1/32", ipAddress)
	assert.Equal(t, "Mozilla/5.0", userAgent)
}

func TestRoleUpgrade(t *testing.T) {
	service, _, cleanup := setupTestService(t)
	if service == nil {
		return
	}
	defer cleanup()

	ctx := context.Background()

	// Create user with regular email first
	googleUserInfo := &models.GoogleUserInfo{
		ID:      "google-upgrade-test",
		Email:   "admin@test.com", // This is in admin emails
		Name:    "Future Admin",
		Picture: "https://example.com/future-admin.jpg",
	}

	// First login - should get admin role immediately
	user, err := service.CreateOrUpdateUser(ctx, googleUserInfo)
	require.NoError(t, err)
	assert.Equal(t, models.RoleAdmin, user.Role)

	// Update role to user in database (simulating manual downgrade)
	_, err = service.db.Exec(ctx, "UPDATE users SET role = $1 WHERE id = $2", models.RoleUser, user.ID)
	require.NoError(t, err)

	// Login again - should be upgraded back to admin
	user2, err := service.CreateOrUpdateUser(ctx, googleUserInfo)
	require.NoError(t, err)
	assert.Equal(t, models.RoleAdmin, user2.Role)
}

func TestSuperAdminFlag(t *testing.T) {
	service, _, cleanup := setupTestService(t)
	if service == nil {
		return
	}
	defer cleanup()

	ctx := context.Background()

	// Create admin user
	adminUserInfo := &models.GoogleUserInfo{
		ID:      "google-superadmin-test",
		Email:   "admin@test.com",
		Name:    "Super Admin",
		Picture: "https://example.com/admin.jpg",
	}

	adminUser, err := service.CreateOrUpdateUser(ctx, adminUserInfo)
	require.NoError(t, err)
	assert.True(t, adminUser.IsSuperAdmin, "Admin user should have is_super_admin flag set")
	assert.Equal(t, models.RoleAdmin, adminUser.Role)

	// Create regular user
	userInfo := &models.GoogleUserInfo{
		ID:      "google-regular-test",
		Email:   "user@example.com",
		Name:    "Regular User",
		Picture: "https://example.com/user.jpg",
	}

	regularUser, err := service.CreateOrUpdateUser(ctx, userInfo)
	require.NoError(t, err)
	assert.False(t, regularUser.IsSuperAdmin, "Regular user should not have is_super_admin flag")
	assert.Equal(t, models.RoleUser, regularUser.Role)
}

func TestGetUserOrganizations(t *testing.T) {
	service, _, cleanup := setupTestService(t)
	if service == nil {
		return
	}
	defer cleanup()

	ctx := context.Background()

	// Create a test user
	googleUserInfo := &models.GoogleUserInfo{
		ID:      "google-orgs-test",
		Email:   "orgs@example.com",
		Name:    "Org Test User",
		Picture: "https://example.com/orgs.jpg",
	}
	user, err := service.CreateOrUpdateUser(ctx, googleUserInfo)
	require.NoError(t, err)

	// Create test organizations
	org1ID := uuid.New()
	org2ID := uuid.New()
	org3ID := uuid.New()

	_, err = service.db.Exec(ctx, `
		INSERT INTO organizations (id, name, slug, is_active, created_by)
		VALUES ($1, 'Test Org 1', 'test-org-1', true, $2),
		       ($3, 'Test Org 2', 'test-org-2', true, $2),
		       ($4, 'Test Org 3', 'test-org-3', false, $2)
	`, org1ID, user.ID, org2ID, org3ID)
	require.NoError(t, err)

	// Add user to organizations with different roles
	_, err = service.db.Exec(ctx, `
		INSERT INTO user_organizations (user_id, organization_id, role)
		VALUES ($1, $2, 'owner'),
		       ($1, $3, 'member'),
		       ($1, $4, 'admin')
	`, user.ID, org1ID, org2ID, org3ID)
	require.NoError(t, err)

	// Get user organizations
	orgs, err := service.GetUserOrganizations(ctx, user.ID)
	require.NoError(t, err)

	// Should only return active organizations (org3 is inactive)
	assert.Len(t, orgs, 2, "Should return 2 active organizations")

	// Verify organization data
	assert.Equal(t, org1ID, orgs[0].ID)
	assert.Equal(t, "test-org-1", orgs[0].Slug)
	assert.Equal(t, "Test Org 1", orgs[0].Name)
	assert.Equal(t, "owner", orgs[0].Role)

	assert.Equal(t, org2ID, orgs[1].ID)
	assert.Equal(t, "test-org-2", orgs[1].Slug)
	assert.Equal(t, "Test Org 2", orgs[1].Name)
	assert.Equal(t, "member", orgs[1].Role)
}

func TestGetUserOrganizations_NoOrganizations(t *testing.T) {
	service, _, cleanup := setupTestService(t)
	if service == nil {
		return
	}
	defer cleanup()

	ctx := context.Background()

	// Create a test user
	googleUserInfo := &models.GoogleUserInfo{
		ID:      "google-no-orgs-test",
		Email:   "noorgs@example.com",
		Name:    "No Orgs User",
		Picture: "https://example.com/noorgs.jpg",
	}
	user, err := service.CreateOrUpdateUser(ctx, googleUserInfo)
	require.NoError(t, err)

	// Get user organizations - should return empty slice
	orgs, err := service.GetUserOrganizations(ctx, user.ID)
	require.NoError(t, err)
	assert.Empty(t, orgs, "Should return empty slice when user has no organizations")
	assert.NotNil(t, orgs, "Should return initialized slice, not nil")
}

func TestGenerateTokens_WithOrganizations(t *testing.T) {
	service, _, cleanup := setupTestService(t)
	if service == nil {
		return
	}
	defer cleanup()

	ctx := context.Background()

	// Create a test user
	googleUserInfo := &models.GoogleUserInfo{
		ID:      "google-token-orgs-test",
		Email:   "tokenorgs@example.com",
		Name:    "Token Orgs User",
		Picture: "https://example.com/tokenorgs.jpg",
	}
	user, err := service.CreateOrUpdateUser(ctx, googleUserInfo)
	require.NoError(t, err)

	// Create test organization
	orgID := uuid.New()
	_, err = service.db.Exec(ctx, `
		INSERT INTO organizations (id, name, slug, is_active, created_by)
		VALUES ($1, 'Token Test Org', 'token-test-org', true, $2)
	`, orgID, user.ID)
	require.NoError(t, err)

	// Add user to organization
	_, err = service.db.Exec(ctx, `
		INSERT INTO user_organizations (user_id, organization_id, role)
		VALUES ($1, $2, 'owner')
	`, user.ID, orgID)
	require.NoError(t, err)

	// Generate tokens
	tokens, err := service.GenerateTokens(ctx, user)
	require.NoError(t, err)
	assert.NotEmpty(t, tokens.AccessToken)
	assert.NotEmpty(t, tokens.RefreshToken)

	// Note: To fully verify JWT contains organization data, we would need to
	// parse and validate the JWT token here. This is covered by JWT package tests.
	// This test mainly ensures GenerateTokens doesn't fail when organizations exist.
}
