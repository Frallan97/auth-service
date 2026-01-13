package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/frans-sjostrom/auth-service/internal/config"
	"github.com/frans-sjostrom/auth-service/internal/database"
	"github.com/frans-sjostrom/auth-service/internal/middleware"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTrackingTest(t *testing.T) (*Handler, *database.DB, func()) {
	cfg := &config.Config{
		DatabaseURL: "postgresql://authuser:authpass@localhost:5433/authdb?sslmode=disable",
	}

	db, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		t.Skip("Test database not available, skipping integration tests")
		return nil, nil, nil
	}

	handler := &Handler{
		db:  db,
		cfg: cfg,
	}

	cleanup := func() {
		// Clean up test data
		db.Exec(context.Background(), "DELETE FROM user_application_logins")
		db.Exec(context.Background(), "DELETE FROM user_organizations")
		db.Exec(context.Background(), "DELETE FROM organizations")
		db.Exec(context.Background(), "DELETE FROM applications")
		db.Exec(context.Background(), "DELETE FROM users WHERE email LIKE '%@test-tracking.com'")
		db.Close()
	}

	return handler, db, cleanup
}

func createTestApplication(t *testing.T, db *database.DB, slug string) uuid.UUID {
	appID := uuid.New()
	_, err := db.Exec(context.Background(), `
		INSERT INTO applications (id, name, slug, origin, is_active)
		VALUES ($1, $2, $3, $4, true)
	`, appID, "Test App "+slug, slug, "https://"+slug+".example.com")
	require.NoError(t, err)
	return appID
}

func createTestUserWithContext(t *testing.T, db *database.DB, email string) (uuid.UUID, context.Context) {
	userID := uuid.New()
	_, err := db.Exec(context.Background(), `
		INSERT INTO users (id, email, name, role, is_super_admin, is_active)
		VALUES ($1, $2, $3, 'user', false, true)
	`, userID, email, "Test User")
	require.NoError(t, err)

	// Create context with user ID
	ctx := context.WithValue(context.Background(), middleware.UserIDKey, userID)
	return userID, ctx
}

