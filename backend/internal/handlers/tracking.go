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

// GetLoginStats returns aggregated login statistics
func (h *Handler) GetLoginStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Only super admins can view overall stats
	if !middleware.IsSuperAdmin(ctx) {
		http.Error(w, "Access denied - super admin required", http.StatusForbidden)
		return
	}

	// Get overall stats
	var stats struct {
		TotalLogins           int `json:"total_logins"`
		UniqueUsers           int `json:"unique_users"`
		UniqueApplications    int `json:"unique_applications"`
		LoginsLast24Hours     int `json:"logins_last_24_hours"`
		LoginsLast7Days       int `json:"logins_last_7_days"`
		LoginsLast30Days      int `json:"logins_last_30_days"`
	}

	// Total logins
	err := h.db.QueryRow(ctx, `SELECT COUNT(*) FROM user_application_logins`).Scan(&stats.TotalLogins)
	if err != nil {
		h.logger.Printf("Failed to get total logins: %v", err)
	}

	// Unique users
	err = h.db.QueryRow(ctx, `SELECT COUNT(DISTINCT user_id) FROM user_application_logins`).Scan(&stats.UniqueUsers)
	if err != nil {
		h.logger.Printf("Failed to get unique users: %v", err)
	}

	// Unique applications
	err = h.db.QueryRow(ctx, `SELECT COUNT(DISTINCT application_id) FROM user_application_logins`).Scan(&stats.UniqueApplications)
	if err != nil {
		h.logger.Printf("Failed to get unique applications: %v", err)
	}

	// Logins in last 24 hours
	err = h.db.QueryRow(ctx, `SELECT COUNT(*) FROM user_application_logins WHERE created_at > NOW() - INTERVAL '24 hours'`).Scan(&stats.LoginsLast24Hours)
	if err != nil {
		h.logger.Printf("Failed to get logins last 24 hours: %v", err)
	}

	// Logins in last 7 days
	err = h.db.QueryRow(ctx, `SELECT COUNT(*) FROM user_application_logins WHERE created_at > NOW() - INTERVAL '7 days'`).Scan(&stats.LoginsLast7Days)
	if err != nil {
		h.logger.Printf("Failed to get logins last 7 days: %v", err)
	}

	// Logins in last 30 days
	err = h.db.QueryRow(ctx, `SELECT COUNT(*) FROM user_application_logins WHERE created_at > NOW() - INTERVAL '30 days'`).Scan(&stats.LoginsLast30Days)
	if err != nil {
		h.logger.Printf("Failed to get logins last 30 days: %v", err)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// GetUserLoginStats returns aggregated login statistics by user
func (h *Handler) GetUserLoginStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Only super admins can view user stats
	if !middleware.IsSuperAdmin(ctx) {
		http.Error(w, "Access denied - super admin required", http.StatusForbidden)
		return
	}

	query := `
		SELECT
			u.id,
			u.email,
			u.name,
			COUNT(l.id) as login_count,
			COUNT(DISTINCT l.application_id) as unique_apps,
			MAX(l.created_at) as last_login
		FROM users u
		LEFT JOIN user_application_logins l ON u.id = l.user_id
		GROUP BY u.id, u.email, u.name
		ORDER BY login_count DESC
		LIMIT 100
	`

	rows, err := h.db.Query(ctx, query)
	if err != nil {
		h.logger.Printf("Failed to query user login stats: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type UserStats struct {
		UserID      uuid.UUID  `json:"user_id"`
		Email       string     `json:"email"`
		Name        string     `json:"name"`
		LoginCount  int        `json:"login_count"`
		UniqueApps  int        `json:"unique_apps"`
		LastLogin   *string    `json:"last_login"`
	}

	var stats []UserStats
	for rows.Next() {
		var s UserStats
		if err := rows.Scan(&s.UserID, &s.Email, &s.Name, &s.LoginCount, &s.UniqueApps, &s.LastLogin); err != nil {
			h.logger.Printf("Failed to scan user stats: %v", err)
			continue
		}
		stats = append(stats, s)
	}

	if stats == nil {
		stats = []UserStats{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// GetApplicationLoginStats returns aggregated login statistics by application
func (h *Handler) GetApplicationLoginStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Only super admins can view application stats
	if !middleware.IsSuperAdmin(ctx) {
		http.Error(w, "Access denied - super admin required", http.StatusForbidden)
		return
	}

	query := `
		SELECT
			a.id,
			a.name,
			a.slug,
			COUNT(l.id) as login_count,
			COUNT(DISTINCT l.user_id) as unique_users,
			MAX(l.created_at) as last_login
		FROM applications a
		LEFT JOIN user_application_logins l ON a.id = l.application_id
		WHERE a.is_active = true
		GROUP BY a.id, a.name, a.slug
		ORDER BY login_count DESC
	`

	rows, err := h.db.Query(ctx, query)
	if err != nil {
		h.logger.Printf("Failed to query application login stats: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type AppStats struct {
		AppID       uuid.UUID  `json:"app_id"`
		Name        string     `json:"name"`
		Slug        string     `json:"slug"`
		LoginCount  int        `json:"login_count"`
		UniqueUsers int        `json:"unique_users"`
		LastLogin   *string    `json:"last_login"`
	}

	var stats []AppStats
	for rows.Next() {
		var s AppStats
		if err := rows.Scan(&s.AppID, &s.Name, &s.Slug, &s.LoginCount, &s.UniqueUsers, &s.LastLogin); err != nil {
			h.logger.Printf("Failed to scan app stats: %v", err)
			continue
		}
		stats = append(stats, s)
	}

	if stats == nil {
		stats = []AppStats{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// GetMyLoginStats returns login statistics for the current user
func (h *Handler) GetMyLoginStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := ctx.Value(middleware.UserIDKey).(uuid.UUID)

	// Get user's personal stats
	var stats struct {
		TotalLogins        int `json:"total_logins"`
		UniqueApplications int `json:"unique_applications"`
		LoginsLast7Days    int `json:"logins_last_7_days"`
		LoginsLast30Days   int `json:"logins_last_30_days"`
	}

	// Total logins
	err := h.db.QueryRow(ctx, `SELECT COUNT(*) FROM user_application_logins WHERE user_id = $1`, userID).Scan(&stats.TotalLogins)
	if err != nil {
		h.logger.Printf("Failed to get total logins: %v", err)
	}

	// Unique applications
	err = h.db.QueryRow(ctx, `SELECT COUNT(DISTINCT application_id) FROM user_application_logins WHERE user_id = $1`, userID).Scan(&stats.UniqueApplications)
	if err != nil {
		h.logger.Printf("Failed to get unique applications: %v", err)
	}

	// Logins in last 7 days
	err = h.db.QueryRow(ctx, `SELECT COUNT(*) FROM user_application_logins WHERE user_id = $1 AND created_at > NOW() - INTERVAL '7 days'`, userID).Scan(&stats.LoginsLast7Days)
	if err != nil {
		h.logger.Printf("Failed to get logins last 7 days: %v", err)
	}

	// Logins in last 30 days
	err = h.db.QueryRow(ctx, `SELECT COUNT(*) FROM user_application_logins WHERE user_id = $1 AND created_at > NOW() - INTERVAL '30 days'`, userID).Scan(&stats.LoginsLast30Days)
	if err != nil {
		h.logger.Printf("Failed to get logins last 30 days: %v", err)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// GetMyLoginsByApp returns the current user's login count by application
func (h *Handler) GetMyLoginsByApp(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := ctx.Value(middleware.UserIDKey).(uuid.UUID)

	query := `
		SELECT
			a.id,
			a.name,
			a.slug,
			COUNT(l.id) as login_count,
			MAX(l.created_at) as last_login
		FROM applications a
		JOIN user_application_logins l ON a.id = l.application_id
		WHERE l.user_id = $1
		GROUP BY a.id, a.name, a.slug
		ORDER BY login_count DESC
	`

	rows, err := h.db.Query(ctx, query, userID)
	if err != nil {
		h.logger.Printf("Failed to query user app login stats: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type AppLoginStats struct {
		AppID      uuid.UUID `json:"app_id"`
		Name       string    `json:"name"`
		Slug       string    `json:"slug"`
		LoginCount int       `json:"login_count"`
		LastLogin  string    `json:"last_login"`
	}

	var stats []AppLoginStats
	for rows.Next() {
		var s AppLoginStats
		if err := rows.Scan(&s.AppID, &s.Name, &s.Slug, &s.LoginCount, &s.LastLogin); err != nil {
			h.logger.Printf("Failed to scan app login stats: %v", err)
			continue
		}
		stats = append(stats, s)
	}

	if stats == nil {
		stats = []AppLoginStats{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}
