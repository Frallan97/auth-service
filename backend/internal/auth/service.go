package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/frans-sjostrom/auth-service/internal/config"
	"github.com/frans-sjostrom/auth-service/internal/database"
	"github.com/frans-sjostrom/auth-service/internal/models"
	customJWT "github.com/frans-sjostrom/auth-service/pkg/jwt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type Service struct {
	db           *database.DB
	cfg          *config.Config
	googleConfig *oauth2.Config
}

func NewService(db *database.DB, cfg *config.Config) *Service {
	googleConfig := &oauth2.Config{
		ClientID:     cfg.GoogleClientID,
		ClientSecret: cfg.GoogleClientSecret,
		RedirectURL:  cfg.GoogleRedirectURL,
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		},
		Endpoint: google.Endpoint,
	}

	return &Service{
		db:           db,
		cfg:          cfg,
		googleConfig: googleConfig,
	}
}

func (s *Service) GetGoogleAuthURL(state string) string {
	return s.googleConfig.AuthCodeURL(state, oauth2.AccessTypeOffline)
}

func (s *Service) ExchangeGoogleCode(ctx context.Context, code string) (*models.GoogleUserInfo, error) {
	token, err := s.googleConfig.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code: %w", err)
	}

	client := s.googleConfig.Client(ctx, token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get user info: %s", body)
	}

	var userInfo models.GoogleUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return nil, fmt.Errorf("failed to decode user info: %w", err)
	}

	return &userInfo, nil
}

