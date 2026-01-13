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

// ListOrganizations returns all organizations (filtered by permissions)
func (h *Handler) ListOrganizations(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	isSuperAdmin := middleware.IsSuperAdmin(ctx)

	var query string
	var args []interface{}

	if isSuperAdmin {
		// Super admins see all organizations
		query = `
			SELECT id, name, slug, description, is_active, created_at, updated_at, created_by
			FROM organizations
			ORDER BY name
		`
	} else {
		// Regular users only see their organizations
		userID := ctx.Value(middleware.UserIDKey).(uuid.UUID)
		query = `
			SELECT DISTINCT o.id, o.name, o.slug, o.description, o.is_active, o.created_at, o.updated_at, o.created_by
			FROM organizations o
			JOIN user_organizations uo ON uo.organization_id = o.id
			WHERE uo.user_id = $1 AND o.is_active = true
			ORDER BY o.name
		`
		args = append(args, userID)
	}

	rows, err := h.db.Query(ctx, query, args...)
	if err != nil {
		h.logger.Printf("Failed to query organizations: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var organizations []models.Organization
	for rows.Next() {
		var org models.Organization
		if err := rows.Scan(&org.ID, &org.Name, &org.Slug, &org.Description, &org.IsActive, &org.CreatedAt, &org.UpdatedAt, &org.CreatedBy); err != nil {
			h.logger.Printf("Failed to scan organization: %v", err)
			continue
		}
		organizations = append(organizations, org)
	}

	if organizations == nil {
		organizations = []models.Organization{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(organizations)
}

// CreateOrganization creates a new organization (super admin only for now)
func (h *Handler) CreateOrganization(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req struct {
		Name        string  `json:"name"`
		Slug        string  `json:"slug"`
		Description *string `json:"description"`
		IsActive    *bool   `json:"is_active"`
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

	// Generate slug if not provided
	if req.Slug == "" {
		req.Slug = generateOrgSlug(req.Name)
	} else {
		if !isValidSlug(req.Slug) {
			http.Error(w, "Slug must be lowercase alphanumeric with hyphens", http.StatusBadRequest)
			return
		}
	}

	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	userID := ctx.Value(middleware.UserIDKey).(uuid.UUID)

	// Create organization
	query := `
		INSERT INTO organizations (name, slug, description, is_active, created_by)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, name, slug, description, is_active, created_at, updated_at, created_by
	`

	var org models.Organization
	err := h.db.QueryRow(ctx, query, req.Name, req.Slug, req.Description, isActive, userID).
		Scan(&org.ID, &org.Name, &org.Slug, &org.Description, &org.IsActive, &org.CreatedAt, &org.UpdatedAt, &org.CreatedBy)

	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			http.Error(w, "Organization slug already exists", http.StatusConflict)
			return
		}
		h.logger.Printf("Failed to create organization: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Automatically add creator as owner
	memberQuery := `
		INSERT INTO user_organizations (user_id, organization_id, role)
		VALUES ($1, $2, $3)
	`
	_, err = h.db.Exec(ctx, memberQuery, userID, org.ID, models.OrgRoleOwner)
	if err != nil {
		h.logger.Printf("Warning: Failed to add creator as owner: %v", err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(org)
}

// GetOrganization returns a single organization by ID
func (h *Handler) GetOrganization(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "Invalid organization ID", http.StatusBadRequest)
		return
	}

	// Check permissions
	if !middleware.IsSuperAdmin(ctx) && !middleware.HasOrgAccess(ctx, id) {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	query := `
		SELECT id, name, slug, description, is_active, created_at, updated_at, created_by
		FROM organizations
		WHERE id = $1
	`

	var org models.Organization
	err = h.db.QueryRow(ctx, query, id).
		Scan(&org.ID, &org.Name, &org.Slug, &org.Description, &org.IsActive, &org.CreatedAt, &org.UpdatedAt, &org.CreatedBy)

	if err == sql.ErrNoRows {
		http.Error(w, "Organization not found", http.StatusNotFound)
		return
	}
	if err != nil {
		h.logger.Printf("Failed to get organization: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(org)
}

// UpdateOrganization updates an existing organization (owner/admin only)
func (h *Handler) UpdateOrganization(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "Invalid organization ID", http.StatusBadRequest)
		return
	}

	// Check permissions - must be owner or admin
	if !middleware.IsSuperAdmin(ctx) && !middleware.IsOrgOwnerOrAdmin(ctx, id) {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	var req struct {
		Name        *string `json:"name"`
		Slug        *string `json:"slug"`
		Description *string `json:"description"`
		IsActive    *bool   `json:"is_active"`
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
	if req.Description != nil {
		updates = append(updates, "description = $"+string(rune(argCount+'0')))
		args = append(args, *req.Description)
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

	query := "UPDATE organizations SET " + strings.Join(updates, ", ") + " WHERE id = $" + string(rune(argCount+'0')) +
		" RETURNING id, name, slug, description, is_active, created_at, updated_at, created_by"

	var org models.Organization
	err = h.db.QueryRow(ctx, query, args...).
		Scan(&org.ID, &org.Name, &org.Slug, &org.Description, &org.IsActive, &org.CreatedAt, &org.UpdatedAt, &org.CreatedBy)

	if err == sql.ErrNoRows {
		http.Error(w, "Organization not found", http.StatusNotFound)
		return
	}
	if err != nil {
		h.logger.Printf("Failed to update organization: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(org)
}

// DeleteOrganization deletes an organization (super admin or owner only)
func (h *Handler) DeleteOrganization(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "Invalid organization ID", http.StatusBadRequest)
		return
	}

	// Check permissions - must be super admin or owner
	if !middleware.IsSuperAdmin(ctx) && !middleware.IsOrgOwner(ctx, id) {
		http.Error(w, "Access denied - only organization owners can delete", http.StatusForbidden)
		return
	}

	query := "DELETE FROM organizations WHERE id = $1"
	result, err := h.db.Exec(ctx, query, id)
	if err != nil {
		h.logger.Printf("Failed to delete organization: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if result.RowsAffected() == 0 {
		http.Error(w, "Organization not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Helper function
func generateOrgSlug(name string) string {
	slug := strings.ToLower(name)
	reg := regexp.MustCompile("[^a-z0-9]+")
	slug = reg.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	return slug
}
