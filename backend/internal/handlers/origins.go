package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/frans-sjostrom/auth-service/internal/models"
	"github.com/go-chi/chi/v5"
)

// ListOrigins returns all allowed origins (admin only)
func (h *Handler) ListOrigins(w http.ResponseWriter, r *http.Request) {
	query := `
		SELECT id, origin, description, is_active, created_at, updated_at, created_by
		FROM allowed_origins
		ORDER BY created_at DESC
	`

	rows, err := h.db.Query(r.Context(), query)
	if err != nil {
		h.logger.Printf("Failed to query origins: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var origins []models.AllowedOrigin
	for rows.Next() {
		var origin models.AllowedOrigin
		if err := rows.Scan(&origin.ID, &origin.Origin, &origin.Description, &origin.IsActive, &origin.CreatedAt, &origin.UpdatedAt, &origin.CreatedBy); err != nil {
			h.logger.Printf("Failed to scan origin: %v", err)
			continue
		}
		origins = append(origins, origin)
	}

	if origins == nil {
		origins = []models.AllowedOrigin{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(origins)
}

// CreateOrigin creates a new allowed origin (admin only)
func (h *Handler) CreateOrigin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Origin      string  `json:"origin"`
		Description *string `json:"description"`
		IsActive    *bool   `json:"is_active"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Origin == "" {
		http.Error(w, "Origin is required", http.StatusBadRequest)
		return
	}

	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	// Get current user from context (set by auth middleware)
	userID := r.Context().Value("user_id")

	query := `
		INSERT INTO allowed_origins (origin, description, is_active, created_by)
		VALUES ($1, $2, $3, $4)
		RETURNING id, origin, description, is_active, created_at, updated_at, created_by
	`

	var origin models.AllowedOrigin
	err := h.db.QueryRow(r.Context(), query, req.Origin, req.Description, isActive, userID).
		Scan(&origin.ID, &origin.Origin, &origin.Description, &origin.IsActive, &origin.CreatedAt, &origin.UpdatedAt, &origin.CreatedBy)

	if err != nil {
		if err.Error() == "ERROR: duplicate key value violates unique constraint \"allowed_origins_origin_key\" (SQLSTATE 23505)" ||
			(err.Error() != "" && contains(err.Error(), "duplicate key")) {
			http.Error(w, "Origin already exists", http.StatusConflict)
			return
		}
		h.logger.Printf("Failed to create origin: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(origin)
}

// UpdateOrigin updates an existing allowed origin (admin only)
func (h *Handler) UpdateOrigin(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid origin ID", http.StatusBadRequest)
		return
	}

	var req struct {
		Origin      *string `json:"origin"`
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

	if req.Origin != nil {
		updates = append(updates, "origin = $"+strconv.Itoa(argCount))
		args = append(args, *req.Origin)
		argCount++
	}
	if req.Description != nil {
		updates = append(updates, "description = $"+strconv.Itoa(argCount))
		args = append(args, *req.Description)
		argCount++
	}
	if req.IsActive != nil {
		updates = append(updates, "is_active = $"+strconv.Itoa(argCount))
		args = append(args, *req.IsActive)
		argCount++
	}

	if len(updates) == 0 {
		http.Error(w, "No fields to update", http.StatusBadRequest)
		return
	}

	args = append(args, id)
	query := "UPDATE allowed_origins SET " + join(updates, ", ") + " WHERE id = $" + strconv.Itoa(argCount) +
		" RETURNING id, origin, description, is_active, created_at, updated_at, created_by"

	var origin models.AllowedOrigin
	err = h.db.QueryRow(r.Context(), query, args...).
		Scan(&origin.ID, &origin.Origin, &origin.Description, &origin.IsActive, &origin.CreatedAt, &origin.UpdatedAt, &origin.CreatedBy)

	if err == sql.ErrNoRows {
		http.Error(w, "Origin not found", http.StatusNotFound)
		return
	}
	if err != nil {
		h.logger.Printf("Failed to update origin: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(origin)
}

// DeleteOrigin deletes an allowed origin (admin only)
func (h *Handler) DeleteOrigin(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid origin ID", http.StatusBadRequest)
		return
	}

	query := "DELETE FROM allowed_origins WHERE id = $1"
	result, err := h.db.Exec(r.Context(), query, id)
	if err != nil {
		h.logger.Printf("Failed to delete origin: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if result.RowsAffected() == 0 {
		http.Error(w, "Origin not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Helper functions
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsInMiddle(s, substr)))
}

func containsInMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func join(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}