func (s *Service) CreateOrUpdateUser(ctx context.Context, googleUserInfo *models.GoogleUserInfo) (*models.User, error) {
	var user models.User

	query := `
		SELECT id, email, google_id, name, avatar_url, role, is_active, created_at, updated_at, deleted_at
		FROM users
		WHERE google_id = $1 AND deleted_at IS NULL
	`

	err := s.db.QueryRow(ctx, query, googleUserInfo.ID).Scan(
		&user.ID, &user.Email, &user.GoogleID, &user.Name, &user.AvatarURL,
		&user.Role, &user.IsActive, &user.CreatedAt, &user.UpdatedAt, &user.DeletedAt,
	)

	if err == pgx.ErrNoRows {
		// Create new user - determine initial role
		initialRole := models.RoleUser

		// Check if email is in admin list
		for _, adminEmail := range s.cfg.AdminEmails {
			if googleUserInfo.Email == adminEmail {
				initialRole = models.RoleAdmin
				break
			}
		}

		insertQuery := `
			INSERT INTO users (email, google_id, name, avatar_url, role, is_active)
			VALUES ($1, $2, $3, $4, $5, true)
			RETURNING id, email, google_id, name, avatar_url, role, is_active, created_at, updated_at, deleted_at
		`
		err = s.db.QueryRow(ctx, insertQuery,
			googleUserInfo.Email, googleUserInfo.ID, googleUserInfo.Name, googleUserInfo.Picture, initialRole,
		).Scan(
			&user.ID, &user.Email, &user.GoogleID, &user.Name, &user.AvatarURL,
			&user.Role, &user.IsActive, &user.CreatedAt, &user.UpdatedAt, &user.DeletedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create user: %w", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("failed to query user: %w", err)
	} else {
		// Update existing user
		// Auto-upgrade to admin if in ADMIN_EMAILS config but role is still 'user'
		if user.Role == models.RoleUser {
			for _, adminEmail := range s.cfg.AdminEmails {
				if user.Email == adminEmail {
					user.Role = models.RoleAdmin
					break
				}
			}
		}

		updateQuery := `
			UPDATE users
			SET name = $1, avatar_url = $2, role = $3, updated_at = NOW()
			WHERE id = $4
			RETURNING id, email, google_id, name, avatar_url, role, is_active, created_at, updated_at, deleted_at
		`
		err = s.db.QueryRow(ctx, updateQuery, googleUserInfo.Name, googleUserInfo.Picture, user.Role, user.ID).Scan(
			&user.ID, &user.Email, &user.GoogleID, &user.Name, &user.AvatarURL,
			&user.Role, &user.IsActive, &user.CreatedAt, &user.UpdatedAt, &user.DeletedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to update user: %w", err)
		}
	}

	return &user, nil
}

func (s *Service) GenerateTokens(ctx context.Context, user *models.User) (*models.TokenPair, error) {
	// Generate access token
	accessToken, err := customJWT.GenerateAccessToken(
		user.ID,
		user.Email,
		user.Name,
		user.Role,
		s.cfg.JWTPrivateKey,
		s.cfg.JWTAccessTokenExpiry,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	// Generate refresh token
	refreshToken, err := customJWT.GenerateRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	// Hash and store refresh token
	tokenHash, err := customJWT.HashRefreshToken(refreshToken)
	if err != nil {
		return nil, fmt.Errorf("failed to hash refresh token: %w", err)
	}

	insertQuery := `
		INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
	`
	_, err = s.db.Exec(ctx, insertQuery, user.ID, tokenHash, time.Now().Add(s.cfg.JWTRefreshTokenExpiry))
	if err != nil {
		return nil, fmt.Errorf("failed to store refresh token: %w", err)
	}

	return &models.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

func (s *Service) RefreshAccessToken(ctx context.Context, refreshToken string) (*models.TokenPair, error) {
	// Find valid refresh token
	query := `
		SELECT id, user_id, token_hash, expires_at
		FROM refresh_tokens
		WHERE revoked_at IS NULL AND expires_at > NOW()
	`

	rows, err := s.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query refresh tokens: %w", err)
	}
	defer rows.Close()

	var tokenRecord *models.RefreshToken
	for rows.Next() {
		var rt models.RefreshToken
		if err := rows.Scan(&rt.ID, &rt.UserID, &rt.TokenHash, &rt.ExpiresAt); err != nil {
			continue
		}

		// Compare token hash
		if err := customJWT.CompareRefreshToken(rt.TokenHash, refreshToken); err == nil {
			tokenRecord = &rt
			break
		}
	}

	if tokenRecord == nil {
		return nil, fmt.Errorf("invalid or expired refresh token")
	}

	// Revoke old refresh token (rotating tokens)
	revokeQuery := `UPDATE refresh_tokens SET revoked_at = NOW() WHERE id = $1`
	_, err = s.db.Exec(ctx, revokeQuery, tokenRecord.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to revoke old token: %w", err)
	}

	// Get user
	var user models.User
	userQuery := `
		SELECT id, email, google_id, name, avatar_url, role, is_active, created_at, updated_at, deleted_at
		FROM users
		WHERE id = $1 AND is_active = true AND deleted_at IS NULL
	`
	err = s.db.QueryRow(ctx, userQuery, tokenRecord.UserID).Scan(
		&user.ID, &user.Email, &user.GoogleID, &user.Name, &user.AvatarURL,
		&user.Role, &user.IsActive, &user.CreatedAt, &user.UpdatedAt, &user.DeletedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("user not found or inactive: %w", err)
	}

	// Generate new token pair
	return s.GenerateTokens(ctx, &user)
}

func (s *Service) RevokeRefreshToken(ctx context.Context, refreshToken string) error {
	query := `
		SELECT id, token_hash
		FROM refresh_tokens
		WHERE revoked_at IS NULL AND expires_at > NOW()
	`

	rows, err := s.db.Query(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to query refresh tokens: %w", err)
	}
	defer rows.Close()

	var tokenID uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		var tokenHash string
		if err := rows.Scan(&id, &tokenHash); err != nil {
			continue
		}

		if err := customJWT.CompareRefreshToken(tokenHash, refreshToken); err == nil {
			tokenID = id
			break
		}
	}

	if tokenID == uuid.Nil {
		return fmt.Errorf("refresh token not found")
	}

	revokeQuery := `UPDATE refresh_tokens SET revoked_at = NOW() WHERE id = $1`
	_, err = s.db.Exec(ctx, revokeQuery, tokenID)
	return err
}

func (s *Service) LogAuthEvent(ctx context.Context, userID *uuid.UUID, action, ipAddress, userAgent string) {
	query := `
		INSERT INTO auth_audit_log (user_id, action, ip_address, user_agent)
		VALUES ($1, $2, $3, $4)
	`
	s.db.Exec(ctx, query, userID, action, ipAddress, userAgent)
}
