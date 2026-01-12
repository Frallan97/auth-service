package handlers

import (
	"log"

	"github.com/frans-sjostrom/auth-service/internal/auth"
	"github.com/frans-sjostrom/auth-service/internal/config"
	"github.com/frans-sjostrom/auth-service/internal/database"
)

type Handler struct {
	db          *database.DB
	cfg         *config.Config
	authService *auth.Service
	logger      *log.Logger
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
