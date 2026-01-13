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

// MemberWithUser represents a member with user details
type MemberWithUser struct {
	ID             uuid.UUID `json:"id"`
	UserID         uuid.UUID `json:"user_id"`
	OrganizationID uuid.UUID `json:"organization_id"`
	Role           string    `json:"role"`
	Email          string    `json:"email"`
	Name           string    `json:"name"`
	AvatarURL      *string   `json:"avatar_url,omitempty"`
	CreatedAt      string    `json:"created_at"`
	UpdatedAt      string    `json:"updated_at"`
}

// ListOrganizationMembers lists all members of an organization
func (h *Handler) ListOrganizationMembers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgIDStr := chi.URLParam(r, "id")
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		http.Error(w, "Invalid organization ID", http.StatusBadRequest)
		return
	}

	// Check permissions - must have access to the organization
	if !middleware.IsSuperAdmin(ctx) && !middleware.HasOrgAccess(ctx, orgID) {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	query := `
		SELECT uo.id, uo.user_id, uo.organization_id, uo.role,
		       u.email, u.name, u.avatar_url, uo.created_at, uo.updated_at
		FROM user_organizations uo
		JOIN users u ON u.id = uo.user_id
		WHERE uo.organization_id = $1 AND u.deleted_at IS NULL
		ORDER BY
			CASE uo.role
				WHEN 'owner' THEN 1
				WHEN 'admin' THEN 2
				WHEN 'member' THEN 3
				WHEN 'viewer' THEN 4
			END,
			u.name
	`

	rows, err := h.db.Query(ctx, query, orgID)
	if err != nil {
		h.logger.Printf("Failed to query organization members: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var members []MemberWithUser
	for rows.Next() {
		var member MemberWithUser
		if err := rows.Scan(&member.ID, &member.UserID, &member.OrganizationID, &member.Role,
			&member.Email, &member.Name, &member.AvatarURL, &member.CreatedAt, &member.UpdatedAt); err != nil {
			h.logger.Printf("Failed to scan member: %v", err)
			continue
		}
		members = append(members, member)
	}

	if members == nil {
		members = []MemberWithUser{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(members)
}

// AddOrganizationMember adds a user to an organization
func (h *Handler) AddOrganizationMember(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgIDStr := chi.URLParam(r, "id")
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		http.Error(w, "Invalid organization ID", http.StatusBadRequest)
		return
	}

	// Check permissions - must be owner or admin
	if !middleware.IsSuperAdmin(ctx) && !middleware.IsOrgOwnerOrAdmin(ctx, orgID) {
		http.Error(w, "Access denied - only organization owners and admins can add members", http.StatusForbidden)
		return
	}

	var req struct {
		UserID uuid.UUID `json:"user_id"`
		Email  string    `json:"email"` // Alternative to user_id
		Role   string    `json:"role"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate role
	if req.Role == "" {
		req.Role = models.OrgRoleMember
	}
	if req.Role != models.OrgRoleOwner && req.Role != models.OrgRoleAdmin &&
		req.Role != models.OrgRoleMember && req.Role != models.OrgRoleViewer {
		http.Error(w, "Invalid role. Must be: owner, admin, member, or viewer", http.StatusBadRequest)
		return
	}

	// If email provided, look up user ID
	if req.UserID == uuid.Nil && req.Email != "" {
		userQuery := `SELECT id FROM users WHERE email = $1 AND deleted_at IS NULL`
		err := h.db.QueryRow(ctx, userQuery, req.Email).Scan(&req.UserID)
		if err == sql.ErrNoRows {
			http.Error(w, "User not found with that email", http.StatusNotFound)
			return
		}
		if err != nil {
			h.logger.Printf("Failed to look up user by email: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	}

	if req.UserID == uuid.Nil {
		http.Error(w, "Either user_id or email must be provided", http.StatusBadRequest)
		return
	}

	// Only owners can add other owners
	if req.Role == models.OrgRoleOwner && !middleware.IsSuperAdmin(ctx) && !middleware.IsOrgOwner(ctx, orgID) {
		http.Error(w, "Only organization owners can add other owners", http.StatusForbidden)
		return
	}

	// Check if user is already a member
	checkQuery := `SELECT id FROM user_organizations WHERE user_id = $1 AND organization_id = $2`
	var existingID uuid.UUID
	err = h.db.QueryRow(ctx, checkQuery, req.UserID, orgID).Scan(&existingID)
	if err == nil {
		http.Error(w, "User is already a member of this organization", http.StatusConflict)
		return
	}

	// Add member
	query := `
		INSERT INTO user_organizations (user_id, organization_id, role)
		VALUES ($1, $2, $3)
		RETURNING id, user_id, organization_id, role, created_at, updated_at
	`

	var membership models.UserOrganization
	err = h.db.QueryRow(ctx, query, req.UserID, orgID, req.Role).
		Scan(&membership.ID, &membership.UserID, &membership.OrganizationID, &membership.Role, &membership.CreatedAt, &membership.UpdatedAt)

	if err != nil {
		h.logger.Printf("Failed to add member: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(membership)
}

// UpdateOrganizationMember updates a member's role
func (h *Handler) UpdateOrganizationMember(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgIDStr := chi.URLParam(r, "id")
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		http.Error(w, "Invalid organization ID", http.StatusBadRequest)
		return
	}

	userIDStr := chi.URLParam(r, "userId")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	// Check permissions - must be owner or admin
	if !middleware.IsSuperAdmin(ctx) && !middleware.IsOrgOwnerOrAdmin(ctx, orgID) {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	var req struct {
		Role string `json:"role"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate role
	if req.Role != models.OrgRoleOwner && req.Role != models.OrgRoleAdmin &&
		req.Role != models.OrgRoleMember && req.Role != models.OrgRoleViewer {
		http.Error(w, "Invalid role. Must be: owner, admin, member, or viewer", http.StatusBadRequest)
		return
	}

	// Get current role to check permissions
	var currentRole string
	checkQuery := `SELECT role FROM user_organizations WHERE user_id = $1 AND organization_id = $2`
	err = h.db.QueryRow(ctx, checkQuery, userID, orgID).Scan(&currentRole)
	if err == sql.ErrNoRows {
		http.Error(w, "User is not a member of this organization", http.StatusNotFound)
		return
	}
	if err != nil {
		h.logger.Printf("Failed to check current role: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Only owners can change owner roles
	if (currentRole == models.OrgRoleOwner || req.Role == models.OrgRoleOwner) &&
		!middleware.IsSuperAdmin(ctx) && !middleware.IsOrgOwner(ctx, orgID) {
		http.Error(w, "Only organization owners can change owner roles", http.StatusForbidden)
		return
	}

	// Prevent removing the last owner
	if currentRole == models.OrgRoleOwner && req.Role != models.OrgRoleOwner {
		countQuery := `SELECT COUNT(*) FROM user_organizations WHERE organization_id = $1 AND role = $2`
		var ownerCount int
		err = h.db.QueryRow(ctx, countQuery, orgID, models.OrgRoleOwner).Scan(&ownerCount)
		if err == nil && ownerCount <= 1 {
			http.Error(w, "Cannot remove the last owner from the organization", http.StatusBadRequest)
			return
		}
	}

	// Update role
	query := `
		UPDATE user_organizations
		SET role = $1, updated_at = NOW()
		WHERE user_id = $2 AND organization_id = $3
		RETURNING id, user_id, organization_id, role, created_at, updated_at
	`

	var membership models.UserOrganization
	err = h.db.QueryRow(ctx, query, req.Role, userID, orgID).
		Scan(&membership.ID, &membership.UserID, &membership.OrganizationID, &membership.Role, &membership.CreatedAt, &membership.UpdatedAt)

	if err != nil {
		h.logger.Printf("Failed to update member role: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(membership)
}

// RemoveOrganizationMember removes a user from an organization
func (h *Handler) RemoveOrganizationMember(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgIDStr := chi.URLParam(r, "id")
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		http.Error(w, "Invalid organization ID", http.StatusBadRequest)
		return
	}

	userIDStr := chi.URLParam(r, "userId")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	// Check permissions - must be owner or admin
	if !middleware.IsSuperAdmin(ctx) && !middleware.IsOrgOwnerOrAdmin(ctx, orgID) {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	// Get member's role
	var memberRole string
	checkQuery := `SELECT role FROM user_organizations WHERE user_id = $1 AND organization_id = $2`
	err = h.db.QueryRow(ctx, checkQuery, userID, orgID).Scan(&memberRole)
	if err == sql.ErrNoRows {
		http.Error(w, "User is not a member of this organization", http.StatusNotFound)
		return
	}
	if err != nil {
		h.logger.Printf("Failed to check member role: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Only owners can remove other owners
	if memberRole == models.OrgRoleOwner && !middleware.IsSuperAdmin(ctx) && !middleware.IsOrgOwner(ctx, orgID) {
		http.Error(w, "Only organization owners can remove other owners", http.StatusForbidden)
		return
	}

	// Prevent removing the last owner
	if memberRole == models.OrgRoleOwner {
		countQuery := `SELECT COUNT(*) FROM user_organizations WHERE organization_id = $1 AND role = $2`
		var ownerCount int
		err = h.db.QueryRow(ctx, countQuery, orgID, models.OrgRoleOwner).Scan(&ownerCount)
		if err == nil && ownerCount <= 1 {
			http.Error(w, "Cannot remove the last owner from the organization", http.StatusBadRequest)
			return
		}
	}

	// Remove member
	query := `DELETE FROM user_organizations WHERE user_id = $1 AND organization_id = $2`
	result, err := h.db.Exec(ctx, query, userID, orgID)
	if err != nil {
		h.logger.Printf("Failed to remove member: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if result.RowsAffected() == 0 {
		http.Error(w, "Member not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetUserOrganizations returns all organizations a user belongs to
func (h *Handler) GetUserOrganizations(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userIDStr := chi.URLParam(r, "id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	// Check permissions - users can see their own orgs, admins can see any
	currentUserID := ctx.Value(middleware.UserIDKey).(uuid.UUID)
	if !middleware.IsSuperAdmin(ctx) && currentUserID != userID {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	query := `
		SELECT o.id, o.name, o.slug, o.description, o.is_active, o.created_at, o.updated_at, o.created_by, uo.role
		FROM organizations o
		JOIN user_organizations uo ON uo.organization_id = o.id
		WHERE uo.user_id = $1 AND o.is_active = true
		ORDER BY o.name
	`

	rows, err := h.db.Query(ctx, query, userID)
	if err != nil {
		h.logger.Printf("Failed to query user organizations: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type OrgWithRole struct {
		models.Organization
		Role string `json:"role"`
	}

	var organizations []OrgWithRole
	for rows.Next() {
		var org OrgWithRole
		if err := rows.Scan(&org.ID, &org.Name, &org.Slug, &org.Description, &org.IsActive,
			&org.CreatedAt, &org.UpdatedAt, &org.CreatedBy, &org.Role); err != nil {
			h.logger.Printf("Failed to scan organization: %v", err)
			continue
		}
		organizations = append(organizations, org)
	}

	if organizations == nil {
		organizations = []OrgWithRole{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(organizations)
}
