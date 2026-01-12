package middleware

import (
	"net/http"
	"sync"
	"time"
)

// rateLimiter implements a token bucket algorithm for rate limiting
type rateLimiter struct {
	tokens       int
	maxTokens    int
	refillRate   time.Duration
	lastRefill   time.Time
	mu           sync.Mutex
}

func newRateLimiter(maxTokens int, refillRate time.Duration) *rateLimiter {
	return &rateLimiter{
		tokens:     maxTokens,
		maxTokens:  maxTokens,
		refillRate: refillRate,
		lastRefill: time.Now(),
	}
}

// allow checks if a request is allowed and consumes a token if so
func (rl *rateLimiter) allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Refill tokens based on time elapsed
	now := time.Now()
	elapsed := now.Sub(rl.lastRefill)
	tokensToAdd := int(elapsed / rl.refillRate)

	if tokensToAdd > 0 {
		rl.tokens = min(rl.maxTokens, rl.tokens+tokensToAdd)
		rl.lastRefill = now
	}

	// Check if we have tokens available
	if rl.tokens > 0 {
		rl.tokens--
		return true
	}

	return false
}

// RateLimitStore manages rate limiters for different IP addresses
type RateLimitStore struct {
	limiters   sync.Map
	maxTokens  int
	refillRate time.Duration
	cleanupInterval time.Duration
}

// NewRateLimitStore creates a new rate limit store
func NewRateLimitStore(requestsPerMinute int) *RateLimitStore {
	store := &RateLimitStore{
		maxTokens:  requestsPerMinute,
		refillRate: time.Minute / time.Duration(requestsPerMinute),
		cleanupInterval: 5 * time.Minute,
	}

	// Start cleanup goroutine to remove old limiters
	go store.cleanup()

	return store
}

// cleanup periodically removes inactive rate limiters to prevent memory leaks
func (s *RateLimitStore) cleanup() {
	ticker := time.NewTicker(s.cleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()
		s.limiters.Range(func(key, value interface{}) bool {
			limiter := value.(*rateLimiter)
			limiter.mu.Lock()
			inactive := now.Sub(limiter.lastRefill) > s.cleanupInterval
			limiter.mu.Unlock()

			if inactive {
				s.limiters.Delete(key)
			}
			return true
		})
	}
}

// getLimiter retrieves or creates a rate limiter for an IP address
func (s *RateLimitStore) getLimiter(ip string) *rateLimiter {
	if limiter, ok := s.limiters.Load(ip); ok {
		return limiter.(*rateLimiter)
	}

	limiter := newRateLimiter(s.maxTokens, s.refillRate)
	actual, _ := s.limiters.LoadOrStore(ip, limiter)
	return actual.(*rateLimiter)
}

// RateLimitMiddleware creates a rate limiting middleware
// requestsPerMinute: maximum number of requests per minute per IP address
func RateLimitMiddleware(requestsPerMinute int) func(http.Handler) http.Handler {
	store := NewRateLimitStore(requestsPerMinute)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get client IP (handle X-Forwarded-For for proxies)
			ip := r.Header.Get("X-Forwarded-For")
			if ip == "" {
				ip = r.Header.Get("X-Real-IP")
			}
			if ip == "" {
				ip = r.RemoteAddr
			}

			limiter := store.getLimiter(ip)
			if !limiter.allow() {
				http.Error(w, "Too many requests. Please try again later.", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
