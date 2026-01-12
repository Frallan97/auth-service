package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/frans-sjostrom/auth-service/internal/config"
	"github.com/frans-sjostrom/auth-service/internal/database"
	"github.com/frans-sjostrom/auth-service/internal/handlers"
	"github.com/frans-sjostrom/auth-service/internal/middleware"
	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Run migrations BEFORE connecting
	log.Println("Running database migrations...")
	if err := database.RunMigrations(cfg.DatabaseURL); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// Connect to database
	db, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	log.Println("Database connected successfully")

	// Load allowed origins from database
	ctx := context.Background()
	dbOrigins, err := db.LoadActiveOrigins(ctx)
	if err != nil {
		log.Printf("Warning: Failed to load origins from database: %v", err)
		log.Println("Using origins from environment variable")
	}

	// Use database origins if available, otherwise fall back to config
	allowedOrigins := cfg.AllowedOrigins
	if len(dbOrigins) > 0 {
		allowedOrigins = dbOrigins
		log.Printf("Loaded %d allowed origins from database", len(dbOrigins))
	} else {
		log.Printf("Using %d allowed origins from environment", len(allowedOrigins))
	}

	// Initialize handlers
	h := handlers.New(db, cfg)

	// Setup router
	r := chi.NewRouter()

	// Middleware
	r.Use(chiMiddleware.Logger)
	r.Use(chiMiddleware.Recoverer)
	r.Use(chiMiddleware.RequestID)
	r.Use(chiMiddleware.RealIP)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Public routes
	r.Route("/api", func(r chi.Router) {
		r.Get("/public-key", h.GetPublicKey)

		r.Route("/auth", func(r chi.Router) {
			// Apply rate limiting to auth endpoints (10 requests per minute)
			r.Use(middleware.RateLimitMiddleware(10))

			r.Get("/google/login", h.GoogleLogin)
			r.Get("/google/callback", h.GoogleCallback)

			// Stricter rate limiting for refresh endpoint (5 requests per minute)
			r.Group(func(r chi.Router) {
				r.Use(middleware.RateLimitMiddleware(5))
				r.Post("/refresh", h.RefreshToken)
			})

			r.Post("/logout", h.Logout)
		})

		// Protected routes
		r.Group(func(r chi.Router) {
			r.Use(middleware.AuthMiddleware(cfg.JWTPublicKey))

			r.Get("/auth/me", h.GetCurrentUser)

			r.Route("/users", func(r chi.Router) {
				// Mixed authorization - handlers check permissions
				r.Get("/{id}", h.GetUser)
				r.Put("/{id}", h.UpdateUser)

				// Admin-only routes
				r.Group(func(r chi.Router) {
					r.Use(middleware.AdminMiddleware())

					r.Get("/", h.ListUsers)
					r.Delete("/{id}", h.DeleteUser)
					r.Post("/{id}/activate", h.ActivateUser)
					r.Post("/{id}/deactivate", h.DeactivateUser)
				})
			})

			// Admin-only allowed origins management
			r.Route("/origins", func(r chi.Router) {
				r.Use(middleware.AdminMiddleware())

				r.Get("/", h.ListOrigins)
				r.Post("/", h.CreateOrigin)
				r.Put("/{id}", h.UpdateOrigin)
				r.Delete("/{id}", h.DeleteOrigin)
			})
		})
	})

	// Start server
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Port),
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	go func() {
		log.Printf("Starting server on port %s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}
