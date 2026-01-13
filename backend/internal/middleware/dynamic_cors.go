package middleware

import (
	"context"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/frans-sjostrom/auth-service/internal/database"
)

// DynamicCORS provides CORS middleware with dynamic origin loading and caching
type DynamicCORS struct {
	db              *database.DB
	cache           []string
	cacheMutex      sync.RWMutex
	lastRefresh     time.Time
	refreshInterval time.Duration
	logger          *log.Logger
	stopChan        chan struct{}
}

// NewDynamicCORS creates a new dynamic CORS middleware
func NewDynamicCORS(db *database.DB, logger *log.Logger) *DynamicCORS {
	cors := &DynamicCORS{
		db:              db,
		refreshInterval: 5 * time.Minute, // Auto-refresh every 5 minutes
		logger:          logger,
		stopChan:        make(chan struct{}),
	}

	// Initial load
	if err := cors.RefreshCache(); err != nil {
		logger.Printf("Warning: Failed to load initial CORS origins: %v", err)
	} else {
		logger.Printf("Loaded %d CORS origins", len(cors.cache))
	}

	// Start background refresh
	go cors.autoRefresh()

	return cors
}

// Middleware returns the HTTP middleware handler
func (c *DynamicCORS) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		if origin != "" {
			c.cacheMutex.RLock()
			allowed := c.isOriginAllowed(origin)
			c.cacheMutex.RUnlock()

			if allowed {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")

				// Handle preflight requests
				if r.Method == "OPTIONS" {
					w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
					w.Header().Set("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type, X-CSRF-Token")
					w.Header().Set("Access-Control-Expose-Headers", "Link")
					w.Header().Set("Access-Control-Max-Age", "300")
					w.WriteHeader(http.StatusOK)
					return
				}
			}
		}

		next.ServeHTTP(w, r)
	})
}

// RefreshCache reloads the allowed origins from the database
func (c *DynamicCORS) RefreshCache() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	origins, err := c.db.LoadActiveOrigins(ctx)
	if err != nil {
		return err
	}

	c.cacheMutex.Lock()
	c.cache = origins
	c.lastRefresh = time.Now()
	c.cacheMutex.Unlock()

	return nil
}

// autoRefresh runs a background goroutine that refreshes the cache periodically
func (c *DynamicCORS) autoRefresh() {
	ticker := time.NewTicker(c.refreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := c.RefreshCache(); err != nil {
				c.logger.Printf("Failed to refresh CORS cache: %v", err)
			} else {
				c.logger.Printf("CORS cache refreshed successfully (%d origins)", len(c.cache))
			}
		case <-c.stopChan:
			return
		}
	}
}

// isOriginAllowed checks if an origin is in the allowed list
func (c *DynamicCORS) isOriginAllowed(origin string) bool {
	for _, allowed := range c.cache {
		if strings.EqualFold(allowed, origin) {
			return true
		}
	}
	return false
}

// Stop stops the background refresh goroutine
func (c *DynamicCORS) Stop() {
	close(c.stopChan)
}

// GetCacheInfo returns information about the cache for debugging
func (c *DynamicCORS) GetCacheInfo() map[string]interface{} {
	c.cacheMutex.RLock()
	defer c.cacheMutex.RUnlock()

	return map[string]interface{}{
		"origin_count":  len(c.cache),
		"last_refresh":  c.lastRefresh,
		"next_refresh":  c.lastRefresh.Add(c.refreshInterval),
		"origins":       c.cache,
	}
}
