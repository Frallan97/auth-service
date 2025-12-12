package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type UserResponse struct {
	ID        uuid.UUID  `json:"id"`
	Email     string     `json:"email"`
	GoogleID  *string    `json:"google_id,omitempty"`
	Name      string     `json:"name"`
	AvatarURL *string    `json:"avatar_url,omitempty"`
	IsActive  bool       `json:"is_active"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}

type ListUsersResponse struct {
	Users      []UserResponse `json:"users"`
	Total      int            `json:"total"`
	Page       int            `json:"page"`
	PageSize   int            `json:"page_size"`
	TotalPages int            `json:"total_pages"`
}

func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse pagination parameters
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	offset := (page - 1) * pageSize

	// Get total count
	var total int
	countQuery := `SELECT COUNT(*) FROM users WHERE deleted_at IS NULL`
	err := h.db.QueryRow(ctx, countQuery).Scan(&total)
	if err != nil {
		http.Error(w, "Failed to count users", http.StatusInternalServerError)
		return
	}

	// Get users
	query := `
		SELECT id, email, google_id, name, avatar_url, is_active, created_at, updated_at, deleted_at
		FROM users
		WHERE deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := h.db.Query(ctx, query, pageSize, offset)
	if err != nil {
		http.Error(w, "Failed to query users", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	users := []UserResponse{}
	for rows.Next() {
		var user UserResponse
		err := rows.Scan(
			&user.ID, &user.Email, &user.GoogleID, &user.Name, &user.AvatarURL,
			&user.IsActive, &user.CreatedAt, &user.UpdatedAt, &user.DeletedAt,
		)
		if err != nil {
			continue
		}
		users = append(users, user)
	}

	totalPages := (total + pageSize - 1) / pageSize

	response := ListUsersResponse{
		Users:      users,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *Handler) GetUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := chi.URLParam(r, "id")

	userID, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	query := `
		SELECT id, email, google_id, name, avatar_url, is_active, created_at, updated_at, deleted_at
		FROM users
		WHERE id = $1 AND deleted_at IS NULL
	`

	var user UserResponse
	err = h.db.QueryRow(ctx, query, userID).Scan(
		&user.ID, &user.Email, &user.GoogleID, &user.Name, &user.AvatarURL,
		&user.IsActive, &user.CreatedAt, &user.UpdatedAt, &user.DeletedAt,
	)

	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

func (h *Handler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := chi.URLParam(r, "id")

	userID, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	var updateReq struct {
		Name      *string `json:"name"`
		AvatarURL *string `json:"avatar_url"`
	}

	if err := json.NewDecoder(r.Body).Decode(&updateReq); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	query := `
		UPDATE users
		SET name = COALESCE($1, name),
		    avatar_url = COALESCE($2, avatar_url),
		    updated_at = NOW()
		WHERE id = $3 AND deleted_at IS NULL
		RETURNING id, email, google_id, name, avatar_url, is_active, created_at, updated_at, deleted_at
	`

	var user UserResponse
	err = h.db.QueryRow(ctx, query, updateReq.Name, updateReq.AvatarURL, userID).Scan(
		&user.ID, &user.Email, &user.GoogleID, &user.Name, &user.AvatarURL,
		&user.IsActive, &user.CreatedAt, &user.UpdatedAt, &user.DeletedAt,
	)

	if err != nil {
		http.Error(w, "Failed to update user", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

func (h *Handler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := chi.URLParam(r, "id")

	userID, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	query := `
		UPDATE users
		SET deleted_at = NOW(), is_active = false
		WHERE id = $1 AND deleted_at IS NULL
	`

	result, err := h.db.Exec(ctx, query, userID)
	if err != nil {
		http.Error(w, "Failed to delete user", http.StatusInternalServerError)
		return
	}

	if result.RowsAffected() == 0 {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "User deleted successfully",
	})
}

func (h *Handler) ActivateUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := chi.URLParam(r, "id")

	userID, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	query := `
		UPDATE users
		SET is_active = true, updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING id, email, google_id, name, avatar_url, is_active, created_at, updated_at, deleted_at
	`

	var user UserResponse
	err = h.db.QueryRow(ctx, query, userID).Scan(
		&user.ID, &user.Email, &user.GoogleID, &user.Name, &user.AvatarURL,
		&user.IsActive, &user.CreatedAt, &user.UpdatedAt, &user.DeletedAt,
	)

	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

func (h *Handler) DeactivateUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := chi.URLParam(r, "id")

	userID, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	query := `
		UPDATE users
		SET is_active = false, updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING id, email, google_id, name, avatar_url, is_active, created_at, updated_at, deleted_at
	`

	var user UserResponse
	err = h.db.QueryRow(ctx, query, userID).Scan(
		&user.ID, &user.Email, &user.GoogleID, &user.Name, &user.AvatarURL,
		&user.IsActive, &user.CreatedAt, &user.UpdatedAt, &user.DeletedAt,
	)

	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}
