package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/frans-sjostrom/auth-service/internal/middleware"
	"github.com/frans-sjostrom/auth-service/internal/models"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// TrackLogin records a user login to an application
func (h *Handler) TrackLogin(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := ctx.Value(middleware.UserIDKey).(uuid.UUID)

	var req struct {
		ApplicationSlug string     `json:"application_slug"`
		ApplicationID   *uuid.UUID `json:"application_id"` // Alternative to slug
		OrganizationID  *uuid.UUID `json:"organization_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Look up application by slug or ID
	var appID uuid.UUID
	if req.ApplicationID != nil {
		appID = *req.ApplicationID
	} else if req.ApplicationSlug != "" {
		appQuery := `SELECT id FROM applications WHERE slug = $1 AND is_active = true`
		err := h.db.QueryRow(ctx, appQuery, req.ApplicationSlug).Scan(&appID)
		if err == sql.ErrNoRows {
			http.Error(w, "Application not found", http.StatusNotFound)
			return
		}
		if err != nil {
			h.logger.Printf("Failed to look up application: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	} else {
		http.Error(w, "Either application_slug or application_id must be provided", http.StatusBadRequest)
		return
	}

	// Get IP address and user agent
	ipAddress := getRealIP(r)
	userAgent := r.UserAgent()

	// Record login
	query := `
		INSERT INTO user_application_logins (user_id, application_id, organization_id, ip_address, user_agent)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, user_id, application_id, organization_id, ip_address, user_agent, created_at
	`

	var login models.UserApplicationLogin
	err := h.db.QueryRow(ctx, query, userID, appID, req.OrganizationID, ipAddress, userAgent).
		Scan(&login.ID, &login.UserID, &login.ApplicationID, &login.OrganizationID, &login.IPAddress, &login.UserAgent, &login.CreatedAt)

	if err != nil {
		h.logger.Printf("Failed to track login: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(login)
}

// GetUserLoginHistory returns login history for a user
func (h *Handler) GetUserLoginHistory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userIDStr := chi.URLParam(r, "id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	// Check permissions - users can see their own history, admins can see any
	currentUserID := ctx.Value(middleware.UserIDKey).(uuid.UUID)
	if !middleware.IsSuperAdmin(ctx) && currentUserID != userID {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	// Parse query parameters
	limit := 100
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsedLimit, err := parseInt(limitStr); err == nil && parsedLimit > 0 && parsedLimit <= 1000 {
			limit = parsedLimit
		}
	}

	query := `
		SELECT l.id, l.user_id, l.application_id, l.organization_id, l.ip_address, l.user_agent, l.created_at,
		       a.name as app_name, a.slug as app_slug
		FROM user_application_logins l
		JOIN applications a ON a.id = l.application_id
		WHERE l.user_id = $1
		ORDER BY l.created_at DESC
		LIMIT $2
	`

	rows, err := h.db.Query(ctx, query, userID, limit)
	if err != nil {
		h.logger.Printf("Failed to query login history: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type LoginWithApp struct {
		models.UserApplicationLogin
		AppName string `json:"app_name"`
		AppSlug string `json:"app_slug"`
	}

	var logins []LoginWithApp
	for rows.Next() {
		var login LoginWithApp
		if err := rows.Scan(&login.ID, &login.UserID, &login.ApplicationID, &login.OrganizationID,
			&login.IPAddress, &login.UserAgent, &login.CreatedAt, &login.AppName, &login.AppSlug); err != nil {
			h.logger.Printf("Failed to scan login: %v", err)
			continue
		}
		logins = append(logins, login)
	}

	if logins == nil {
		logins = []LoginWithApp{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(logins)
}

// GetApplicationLoginHistory returns login history for an application
func (h *Handler) GetApplicationLoginHistory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	appIDStr := chi.URLParam(r, "id")
	appID, err := uuid.Parse(appIDStr)
	if err != nil {
		http.Error(w, "Invalid application ID", http.StatusBadRequest)
		return
	}

	// Only super admins can view application login history
	if !middleware.IsSuperAdmin(ctx) {
		http.Error(w, "Access denied - super admin required", http.StatusForbidden)
		return
	}

	// Parse query parameters
	limit := 100
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsedLimit, err := parseInt(limitStr); err == nil && parsedLimit > 0 && parsedLimit <= 1000 {
			limit = parsedLimit
		}
	}

	query := `
		SELECT l.id, l.user_id, l.application_id, l.organization_id, l.ip_address, l.user_agent, l.created_at,
		       u.email, u.name as user_name
		FROM user_application_logins l
		JOIN users u ON u.id = l.user_id
		WHERE l.application_id = $1
		ORDER BY l.created_at DESC
		LIMIT $2
	`

	rows, err := h.db.Query(ctx, query, appID, limit)
	if err != nil {
		h.logger.Printf("Failed to query application login history: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type LoginWithUser struct {
		models.UserApplicationLogin
		Email    string `json:"email"`
		UserName string `json:"user_name"`
	}

	var logins []LoginWithUser
	for rows.Next() {
		var login LoginWithUser
		if err := rows.Scan(&login.ID, &login.UserID, &login.ApplicationID, &login.OrganizationID,
			&login.IPAddress, &login.UserAgent, &login.CreatedAt, &login.Email, &login.UserName); err != nil {
			h.logger.Printf("Failed to scan login: %v", err)
			continue
		}
		logins = append(logins, login)
	}

	if logins == nil {
		logins = []LoginWithUser{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(logins)
}

// GetOrganizationLoginHistory returns login history for an organization
func (h *Handler) GetOrganizationLoginHistory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgIDStr := chi.URLParam(r, "id")
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		http.Error(w, "Invalid organization ID", http.StatusBadRequest)
		return
	}

	// Check permissions - must be owner or admin of the organization
	if !middleware.IsSuperAdmin(ctx) && !middleware.IsOrgOwnerOrAdmin(ctx, orgID) {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	// Parse query parameters
	limit := 100
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsedLimit, err := parseInt(limitStr); err == nil && parsedLimit > 0 && parsedLimit <= 1000 {
			limit = parsedLimit
		}
	}

	query := `
		SELECT l.id, l.user_id, l.application_id, l.organization_id, l.ip_address, l.user_agent, l.created_at,
		       u.email, u.name as user_name, a.name as app_name, a.slug as app_slug
		FROM user_application_logins l
		JOIN users u ON u.id = l.user_id
		JOIN applications a ON a.id = l.application_id
		WHERE l.organization_id = $1
		ORDER BY l.created_at DESC
		LIMIT $2
	`

	rows, err := h.db.Query(ctx, query, orgID, limit)
	if err != nil {
		h.logger.Printf("Failed to query organization login history: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type LoginWithDetails struct {
		models.UserApplicationLogin
		Email    string `json:"email"`
		UserName string `json:"user_name"`
		AppName  string `json:"app_name"`
		AppSlug  string `json:"app_slug"`
	}

	var logins []LoginWithDetails
	for rows.Next() {
		var login LoginWithDetails
		if err := rows.Scan(&login.ID, &login.UserID, &login.ApplicationID, &login.OrganizationID,
			&login.IPAddress, &login.UserAgent, &login.CreatedAt, &login.Email, &login.UserName,
			&login.AppName, &login.AppSlug); err != nil {
			h.logger.Printf("Failed to scan login: %v", err)
			continue
		}
		logins = append(logins, login)
	}

	if logins == nil {
		logins = []LoginWithDetails{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(logins)
}

// Helper functions
func getRealIP(r *http.Request) *string {
	// Try to get real IP from common headers
	ip := r.Header.Get("X-Real-IP")
	if ip == "" {
		ip = r.Header.Get("X-Forwarded-For")
	}
	if ip == "" {
		ip = r.RemoteAddr
	}
	if ip == "" {
		return nil
	}
	return &ip
}

func parseInt(s string) (int, error) {
	var i int
	err := json.Unmarshal([]byte(s), &i)
	return i, err
}
