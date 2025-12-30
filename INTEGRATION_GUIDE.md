# Auth Service Integration Guide

This guide explains how to integrate any new application with the central auth-service for Google OAuth authentication.

## Table of Contents

1. [Overview](#overview)
2. [Backend Integration](#backend-integration)
3. [Frontend Integration](#frontend-integration)
4. [Testing](#testing)
5. [Deployment](#deployment)
6. [Troubleshooting](#troubleshooting)

---

## Overview

The auth-service provides centralized authentication using Google OAuth 2.0. Applications integrate with auth-service to:

1. **Authenticate users** via Google OAuth
2. **Validate JWT tokens** to protect routes
3. **Access user information** from JWT claims

### Architecture

```
User Browser → Your Frontend → Your Backend → Protected Resources
                    ↓              ↓
              auth-service    JWT Validation
              (OAuth Flow)    (Public Key)
```

**Key Concepts:**
- **JWT Tokens**: Signed with RS256 using auth-service's private key
- **Public Key**: Your backend fetches auth-service's public key to validate tokens
- **Claims**: Tokens include `sub` (user_id), `email`, and `name`

---

## Backend Integration

### Step 1: Create JWT Validation Package

Create a package to fetch the public key and validate JWT tokens.

**File:** `backend/pkg/auth/jwt.go` (Go example)

```go
package auth

import (
	"crypto/rsa"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const AuthServicePublicKeyURL = "https://auth.vibeoholic.com/api/auth/public-key"

// Claims represents the JWT token claims from auth-service
type Claims struct {
	UserID string `json:"sub"`
	Email  string `json:"email"`
	Name   string `json:"name"`
	jwt.RegisteredClaims
}

// PublicKeyCache stores the public key and when it was fetched
type PublicKeyCache struct {
	Key       *rsa.PublicKey
	FetchedAt time.Time
}

var keyCache *PublicKeyCache

// FetchPublicKey fetches the public key from auth-service
// Uses a 1-hour cache to avoid excessive requests
func FetchPublicKey() (*rsa.PublicKey, error) {
	// Check cache (refresh every hour)
	if keyCache != nil && time.Since(keyCache.FetchedAt) < 1*time.Hour {
		return keyCache.Key, nil
	}

	resp, err := http.Get(AuthServicePublicKeyURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch public key: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch public key: status %d", resp.StatusCode)
	}

	keyData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read public key: %w", err)
	}

	publicKey, err := jwt.ParseRSAPublicKeyFromPEM(keyData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}

	// Cache the key
	keyCache = &PublicKeyCache{
		Key:       publicKey,
		FetchedAt: time.Now(),
	}

	return publicKey, nil
}

// ValidateToken validates a JWT token using the auth-service public key
func ValidateToken(tokenString string, publicKey *rsa.PublicKey) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Verify signing method
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return publicKey, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token")
}
```

### Step 2: Create Authentication Middleware

Create middleware to protect routes and extract user information from JWT tokens.

**File:** `backend/internal/middleware/auth.go` (Go/Gin example)

```go
package middleware

import (
	"crypto/rsa"
	"net/http"
	"strings"

	"your-project/pkg/auth"
	"github.com/gin-gonic/gin"
)

// AuthMiddleware validates JWT tokens from auth-service
func AuthMiddleware(publicKey *rsa.PublicKey) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract token from Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing authorization header"})
			c.Abort()
			return
		}

		// Check Bearer prefix
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization header format"})
			c.Abort()
			return
		}

		tokenString := parts[1]

		// Validate token
		claims, err := auth.ValidateToken(tokenString, publicKey)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token", "details": err.Error()})
			c.Abort()
			return
		}

		// Store claims in context for use in handlers
		c.Set("user_id", claims.UserID)
		c.Set("user_email", claims.Email)
		c.Set("user_name", claims.Name)

		c.Next()
	}
}

// OptionalAuthMiddleware allows both authenticated and unauthenticated requests
// but sets user context if a valid token is provided
func OptionalAuthMiddleware(publicKey *rsa.PublicKey) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Next()
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.Next()
			return
		}

		tokenString := parts[1]
		claims, err := auth.ValidateToken(tokenString, publicKey)
		if err == nil {
			c.Set("user_id", claims.UserID)
			c.Set("user_email", claims.Email)
			c.Set("user_name", claims.Name)
		}

		c.Next()
	}
}
```

### Step 3: Update main.go

Fetch the public key on startup and apply middleware to protected routes.

```go
func main() {
	// ... existing initialization code ...

	// Fetch public key from auth-service for JWT validation
	log.Println("Fetching public key from auth-service...")
	publicKey, err := auth.FetchPublicKey()
	if err != nil {
		log.Fatal("Failed to fetch public key from auth-service:", err)
	}
	log.Println("Public key fetched successfully")

	// Setup router
	r := gin.New()
	r.Use(gin.Recovery())

	// CORS middleware - MUST include Authorization header
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		c.Writer.Header().Set("Cache-Control", "no-cache")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// Public endpoints (no auth required)
	r.GET("/health", healthHandler)

	// OAuth callback endpoint
	users := r.Group("/api/users")
	{
		users.GET("/auth/callback", func(c *gin.Context) {
			token := c.Query("token")
			if token == "" {
				c.JSON(400, gin.H{"error": "token query parameter required"})
				return
			}

			// Validate token
			claims, err := auth.ValidateToken(token, publicKey)
			if err != nil {
				c.JSON(401, gin.H{"error": "invalid token", "details": err.Error()})
				return
			}

			// Create or get user based on claims.Email or claims.UserID
			user, err := userService.LoginOrRegister(claims.Email)
			if err != nil {
				c.JSON(500, gin.H{"error": err.Error()})
				return
			}

			// Return user info and token for frontend to store
			c.JSON(200, gin.H{
				"user":  user,
				"token": token,
			})
		})
	}

	// Protected endpoints (auth required)
	protected := r.Group("/api")
	protected.Use(middleware.AuthMiddleware(publicKey))
	{
		protected.POST("/sessions/start", startSessionHandler)
		protected.GET("/sessions/active", getActiveSessionHandler)
		protected.POST("/orders", createOrderHandler)
		// ... other protected routes ...
	}

	r.Run(":8080")
}
```

### Step 4: Add JWT Dependency

Add the JWT library to your `go.mod`:

```bash
go get github.com/golang-jwt/jwt/v5
go mod tidy
```

---

## Frontend Integration

### Step 1: Create Auth Service

Create a service to manage authentication state and OAuth flow.

**File:** `frontend/src/lib/auth.ts`

```typescript
/**
 * Authentication service for managing JWT tokens and OAuth flow with auth-service
 */

const AUTH_SERVICE_URL = "https://auth.vibeoholic.com";
const TOKEN_KEY = "auth_token";
const USER_KEY = "auth_user";

export interface AuthUser {
  id: string;
  email: string;
  name: string;
}

export class AuthService {
  /**
   * Redirect to auth-service for Google OAuth login
   * After successful login, user will be redirected back to /auth/callback with token
   */
  login(): void {
    const callbackUrl = `${window.location.origin}/auth/callback`;
    const authUrl = `${AUTH_SERVICE_URL}/api/auth/google/login?redirect_url=${encodeURIComponent(
      callbackUrl
    )}`;
    window.location.href = authUrl;
  }

  /**
   * Handle OAuth callback from auth-service
   * Validates token and stores user info
   * @param token - JWT token from auth-service
   * @returns User info from backend
   */
  async handleCallback(token: string): Promise<AuthUser> {
    // Store token immediately
    this.setToken(token);

    // Validate token and get user info from backend
    const response = await fetch(
      `/api/users/auth/callback?token=${encodeURIComponent(token)}`,
      {
        method: "GET",
        headers: {
          "Content-Type": "application/json",
        },
      }
    );

    if (!response.ok) {
      this.clearAuth();
      const error = await response.json();
      throw new Error(error.error || "Failed to authenticate");
    }

    const data = await response.json();

    // Store user info
    const user: AuthUser = {
      id: data.user.id,
      email: data.user.email,
      name: data.user.username,
    };

    this.setUser(user);

    return user;
  }

  /**
   * Store JWT token in localStorage
   */
  setToken(token: string): void {
    localStorage.setItem(TOKEN_KEY, token);
  }

  /**
   * Get JWT token from localStorage
   */
  getToken(): string | null {
    return localStorage.getItem(TOKEN_KEY);
  }

  /**
   * Store user info in localStorage
   */
  setUser(user: AuthUser): void {
    localStorage.setItem(USER_KEY, JSON.stringify(user));
  }

  /**
   * Get user info from localStorage
   */
  getUser(): AuthUser | null {
    const userJson = localStorage.getItem(USER_KEY);
    if (!userJson) {
      return null;
    }

    try {
      return JSON.parse(userJson);
    } catch {
      return null;
    }
  }

  /**
   * Check if user is authenticated (has valid token)
   */
  isAuthenticated(): boolean {
    return this.getToken() !== null;
  }

  /**
   * Get Authorization header for API requests
   */
  getAuthHeader(): Record<string, string> {
    const token = this.getToken();
    if (!token) {
      return {};
    }

    return {
      Authorization: `Bearer ${token}`,
    };
  }

  /**
   * Logout user (clear token and user info)
   */
  logout(): void {
    this.clearAuth();
  }

  /**
   * Clear all auth data from localStorage
   */
  private clearAuth(): void {
    localStorage.removeItem(TOKEN_KEY);
    localStorage.removeItem(USER_KEY);
  }
}

// Export singleton instance
export const authService = new AuthService();
```

### Step 2: Create Login Component

Create a login page that redirects users to auth-service.

**File:** `frontend/src/components/Login.tsx`

```typescript
import { authService } from "@/lib/auth";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";

interface LoginProps {
  onLoginSuccess: (userId: string, username: string) => void;
}

export function Login({ onLoginSuccess }: LoginProps) {
  const handleGoogleLogin = () => {
    authService.login();
  };

  return (
    <div className="min-h-screen flex items-center justify-center bg-gradient-to-br from-gray-50 to-gray-100 p-4">
      <Card className="w-full max-w-md">
        <CardHeader className="space-y-1">
          <CardTitle className="text-2xl font-bold">Welcome to Your App</CardTitle>
          <CardDescription>
            Sign in with your Google account to continue
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="space-y-4">
            <Button
              onClick={handleGoogleLogin}
              className="w-full flex items-center justify-center gap-2"
            >
              <svg className="w-5 h-5" viewBox="0 0 24 24">
                {/* Google logo SVG paths */}
              </svg>
              Sign in with Google
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
```

### Step 3: Create AuthCallback Component

Handle the OAuth callback and token validation.

**File:** `frontend/src/components/AuthCallback.tsx`

```typescript
import { useEffect, useState } from "react";
import { authService } from "@/lib/auth";

interface AuthCallbackProps {
  onSuccess: (userId: string, username: string) => void;
}

export function AuthCallback({ onSuccess }: AuthCallbackProps) {
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const handleCallback = async () => {
      try {
        // Get token from URL query parameter
        const urlParams = new URLSearchParams(window.location.search);
        const token = urlParams.get("token");

        if (!token) {
          setError("No authentication token received");
          setLoading(false);
          return;
        }

        // Validate token and get user info from backend
        const user = await authService.handleCallback(token);

        // Clear URL params
        window.history.replaceState({}, document.title, "/");

        // Notify parent component of successful login
        onSuccess(user.id, user.name);
      } catch (err) {
        setError(err instanceof Error ? err.message : "Authentication failed");
        setLoading(false);
      }
    };

    handleCallback();
  }, [onSuccess]);

  return (
    <div className="min-h-screen flex items-center justify-center">
      {loading ? (
        <div>Authenticating...</div>
      ) : (
        <div>Error: {error}</div>
      )}
    </div>
  );
}
```

### Step 4: Update API Services

Add authentication headers to all protected API requests.

**Example:** `frontend/src/services/orders.ts`

```typescript
import { authService } from "@/lib/auth";

export class OrderService {
  async createOrder(sessionId: string, request: CreateOrderRequest): Promise<Order> {
    const response = await fetch(`/api/orders?session_id=${sessionId}`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        ...authService.getAuthHeader(), // Add auth header
      },
      body: JSON.stringify(request),
    });

    if (!response.ok) {
      const error = await response.json();
      throw new Error(error.error || "Failed to create order");
    }

    return response.json();
  }

  // Apply to all protected endpoints
  async getOrders(sessionId: string): Promise<Order[]> {
    const response = await fetch(`/api/orders?session_id=${sessionId}`, {
      headers: {
        ...authService.getAuthHeader(), // Add auth header
      },
    });

    // ... rest of the method
  }
}
```

### Step 5: Update App Router

Handle routing for login and callback pages.

```typescript
export function App() {
  const [userId, setUserId] = useState<string | null>(null);
  const [username, setUsername] = useState<string | null>(null);

  // Check if user is authenticated on mount
  useEffect(() => {
    if (authService.isAuthenticated()) {
      const user = authService.getUser();
      if (user) {
        setUserId(user.id);
        setUsername(user.name);
      }
    }
  }, []);

  const handleLoginSuccess = (newUserId: string, newUsername: string) => {
    setUserId(newUserId);
    setUsername(newUsername);
  };

  const handleLogout = () => {
    authService.logout();
    setUserId(null);
    setUsername(null);
  };

  // Handle OAuth callback route
  if (window.location.pathname === "/auth/callback") {
    return <AuthCallback onSuccess={handleLoginSuccess} />;
  }

  // Show login screen if not logged in
  if (!userId) {
    return <Login onLoginSuccess={handleLoginSuccess} />;
  }

  // Main app UI
  return (
    <div>
      <button onClick={handleLogout}>Logout</button>
      {/* Rest of your app */}
    </div>
  );
}
```

---

## Testing

### Backend Testing

1. **Test Public Key Fetch:**
   ```bash
   curl https://auth.vibeoholic.com/api/auth/public-key
   ```

2. **Test JWT Validation:**
   - Get a valid token from auth-service by logging in through frontend
   - Test protected endpoint with token:
     ```bash
     curl -H "Authorization: Bearer YOUR_TOKEN" http://localhost:8080/api/protected-endpoint
     ```

3. **Test Without Token:**
   ```bash
   curl http://localhost:8080/api/protected-endpoint
   # Should return 401 Unauthorized
   ```

### Frontend Testing

1. **Test Login Flow:**
   - Visit your app's frontend
   - Click "Sign in with Google"
   - Should redirect to auth.vibeoholic.com
   - After Google login, should redirect back to your app
   - Should show authenticated UI

2. **Test Token Storage:**
   - Open browser DevTools → Application → Local Storage
   - Verify `auth_token` and `auth_user` are stored

3. **Test Logout:**
   - Click logout button
   - Verify token is cleared from localStorage
   - Should show login screen again

---

## Deployment

### Update Google OAuth Settings

Add your production URL to auth-service's Google OAuth configuration:

```bash
# Contact admin to add your redirect URI:
https://your-app.vibeoholic.com/auth/callback
```

### Environment Variables

**Backend:**
```env
# No auth-service specific env vars needed
# Public key is fetched from https://auth.vibeoholic.com/api/auth/public-key
```

**Frontend:**
```env
VITE_API_URL=https://your-app.vibeoholic.com
```

### CORS Configuration

Ensure your backend allows requests from your frontend domain with Authorization header:

```go
c.Writer.Header().Set("Access-Control-Allow-Origin", "https://your-app.vibeoholic.com")
c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
```

---

## Troubleshooting

### Common Issues

#### 1. "Invalid token" errors

**Cause:** Token signature validation fails

**Solutions:**
- Verify public key is fetched correctly: `curl https://auth.vibeoholic.com/api/auth/public-key`
- Check token is being sent in Authorization header: `Bearer YOUR_TOKEN`
- Ensure token hasn't expired (tokens expire after 24 hours)

#### 2. "Missing authorization header"

**Cause:** Frontend not sending Authorization header

**Solutions:**
- Verify `authService.getAuthHeader()` is called in API requests
- Check CORS allows Authorization header
- Verify token is stored in localStorage

#### 3. CORS errors

**Cause:** Backend not allowing frontend origin or Authorization header

**Solutions:**
- Add `Authorization` to `Access-Control-Allow-Headers`
- Add your frontend URL to `Access-Control-Allow-Origin`
- Handle OPTIONS preflight requests

#### 4. Redirect loop on /auth/callback

**Cause:** Callback handler not clearing URL params

**Solutions:**
- Use `window.history.replaceState({}, document.title, "/")` after handling token
- Ensure callback component only runs once (use `useEffect` with empty deps)

#### 5. "Failed to fetch public key"

**Cause:** Network issue or auth-service is down

**Solutions:**
- Verify auth-service is running: `curl https://auth.vibeoholic.com/health`
- Check network connectivity from backend server
- Implement retry logic with exponential backoff

---

## Security Best Practices

1. **Always use HTTPS in production**
2. **Never log JWT tokens** (they contain sensitive information)
3. **Validate tokens on every protected request** (don't trust client-side auth state)
4. **Set short token expiration times** (current: 24 hours)
5. **Implement token refresh** if needed for long-lived sessions
6. **Store tokens securely** (localStorage is acceptable for web apps)
7. **Clear tokens on logout** to prevent session hijacking

---

## Additional Resources

- **Auth Service Repository:** https://github.com/your-org/auth-service
- **Auth Service Docs:** See `auth-service/README.md`
- **Example Integration:** See `option-platform` repository

---

## Support

For questions or issues:
1. Check this guide and troubleshooting section
2. Review example integration in `option-platform`
3. Check auth-service logs: `kubectl logs -n auth-service -l app=backend`
4. Open an issue in the auth-service repository