func TestTrackLogin_WithApplicationSlug(t *testing.T) {
	handler, db, cleanup := setupTrackingTest(t)
	if handler == nil {
		return
	}
	defer cleanup()

	// Create test user and application
	userID, ctx := createTestUserWithContext(t, db, "user1@test-tracking.com")
	appID := createTestApplication(t, db, "test-app-1")

	// Create request
	reqBody := map[string]interface{}{
		"application_slug": "test-app-1",
	}
	bodyJSON, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/track-login", bytes.NewReader(bodyJSON))
	req = req.WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Real-IP", "192.168.1.100")
	req.Header.Set("User-Agent", "Test Browser")

	// Execute request
	w := httptest.NewRecorder()
	handler.TrackLogin(w, req)

	// Verify response
	assert.Equal(t, http.StatusCreated, w.Code)

	// Verify login was recorded
	var count int
	err := db.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM user_application_logins
		WHERE user_id = $1 AND application_id = $2
	`, userID, appID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// Verify IP and user agent were recorded
	var ipAddress, userAgent string
	err = db.QueryRow(context.Background(), `
		SELECT CAST(ip_address AS TEXT), user_agent FROM user_application_logins
		WHERE user_id = $1 AND application_id = $2
	`, userID, appID).Scan(&ipAddress, &userAgent)
	require.NoError(t, err)
	assert.Equal(t, "192.168.1.100/32", ipAddress)
	assert.Equal(t, "Test Browser", userAgent)
}

func TestTrackLogin_WithApplicationID(t *testing.T) {
	handler, db, cleanup := setupTrackingTest(t)
	if handler == nil {
		return
	}
	defer cleanup()

	// Create test user and application
	userID, ctx := createTestUserWithContext(t, db, "user2@test-tracking.com")
	appID := createTestApplication(t, db, "test-app-2")

	// Create request with application ID instead of slug
	reqBody := map[string]interface{}{
		"application_id": appID.String(),
	}
	bodyJSON, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/track-login", bytes.NewReader(bodyJSON))
	req = req.WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")

	// Execute request
	w := httptest.NewRecorder()
	handler.TrackLogin(w, req)

	// Verify response
	assert.Equal(t, http.StatusCreated, w.Code)

	// Verify login was recorded
	var count int
	err := db.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM user_application_logins
		WHERE user_id = $1 AND application_id = $2
	`, userID, appID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestTrackLogin_WithOrganization(t *testing.T) {
	handler, db, cleanup := setupTrackingTest(t)
	if handler == nil {
		return
	}
	defer cleanup()

	// Create test user, application, and organization
	userID, ctx := createTestUserWithContext(t, db, "user3@test-tracking.com")
	appID := createTestApplication(t, db, "test-app-3")

	orgID := uuid.New()
	_, err := db.Exec(context.Background(), `
		INSERT INTO organizations (id, name, slug, is_active, created_by)
		VALUES ($1, 'Test Org', 'test-org', true, $2)
	`, orgID, userID)
	require.NoError(t, err)

	// Create request with organization ID
	reqBody := map[string]interface{}{
		"application_slug": "test-app-3",
		"organization_id":  orgID.String(),
	}
	bodyJSON, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/track-login", bytes.NewReader(bodyJSON))
	req = req.WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")

	// Execute request
	w := httptest.NewRecorder()
	handler.TrackLogin(w, req)

	// Verify response
	assert.Equal(t, http.StatusCreated, w.Code)

	// Verify organization was recorded
	var recordedOrgID *uuid.UUID
	err = db.QueryRow(context.Background(), `
		SELECT organization_id FROM user_application_logins
		WHERE user_id = $1 AND application_id = $2
	`, userID, appID).Scan(&recordedOrgID)
	require.NoError(t, err)
	assert.NotNil(t, recordedOrgID)
	assert.Equal(t, orgID, *recordedOrgID)
}

func TestTrackLogin_InvalidApplicationSlug(t *testing.T) {
	handler, _, cleanup := setupTrackingTest(t)
	if handler == nil {
		return
	}
	defer cleanup()

	// Create test user
	_, ctx := createTestUserWithContext(t, handler.db, "user4@test-tracking.com")

	// Create request with non-existent application slug
	reqBody := map[string]interface{}{
		"application_slug": "non-existent-app",
	}
	bodyJSON, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/track-login", bytes.NewReader(bodyJSON))
	req = req.WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")

	// Execute request
	w := httptest.NewRecorder()
	handler.TrackLogin(w, req)

	// Should return 404
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetUserLoginHistory(t *testing.T) {
	handler, db, cleanup := setupTrackingTest(t)
	if handler == nil {
		return
	}
	defer cleanup()

	// Create test user and application
	userID, ctx := createTestUserWithContext(t, db, "user5@test-tracking.com")
	appID := createTestApplication(t, db, "test-app-5")

	// Add context values for authorization
	ctx = context.WithValue(ctx, middleware.IsSuperAdminKey, false)

	// Create login records
	_, err := db.Exec(context.Background(), `
		INSERT INTO user_application_logins (user_id, application_id, ip_address, user_agent)
		VALUES ($1, $2, '192.168.1.1', 'Browser 1'),
		       ($1, $2, '192.168.1.2', 'Browser 2')
	`, userID, appID)
	require.NoError(t, err)

	// Note: This test verifies the data exists in the database
	// Full handler testing would require chi router setup
	var count int
	err = db.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM user_application_logins WHERE user_id = $1
	`, userID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 2, count)
}

func TestGetRealIP(t *testing.T) {
	tests := []struct {
		name     string
		headers  map[string]string
		expected string
	}{
		{
			name:     "X-Real-IP header",
			headers:  map[string]string{"X-Real-IP": "1.2.3.4"},
			expected: "1.2.3.4",
		},
		{
			name:     "X-Forwarded-For header",
			headers:  map[string]string{"X-Forwarded-For": "5.6.7.8"},
			expected: "5.6.7.8",
		},
		{
			name:     "X-Real-IP takes precedence",
			headers:  map[string]string{"X-Real-IP": "1.2.3.4", "X-Forwarded-For": "5.6.7.8"},
			expected: "1.2.3.4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			ip := getRealIP(req)
			require.NotNil(t, ip)
			assert.Equal(t, tt.expected, *ip)
		})
	}
}

func TestParseInt(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expected  int
		shouldErr bool
	}{
		{
			name:      "Valid integer",
			input:     "42",
			expected:  42,
			shouldErr: false,
		},
		{
			name:      "Invalid string",
			input:     "not-a-number",
			expected:  0,
			shouldErr: true,
		},
		{
			name:      "Negative integer",
			input:     "-10",
			expected:  -10,
			shouldErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseInt(tt.input)
			if tt.shouldErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}
