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
	"github.com/prometheus/client_golang/prometheus/promhttp"
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

	// Initialize dynamic CORS middleware
	logger := log.New(os.Stdout, "[CORS] ", log.LstdFlags)
	dynamicCORS := middleware.NewDynamicCORS(db, logger)
	defer dynamicCORS.Stop()

	// Initialize handlers with CORS middleware reference
	h := handlers.New(db, cfg)
	h.SetCORSMiddleware(dynamicCORS)

	// Setup router
	r := chi.NewRouter()

	// Middleware
	r.Use(chiMiddleware.Logger)
	r.Use(chiMiddleware.Recoverer)
	r.Use(chiMiddleware.RequestID)
	r.Use(chiMiddleware.RealIP)
	r.Use(middleware.MetricsMiddleware)
	r.Use(dynamicCORS.Middleware)

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Prometheus metrics endpoint
	r.Handle("/metrics", promhttp.Handler())

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

			// Admin-only allowed origins management (backward compatibility)
			r.Route("/origins", func(r chi.Router) {
				r.Use(middleware.AdminMiddleware())

				r.Get("/", h.ListOrigins)
				r.Post("/", h.CreateOrigin)
				r.Put("/{id}", h.UpdateOrigin)
				r.Delete("/{id}", h.DeleteOrigin)
			})

			// Admin-only applications management
			r.Route("/applications", func(r chi.Router) {
				r.Use(middleware.AdminMiddleware())

				r.Get("/", h.ListApplications)
				r.Post("/", h.CreateApplication)
				r.Get("/{id}", h.GetApplication)
				r.Put("/{id}", h.UpdateApplication)
				r.Delete("/{id}", h.DeleteApplication)
				r.Post("/reload-cors", h.ReloadCORS)
				r.Get("/{id}/logins", h.GetApplicationLoginHistory)
			})

			// Organization management
			r.Route("/organizations", func(r chi.Router) {
				// List and create require authentication
				r.Get("/", h.ListOrganizations)
				r.Post("/", h.CreateOrganization)

				// Individual organization operations
				r.Route("/{id}", func(r chi.Router) {
					r.Get("/", h.GetOrganization)
					r.Put("/", h.UpdateOrganization)
					r.Delete("/", h.DeleteOrganization)

					// Member management
					r.Get("/members", h.ListOrganizationMembers)
					r.Post("/members", h.AddOrganizationMember)
					r.Put("/members/{userId}", h.UpdateOrganizationMember)
					r.Delete("/members/{userId}", h.RemoveOrganizationMember)

					// Organization login history
					r.Get("/logins", h.GetOrganizationLoginHistory)
				})
			})

			// User endpoints
			r.Get("/users/{id}/organizations", h.GetUserOrganizations)
			r.Get("/users/{id}/logins", h.GetUserLoginHistory)

			// Login tracking
			r.Post("/track-login", h.TrackLogin)

			// Login statistics (admin only)
			r.Get("/stats/logins", h.GetLoginStats)
			r.Get("/stats/users", h.GetUserLoginStats)
			r.Get("/stats/applications", h.GetApplicationLoginStats)

			// Personal login statistics (authenticated users)
			r.Get("/me/stats", h.GetMyLoginStats)
			r.Get("/me/logins-by-app", h.GetMyLoginsByApp)
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
