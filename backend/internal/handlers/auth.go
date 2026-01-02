package handlers

import (
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"strings"
	"time"

	"github.com/frans-sjostrom/auth-service/internal/middleware"
	"github.com/google/uuid"
)

func (h *Handler) GoogleLogin(w http.ResponseWriter, r *http.Request) {
	// Get redirect_uri parameter (where to send user after auth)
	redirectURI := r.URL.Query().Get("redirect_uri")
	if redirectURI == "" {
		// Default to first allowed origin if not specified
		redirectURI = h.cfg.AllowedOrigins[0]
	}

	// Validate redirect_uri is in allowed origins
	validRedirect := false
	for _, origin := range h.cfg.AllowedOrigins {
		if redirectURI == origin || strings.HasPrefix(redirectURI, origin+"/") {
			validRedirect = true
			break
		}
	}
	if !validRedirect {
		http.Error(w, "Invalid redirect_uri", http.StatusBadRequest)
		return
	}

	// Generate random state
	b := make([]byte, 16)
	rand.Read(b)
	state := base64.URLEncoding.EncodeToString(b)

	// Store state and redirect_uri in cookies for CSRF protection
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Expires:  time.Now().Add(10 * time.Minute),
		HttpOnly: true,
		Secure:   h.cfg.Env == "production",
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
	})

	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_redirect",
		Value:    redirectURI,
		Expires:  time.Now().Add(10 * time.Minute),
		HttpOnly: true,
		Secure:   h.cfg.Env == "production",
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
	})

	url := h.authService.GetGoogleAuthURL(state)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

func (h *Handler) GoogleCallback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Verify state
	stateCookie, err := r.Cookie("oauth_state")
	if err != nil {
		http.Error(w, "State cookie not found", http.StatusBadRequest)
		return
	}

	if r.URL.Query().Get("state") != stateCookie.Value {
		http.Error(w, "Invalid state parameter", http.StatusBadRequest)
		return
	}

	// Get redirect_uri from cookie
	redirectCookie, err := r.Cookie("oauth_redirect")
	if err != nil {
		http.Error(w, "Redirect URI cookie not found", http.StatusBadRequest)
		return
	}
	redirectURI := redirectCookie.Value

	// Clear state and redirect cookies
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    "",
		Expires:  time.Now().Add(-1 * time.Hour),
		HttpOnly: true,
		Path:     "/",
	})

	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_redirect",
		Value:    "",
		Expires:  time.Now().Add(-1 * time.Hour),
		HttpOnly: true,
		Path:     "/",
	})

	// Exchange code for user info
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "Code not found", http.StatusBadRequest)
		return
	}

	userInfo, err := h.authService.ExchangeGoogleCode(ctx, code)
	if err != nil {
		http.Error(w, "Failed to exchange code: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Create or update user
	user, err := h.authService.CreateOrUpdateUser(ctx, userInfo)
	if err != nil {
		http.Error(w, "Failed to create user: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Check if user is active
	if !user.IsActive {
		http.Error(w, "User account is inactive", http.StatusForbidden)
		return
	}

	// Generate tokens
	tokens, err := h.authService.GenerateTokens(ctx, user)
	if err != nil {
		http.Error(w, "Failed to generate tokens: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Set refresh token as HTTP-only cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    tokens.RefreshToken,
		Expires:  time.Now().Add(h.cfg.JWTRefreshTokenExpiry),
		HttpOnly: true,
		Secure:   h.cfg.Env == "production",
		SameSite: http.SameSiteStrictMode,
		Path:     "/",
	})

	// Log auth event
	h.authService.LogAuthEvent(ctx, &user.ID, "LOGIN", r.RemoteAddr, r.UserAgent())

	// Redirect to application callback with access token
	// Frontend should extract it and store in memory
	redirectURL := redirectURI + "/auth/callback?access_token=" + tokens.AccessToken
	http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
}

func (h *Handler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get refresh token from cookie
	cookie, err := r.Cookie("refresh_token")
	if err != nil {
		http.Error(w, "Refresh token not found", http.StatusUnauthorized)
		return
	}

	// Refresh tokens
	tokens, err := h.authService.RefreshAccessToken(ctx, cookie.Value)
	if err != nil {
		http.Error(w, "Failed to refresh token: "+err.Error(), http.StatusUnauthorized)
		return
	}

	// Set new refresh token as cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    tokens.RefreshToken,
		Expires:  time.Now().Add(h.cfg.JWTRefreshTokenExpiry),
		HttpOnly: true,
		Secure:   h.cfg.Env == "production",
		SameSite: http.SameSiteStrictMode,
		Path:     "/",
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"access_token": tokens.AccessToken,
	})
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get refresh token from cookie
	cookie, err := r.Cookie("refresh_token")
	if err == nil {
		// Revoke refresh token
		h.authService.RevokeRefreshToken(ctx, cookie.Value)
	}

	// Clear refresh token cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Expires:  time.Now().Add(-1 * time.Hour),
		HttpOnly: true,
		Secure:   h.cfg.Env == "production",
		SameSite: http.SameSiteStrictMode,
		Path:     "/",
	})

	// Log auth event if we have user context
	if userID, ok := r.Context().Value(middleware.UserIDKey).(uuid.UUID); ok {
		h.authService.LogAuthEvent(ctx, &userID, "LOGOUT", r.RemoteAddr, r.UserAgent())
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Logged out successfully",
	})
}

func (h *Handler) GetCurrentUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, ok := ctx.Value(middleware.UserIDKey).(uuid.UUID)
	if !ok {
		http.Error(w, "User ID not found in context", http.StatusUnauthorized)
		return
	}

	query := `
		SELECT id, email, google_id, name, avatar_url, role, is_active, created_at, updated_at
		FROM users
		WHERE id = $1 AND deleted_at IS NULL
	`

	var user struct {
		ID        uuid.UUID  `json:"id"`
		Email     string     `json:"email"`
		GoogleID  *string    `json:"google_id,omitempty"`
		Name      string     `json:"name"`
		AvatarURL *string    `json:"avatar_url,omitempty"`
		Role      string     `json:"role"`
		IsActive  bool       `json:"is_active"`
		CreatedAt time.Time  `json:"created_at"`
		UpdatedAt time.Time  `json:"updated_at"`
	}

	err := h.db.QueryRow(ctx, query, userID).Scan(
		&user.ID, &user.Email, &user.GoogleID, &user.Name, &user.AvatarURL,
		&user.Role, &user.IsActive, &user.CreatedAt, &user.UpdatedAt,
	)

	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

func (h *Handler) GetPublicKey(w http.ResponseWriter, r *http.Request) {
	publicKeyBytes := x509.MarshalPKCS1PublicKey(h.cfg.JWTPublicKey)
	publicKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PUBLIC KEY",
		Bytes: publicKeyBytes,
	})

	w.Header().Set("Content-Type", "text/plain")
	w.Write(publicKeyPEM)
}
