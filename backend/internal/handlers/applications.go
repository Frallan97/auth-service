package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"regexp"
	"strings"

	"github.com/frans-sjostrom/auth-service/internal/middleware"
	"github.com/frans-sjostrom/auth-service/internal/models"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// ListApplications returns all registered applications (super admin only)
func (h *Handler) ListApplications(w http.ResponseWriter, r *http.Request) {
	query := `
		SELECT id, name, slug, origin, description, redirect_uris, is_active, created_at, updated_at, created_by
		FROM applications
		ORDER BY created_at DESC
	`

	rows, err := h.db.Query(r.Context(), query)
	if err != nil {
		h.logger.Printf("Failed to query applications: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var applications []models.Application
	for rows.Next() {
		var app models.Application
		if err := rows.Scan(&app.ID, &app.Name, &app.Slug, &app.Origin, &app.Description, &app.RedirectURIs, &app.IsActive, &app.CreatedAt, &app.UpdatedAt, &app.CreatedBy); err != nil {
			h.logger.Printf("Failed to scan application: %v", err)
			continue
		}
		applications = append(applications, app)
	}

	if applications == nil {
		applications = []models.Application{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(applications)
}

// CreateApplication creates a new registered application (super admin only)
func (h *Handler) CreateApplication(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name         string   `json:"name"`
		Slug         string   `json:"slug"`
		Origin       string   `json:"origin"`
		Description  *string  `json:"description"`
		RedirectURIs []string `json:"redirect_uris"`
		IsActive     *bool    `json:"is_active"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validation
	if req.Name == "" {
		http.Error(w, "Name is required", http.StatusBadRequest)
		return
	}
	if req.Origin == "" {
		http.Error(w, "Origin is required", http.StatusBadRequest)
		return
	}

	// Generate slug if not provided
	if req.Slug == "" {
		req.Slug = generateSlug(req.Name)
	} else {
		// Validate slug format
		if !isValidSlug(req.Slug) {
			http.Error(w, "Slug must be lowercase alphanumeric with hyphens", http.StatusBadRequest)
			return
		}
	}

	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	// Get current user from context
	userID := r.Context().Value(middleware.UserIDKey)

	query := `
		INSERT INTO applications (name, slug, origin, description, redirect_uris, is_active, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, name, slug, origin, description, redirect_uris, is_active, created_at, updated_at, created_by
	`

	var app models.Application
	err := h.db.QueryRow(r.Context(), query, req.Name, req.Slug, req.Origin, req.Description, req.RedirectURIs, isActive, userID).
		Scan(&app.ID, &app.Name, &app.Slug, &app.Origin, &app.Description, &app.RedirectURIs, &app.IsActive, &app.CreatedAt, &app.UpdatedAt, &app.CreatedBy)

	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			if strings.Contains(err.Error(), "slug") {
				http.Error(w, "Slug already exists", http.StatusConflict)
			} else if strings.Contains(err.Error(), "origin") {
				http.Error(w, "Origin already exists", http.StatusConflict)
			} else {
				http.Error(w, "Application already exists", http.StatusConflict)
			}
			return
		}
		h.logger.Printf("Failed to create application: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Refresh CORS cache immediately
	if h.corsMiddleware != nil {
		if err := h.corsMiddleware.RefreshCache(); err != nil {
			h.logger.Printf("Warning: Failed to refresh CORS cache: %v", err)
		} else {
			h.logger.Printf("CORS cache refreshed after creating application: %s", app.Name)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(app)
}

// GetApplication returns a single application by ID
func (h *Handler) GetApplication(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "Invalid application ID", http.StatusBadRequest)
		return
	}

	query := `
		SELECT id, name, slug, origin, description, redirect_uris, is_active, created_at, updated_at, created_by
		FROM applications
		WHERE id = $1
	`

	var app models.Application
	err = h.db.QueryRow(r.Context(), query, id).
		Scan(&app.ID, &app.Name, &app.Slug, &app.Origin, &app.Description, &app.RedirectURIs, &app.IsActive, &app.CreatedAt, &app.UpdatedAt, &app.CreatedBy)

	if err == sql.ErrNoRows {
		http.Error(w, "Application not found", http.StatusNotFound)
		return
	}
	if err != nil {
		h.logger.Printf("Failed to get application: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(app)
}

// UpdateApplication updates an existing application (super admin only)
func (h *Handler) UpdateApplication(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "Invalid application ID", http.StatusBadRequest)
		return
	}

	var req struct {
		Name         *string  `json:"name"`
		Slug         *string  `json:"slug"`
		Origin       *string  `json:"origin"`
		Description  *string  `json:"description"`
		RedirectURIs []string `json:"redirect_uris"`
		IsActive     *bool    `json:"is_active"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Build dynamic update query
	updates := []string{}
	args := []interface{}{}
	argCount := 1

	if req.Name != nil {
		updates = append(updates, "name = $"+string(rune(argCount+'0')))
		args = append(args, *req.Name)
		argCount++
	}
	if req.Slug != nil {
		if !isValidSlug(*req.Slug) {
			http.Error(w, "Slug must be lowercase alphanumeric with hyphens", http.StatusBadRequest)
			return
		}
		updates = append(updates, "slug = $"+string(rune(argCount+'0')))
		args = append(args, *req.Slug)
		argCount++
	}
	if req.Origin != nil {
		updates = append(updates, "origin = $"+string(rune(argCount+'0')))
		args = append(args, *req.Origin)
		argCount++
	}
	if req.Description != nil {
		updates = append(updates, "description = $"+string(rune(argCount+'0')))
		args = append(args, *req.Description)
		argCount++
	}
	if req.RedirectURIs != nil {
		updates = append(updates, "redirect_uris = $"+string(rune(argCount+'0')))
		args = append(args, req.RedirectURIs)
		argCount++
	}
	if req.IsActive != nil {
		updates = append(updates, "is_active = $"+string(rune(argCount+'0')))
		args = append(args, *req.IsActive)
		argCount++
	}

	if len(updates) == 0 {
		http.Error(w, "No fields to update", http.StatusBadRequest)
		return
	}

	updates = append(updates, "updated_at = NOW()")
	args = append(args, id)

	query := "UPDATE applications SET " + strings.Join(updates, ", ") + " WHERE id = $" + string(rune(argCount+'0')) +
		" RETURNING id, name, slug, origin, description, redirect_uris, is_active, created_at, updated_at, created_by"

	var app models.Application
	err = h.db.QueryRow(r.Context(), query, args...).
		Scan(&app.ID, &app.Name, &app.Slug, &app.Origin, &app.Description, &app.RedirectURIs, &app.IsActive, &app.CreatedAt, &app.UpdatedAt, &app.CreatedBy)

	if err == sql.ErrNoRows {
		http.Error(w, "Application not found", http.StatusNotFound)
		return
	}
	if err != nil {
		h.logger.Printf("Failed to update application: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Refresh CORS cache immediately
	if h.corsMiddleware != nil {
		if err := h.corsMiddleware.RefreshCache(); err != nil {
			h.logger.Printf("Warning: Failed to refresh CORS cache: %v", err)
		} else {
			h.logger.Printf("CORS cache refreshed after updating application: %s", app.Name)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(app)
}

// DeleteApplication deletes an application (super admin only)
func (h *Handler) DeleteApplication(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "Invalid application ID", http.StatusBadRequest)
		return
	}

	query := "DELETE FROM applications WHERE id = $1"
	result, err := h.db.Exec(r.Context(), query, id)
	if err != nil {
		h.logger.Printf("Failed to delete application: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if result.RowsAffected() == 0 {
		http.Error(w, "Application not found", http.StatusNotFound)
		return
	}

	// Refresh CORS cache immediately
	if h.corsMiddleware != nil {
		if err := h.corsMiddleware.RefreshCache(); err != nil {
			h.logger.Printf("Warning: Failed to refresh CORS cache: %v", err)
		} else {
			h.logger.Printf("CORS cache refreshed after deleting application")
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

// ReloadCORS manually triggers a CORS cache reload (super admin only)
func (h *Handler) ReloadCORS(w http.ResponseWriter, r *http.Request) {
	if h.corsMiddleware == nil {
		http.Error(w, "CORS middleware not configured", http.StatusInternalServerError)
		return
	}

	if err := h.corsMiddleware.RefreshCache(); err != nil {
		h.logger.Printf("Failed to refresh CORS cache: %v", err)
		http.Error(w, "Failed to refresh CORS cache", http.StatusInternalServerError)
		return
	}

	h.logger.Printf("CORS cache manually refreshed")
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "CORS cache refreshed successfully",
	})
}

// Helper functions
func generateSlug(name string) string {
	// Convert to lowercase
	slug := strings.ToLower(name)
	// Replace spaces and special characters with hyphens
	reg := regexp.MustCompile("[^a-z0-9]+")
	slug = reg.ReplaceAllString(slug, "-")
	// Remove leading/trailing hyphens
	slug = strings.Trim(slug, "-")
	return slug
}

func isValidSlug(slug string) bool {
	// Slug must be lowercase alphanumeric with hyphens, no leading/trailing hyphens
	matched, _ := regexp.MatchString("^[a-z0-9]+(-[a-z0-9]+)*$", slug)
	return matched
}

// Backward compatibility - keep old function names for existing routes
func (h *Handler) ListOrigins(w http.ResponseWriter, r *http.Request) {
	h.ListApplications(w, r)
}

func (h *Handler) CreateOrigin(w http.ResponseWriter, r *http.Request) {
	h.CreateApplication(w, r)
}

func (h *Handler) UpdateOrigin(w http.ResponseWriter, r *http.Request) {
	h.UpdateApplication(w, r)
}

func (h *Handler) DeleteOrigin(w http.ResponseWriter, r *http.Request) {
	h.DeleteApplication(w, r)
}
