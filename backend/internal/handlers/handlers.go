package handlers

import (
	"log"

	"github.com/frans-sjostrom/auth-service/internal/auth"
	"github.com/frans-sjostrom/auth-service/internal/config"
	"github.com/frans-sjostrom/auth-service/internal/database"
)

// CORSRefresher interface for dynamic CORS middleware
type CORSRefresher interface {
	RefreshCache() error
}

type Handler struct {
	db             *database.DB
	cfg            *config.Config
	authService    *auth.Service
	logger         *log.Logger
	corsMiddleware CORSRefresher
}

func New(db *database.DB, cfg *config.Config) *Handler {
	authService := auth.NewService(db, cfg)
	return &Handler{
		db:          db,
		cfg:         cfg,
		authService: authService,
		logger:      log.Default(),
	}
}

// SetCORSMiddleware sets the CORS middleware for handlers to trigger cache refresh
func (h *Handler) SetCORSMiddleware(cors CORSRefresher) {
	h.corsMiddleware = cors
}
