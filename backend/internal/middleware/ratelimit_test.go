package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRateLimiter_Allow(t *testing.T) {
	limiter := newRateLimiter(3, 100*time.Millisecond)

	// Should allow first 3 requests
	assert.True(t, limiter.allow())
	assert.True(t, limiter.allow())
	assert.True(t, limiter.allow())

	// Should block 4th request
	assert.False(t, limiter.allow())

	// Wait for refill
	time.Sleep(150 * time.Millisecond)

	// Should allow request after refill
	assert.True(t, limiter.allow())
}

func TestRateLimiter_Refill(t *testing.T) {
	limiter := newRateLimiter(2, 50*time.Millisecond)

	// Consume all tokens
	assert.True(t, limiter.allow())
	assert.True(t, limiter.allow())
	assert.False(t, limiter.allow())

	// Wait for multiple refills
	time.Sleep(120 * time.Millisecond) // Should refill 2 tokens

	// Should have tokens again
	assert.True(t, limiter.allow())
	assert.True(t, limiter.allow())
	assert.False(t, limiter.allow())
}

func TestRateLimitMiddleware_AllowsRequests(t *testing.T) {
	// Create middleware with 5 requests per minute
	middleware := RateLimitMiddleware(5)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))

	// Test first request should succeed
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:1234"
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "OK", rr.Body.String())
}

func TestRateLimitMiddleware_BlocksExcessiveRequests(t *testing.T) {
	// Create middleware with 3 requests per minute
	middleware := RateLimitMiddleware(3)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))

	// Make 3 successful requests
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.1:1234"
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code, "Request %d should succeed", i+1)
	}

	// 4th request should be blocked
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:1234"
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
	assert.Contains(t, rr.Body.String(), "Too many requests")
}

func TestRateLimitMiddleware_DifferentIPs(t *testing.T) {
	// Create middleware with 2 requests per minute
	middleware := RateLimitMiddleware(2)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First IP makes 2 requests
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.1:1234"
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
	}

	// First IP's 3rd request should be blocked
	req1 := httptest.NewRequest("GET", "/test", nil)
	req1.RemoteAddr = "192.168.1.1:1234"
	rr1 := httptest.NewRecorder()
	handler.ServeHTTP(rr1, req1)
	assert.Equal(t, http.StatusTooManyRequests, rr1.Code)

	// Second IP should still be allowed
	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.RemoteAddr = "192.168.1.2:5678"
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)
	assert.Equal(t, http.StatusOK, rr2.Code)
}

func TestRateLimitMiddleware_XForwardedFor(t *testing.T) {
	middleware := RateLimitMiddleware(2)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Make 2 requests with X-Forwarded-For header
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-Forwarded-For", "10.0.0.1")
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
	}

	// 3rd request should be blocked based on X-Forwarded-For
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Forwarded-For", "10.0.0.1")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
}

func TestRateLimitStore_GetLimiter(t *testing.T) {
	store := NewRateLimitStore(10)

	// Get limiter for first IP
	limiter1 := store.getLimiter("192.168.1.1")
	assert.NotNil(t, limiter1)

	// Getting same IP should return same limiter
	limiter2 := store.getLimiter("192.168.1.1")
	assert.Equal(t, limiter1, limiter2, "Should return same limiter for same IP")

	// Different IP should get different limiter
	limiter3 := store.getLimiter("192.168.1.2")
	assert.NotEqual(t, limiter1, limiter3, "Different IPs should have different limiters")
}
