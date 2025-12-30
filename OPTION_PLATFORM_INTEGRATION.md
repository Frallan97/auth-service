# Option Platform Integration with Central Auth Service

## Overview

This document outlines the plan to integrate `options.vibeoholic.com` (option-platform) with the central auth-service at `auth.vibeoholic.com`. This will replace the current simple username-based authentication with Google OAuth through the central auth service.

**Target:** Users log in via Google OAuth at auth-service, then access options.vibeoholic.com with a validated JWT token.

---

## Current State Analysis

### Option Platform (options.vibeoholic.com)

**Current Authentication:**
- Simple username-based login (no password)
- `POST /api/users/login` with `{"username": "..."}`
- `LoginOrRegister()` function creates/finds users by username
- No real security - MVP placeholder authentication

**User Model:**
```go
type User struct {
    ID           uuid.UUID
    Email        string
    Username     string
    PasswordHash string  // Currently just "test"
    CreatedAt    time.Time
    UpdatedAt    time.Time
}
```

**Sessions:**
- Trading sessions tied to `user_id`
- Portfolios, orders, trades tracked per session
- No JWT validation currently

### Auth Service (auth.vibeoholic.com)

**Current Capabilities:**
- ✅ Google OAuth working
- ✅ JWT token generation (RS256)
- ✅ Refresh tokens with rotation
- ✅ User management
- ✅ Public key endpoint for token validation
- ✅ Ready for multi-application support

**Missing for Multi-App:**
- ❌ Application registry (see MULTI_SOLUTION_PLAN.md Phase 1)
- ❌ Application-scoped tokens (app_id claim)
- ❌ Application user management

---

## Integration Approach

### Option A: Quick Integration (Recommended for Now)

**Use auth-service as-is** before implementing multi-application features.

**Pros:**
- Fast to implement (1-2 days)
- Gets real authentication working quickly
- Can migrate to full multi-app later

**Cons:**
- Tokens won't have app_id initially
- No per-application role management yet

**Timeline:** Can start immediately

### Option B: Wait for Phase 1 Multi-App Features

Implement Phase 1 of MULTI_SOLUTION_PLAN.md first:
- Add applications table
- Add application_users table
- Update JWT tokens with app_id

**Pros:**
- Proper multi-app architecture from the start
- Better long-term solution

**Cons:**
- Requires 1-2 weeks of additional auth-service development
- Delays getting option-platform secured

**Recommendation:** Use **Option A** now, migrate to full multi-app in Phase 2

---

## Implementation Plan - Quick Integration (Option A)

### Phase 1: Register option-platform OAuth Client (30 minutes)

Since we don't have the applications table yet, we'll use a second OAuth client in Google Cloud Console.

#### Step 1: Create OAuth Client for option-platform

```bash
# Open Google Cloud Console
open "https://console.cloud.google.com/apis/credentials?project=auth-service-481021"
```

1. Click **"+ CREATE CREDENTIALS"** → **"OAuth client ID"**
2. **Application type:** Web application
3. **Name:** option-platform-production
4. **Authorized JavaScript origins:**
   ```
   https://options.vibeoholic.com
   https://auth.vibeoholic.com
   ```
5. **Authorized redirect URIs:**
   ```
   https://auth.vibeoholic.com/api/auth/google/callback?app=option-platform
   https://options.vibeoholic.com/auth/callback
   ```
6. Click **Create**
7. **Save the Client ID and Secret** (we'll use same ones as auth-service for now)

**Alternative (Simpler for MVP):** Use the same OAuth client as auth-service:
- Client ID: `456207493374-6f7uhqe17piiqiv626gvl65qtcm625l0.apps.googleusercontent.com`
- Just add `https://options.vibeoholic.com` to authorized origins
- Add `https://options.vibeoholic.com/auth/callback` to redirect URIs

---

### Phase 2: Backend Changes (option-platform)

#### Step 2.1: Add JWT Validation Package

**File:** `option-platform/backend/pkg/auth/jwt.go` (new file)

```go
package auth

import (
	"crypto/rsa"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/golang-jwt/jwt/v5"
)

const AuthServicePublicKeyURL = "https://auth.vibeoholic.com/api/auth/public-key"

// Claims represents the JWT token claims
type Claims struct {
	UserID string `json:"sub"`
	Email  string `json:"email"`
	Name   string `json:"name"`
	jwt.RegisteredClaims
}

// FetchPublicKey fetches the public key from auth-service
func FetchPublicKey() (*rsa.PublicKey, error) {
	resp, err := http.Get(AuthServicePublicKeyURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch public key: %w", err)
	}
	defer resp.Body.Close()

	keyData, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read public key: %w", err)
	}

	publicKey, err := jwt.ParseRSAPublicKeyFromPEM(keyData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
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

#### Step 2.2: Add Authentication Middleware

**File:** `option-platform/backend/middleware/auth.go` (new file)

```go
package middleware

import (
	"crypto/rsa"
	"net/http"
	"strings"

	"github.com/frans-sjostrom/option-platform/backend2/pkg/auth"
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
```

#### Step 2.3: Initialize Public Key on Startup

**File:** `option-platform/backend/cmd/api/main.go`

Add at startup (after database initialization):

```go
// Fetch auth-service public key for JWT validation
log.Println("Fetching auth-service public key...")
publicKey, err := auth.FetchPublicKey()
if err != nil {
	log.Fatal("Failed to fetch auth-service public key:", err)
}
log.Println("Successfully fetched auth-service public key")
```

#### Step 2.4: Update User Endpoints

Replace the simple login endpoint with auth-service integration:

**File:** `option-platform/backend/cmd/api/main.go`

```go
// User endpoints
users := r.Group("/api/users")
{
	// Login/register via auth-service (frontend handles OAuth redirect)
	// This endpoint accepts the JWT token after OAuth completes
	users.POST("/auth/callback", func(c *gin.Context) {
		var req struct {
			Token string `json:"token" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}

		// Validate token
		claims, err := auth.ValidateToken(req.Token, publicKey)
		if err != nil {
			c.JSON(401, gin.H{"error": "invalid token"})
			return
		}

		// Create or update user from JWT claims
		userID, err := uuid.Parse(claims.UserID)
		if err != nil {
			c.JSON(400, gin.H{"error": "invalid user_id in token"})
			return
		}

		// Check if user exists
		var user models.User
		result := database.DB.Where("id = ?", userID).First(&user)

		if result.Error != nil {
			// User doesn't exist, create from token claims
			user = models.User{
				ID:       userID,
				Email:    claims.Email,
				Username: claims.Name, // Use Google name as username
			}
			if err := database.DB.Create(&user).Error; err != nil {
				c.JSON(500, gin.H{"error": "failed to create user"})
				return
			}
		} else {
			// Update user info if changed
			user.Email = claims.Email
			user.Username = claims.Name
			database.DB.Save(&user)
		}

		c.JSON(200, user)
	})

	// Get user profile (protected endpoint)
	users.GET("/me", AuthMiddleware(publicKey), func(c *gin.Context) {
		userID := c.GetString("user_id")

		var user models.User
		if err := database.DB.Where("id = ?", userID).First(&user).Error; err != nil {
			c.JSON(404, gin.H{"error": "user not found"})
			return
		}

		c.JSON(200, user)
	})
}

// Protect session endpoints with auth
sessions := r.Group("/api/sessions")
sessions.Use(AuthMiddleware(publicKey)) // Apply to all session routes
{
	// ... existing session endpoints ...
}

// Protect other endpoints as needed
orders := r.Group("/api/orders")
orders.Use(AuthMiddleware(publicKey))
{
	// ... existing order endpoints ...
}
```

#### Step 2.5: Update go.mod

```bash
cd /home/frans-sjostrom/Documents/hezner-hosted-projects/option-platform/backend
go get github.com/golang-jwt/jwt/v5
go mod tidy
```

---

### Phase 3: Frontend Changes (option-platform)

#### Step 3.1: Create Auth Service

**File:** `option-platform/frontend/src/lib/auth.ts`

```typescript
const AUTH_SERVICE_URL = 'https://auth.vibeoholic.com'
const API_URL = import.meta.env.VITE_API_URL || 'http://localhost:8080'

export interface AuthTokens {
  access_token: string
  refresh_token?: string
}

export interface User {
  id: string
  email: string
  username: string
}

export class AuthService {
  private static readonly TOKEN_KEY = 'option_platform_token'
  private static readonly USER_KEY = 'option_platform_user'

  // Redirect to auth-service for login
  static login() {
    const redirectUrl = encodeURIComponent(window.location.origin + '/auth/callback')
    window.location.href = `${AUTH_SERVICE_URL}/api/auth/google/login?redirect=${redirectUrl}`
  }

  // Handle OAuth callback
  static async handleCallback(code: string): Promise<User> {
    // Exchange code for token (if auth-service provides this endpoint)
    // OR get token from URL params/hash
    const urlParams = new URLSearchParams(window.location.search)
    const token = urlParams.get('token') || sessionStorage.getItem('temp_token')

    if (!token) {
      throw new Error('No token received from auth-service')
    }

    // Save token
    this.setToken(token)

    // Authenticate with our backend
    const response = await fetch(`${API_URL}/api/users/auth/callback`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ token }),
    })

    if (!response.ok) {
      throw new Error('Failed to authenticate with backend')
    }

    const user = await response.json()
    this.setUser(user)
    return user
  }

  // Logout
  static async logout() {
    this.clearAuth()
    // Optional: call auth-service logout
    await fetch(`${AUTH_SERVICE_URL}/api/auth/logout`, {
      method: 'POST',
      credentials: 'include',
    })
    window.location.href = '/login'
  }

  // Get current user
  static getUser(): User | null {
    const userStr = localStorage.getItem(this.USER_KEY)
    return userStr ? JSON.parse(userStr) : null
  }

  // Get token
  static getToken(): string | null {
    return localStorage.getItem(this.TOKEN_KEY)
  }

  // Check if authenticated
  static isAuthenticated(): boolean {
    return !!this.getToken()
  }

  // Set token
  private static setToken(token: string) {
    localStorage.setItem(this.TOKEN_KEY, token)
  }

  // Set user
  private static setUser(user: User) {
    localStorage.setItem(this.USER_KEY, JSON.stringify(user))
  }

  // Clear auth
  private static clearAuth() {
    localStorage.removeItem(this.TOKEN_KEY)
    localStorage.removeItem(this.USER_KEY)
  }

  // Add token to requests
  static getAuthHeader(): { Authorization: string } | {} {
    const token = this.getToken()
    return token ? { Authorization: `Bearer ${token}` } : {}
  }
}
```

#### Step 3.2: Create Auth Callback Page

**File:** `option-platform/frontend/src/pages/AuthCallback.tsx`

```typescript
import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { AuthService } from '../lib/auth'

export default function AuthCallback() {
  const navigate = useNavigate()
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    const handleAuth = async () => {
      try {
        const urlParams = new URLSearchParams(window.location.search)
        const code = urlParams.get('code')

        if (!code) {
          setError('No authorization code received')
          return
        }

        await AuthService.handleCallback(code)
        navigate('/dashboard')
      } catch (err) {
        console.error('Auth callback error:', err)
        setError(err instanceof Error ? err.message : 'Authentication failed')
      }
    }

    handleAuth()
  }, [navigate])

  if (error) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <div className="text-center">
          <h1 className="text-2xl font-bold text-red-600 mb-4">Authentication Error</h1>
          <p className="text-gray-600 mb-4">{error}</p>
          <button
            onClick={() => AuthService.login()}
            className="px-4 py-2 bg-blue-600 text-white rounded hover:bg-blue-700"
          >
            Try Again
          </button>
        </div>
      </div>
    )
  }

  return (
    <div className="flex items-center justify-center min-h-screen">
      <div className="text-center">
        <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-600 mx-auto mb-4"></div>
        <p className="text-gray-600">Completing authentication...</p>
      </div>
    </div>
  )
}
```

#### Step 3.3: Create Login Page

**File:** `option-platform/frontend/src/pages/Login.tsx`

```typescript
import { AuthService } from '../lib/auth'

export default function Login() {
  return (
    <div className="flex items-center justify-center min-h-screen bg-gray-100">
      <div className="bg-white p-8 rounded-lg shadow-md w-full max-w-md">
        <h1 className="text-3xl font-bold text-center mb-8">Option Cockpit</h1>
        <p className="text-gray-600 text-center mb-8">
          Trading bot testing platform for options
        </p>
        <button
          onClick={() => AuthService.login()}
          className="w-full flex items-center justify-center gap-3 px-6 py-3 bg-white border border-gray-300 rounded-lg hover:bg-gray-50 transition-colors"
        >
          <svg className="w-6 h-6" viewBox="0 0 24 24">
            <path
              fill="#4285F4"
              d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z"
            />
            <path
              fill="#34A853"
              d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"
            />
            <path
              fill="#FBBC05"
              d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z"
            />
            <path
              fill="#EA4335"
              d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z"
            />
          </svg>
          <span className="text-gray-700 font-medium">Sign in with Google</span>
        </button>
      </div>
    </div>
  )
}
```

#### Step 3.4: Update API Client to Include Token

**File:** `option-platform/frontend/src/lib/api.ts` (update existing)

```typescript
import { AuthService } from './auth'

const API_URL = import.meta.env.VITE_API_URL || 'http://localhost:8080'

async function fetchAPI(endpoint: string, options: RequestInit = {}) {
  const response = await fetch(`${API_URL}${endpoint}`, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...AuthService.getAuthHeader(), // Add JWT token
      ...options.headers,
    },
  })

  if (response.status === 401) {
    // Token expired or invalid
    AuthService.logout()
    throw new Error('Unauthorized')
  }

  if (!response.ok) {
    throw new Error(`API error: ${response.statusText}`)
  }

  return response.json()
}

// Example: Get user sessions
export async function getUserSessions(userId: string) {
  return fetchAPI(`/api/sessions?user_id=${userId}`)
}

// ... other API functions ...
```

#### Step 3.5: Add Protected Route Component

**File:** `option-platform/frontend/src/components/ProtectedRoute.tsx`

```typescript
import { Navigate } from 'react-router-dom'
import { AuthService } from '../lib/auth'

interface ProtectedRouteProps {
  children: React.ReactNode
}

export default function ProtectedRoute({ children }: ProtectedRouteProps) {
  if (!AuthService.isAuthenticated()) {
    return <Navigate to="/login" replace />
  }

  return <>{children}</>
}
```

#### Step 3.6: Update Routes

**File:** `option-platform/frontend/src/App.tsx`

```typescript
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import Login from './pages/Login'
import AuthCallback from './pages/AuthCallback'
import Dashboard from './pages/Dashboard'
import ProtectedRoute from './components/ProtectedRoute'

function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/login" element={<Login />} />
        <Route path="/auth/callback" element={<AuthCallback />} />
        <Route
          path="/dashboard"
          element={
            <ProtectedRoute>
              <Dashboard />
            </ProtectedRoute>
          }
        />
        <Route path="/" element={<Navigate to="/dashboard" replace />} />
      </Routes>
    </BrowserRouter>
  )
}

export default App
```

---

### Phase 4: Testing & Deployment

#### Step 4.1: Local Testing

```bash
# Backend
cd option-platform/backend
go mod tidy
go run cmd/api/main.go

# Frontend
cd option-platform/frontend
bun install
bun dev
```

**Test Flow:**
1. Go to `http://localhost:5173/login`
2. Click "Sign in with Google"
3. Redirects to auth.vibeoholic.com
4. Sign in with Google
5. Redirects back to `http://localhost:5173/auth/callback`
6. Should show dashboard

#### Step 4.2: Deploy to Production

```bash
# Commit changes
git add .
git commit -m "Integrate with central auth-service

- Add JWT validation using auth-service public key
- Add auth middleware for protected endpoints
- Update frontend to use OAuth flow via auth-service
- Remove simple username-based login
- Add /auth/callback route for OAuth redirect"

git push origin main
```

GitHub Actions should build and deploy automatically.

#### Step 4.3: Update Google OAuth Redirect URIs

Add to your OAuth client in Google Cloud Console:
```
https://options.vibeoholic.com
https://options.vibeoholic.com/auth/callback
```

#### Step 4.4: Verify Production

1. Go to https://options.vibeoholic.com
2. Should redirect to login if not authenticated
3. Click "Sign in with Google"
4. Auth flow completes
5. Redirected back to dashboard
6. Can create sessions, place orders, etc.

---

## Migration Strategy

### For Existing Users

If you have existing users in option-platform database:

1. **Keep existing User table** - Don't drop it
2. **Match by email** - When JWT token comes in, match user by email
3. **Update user ID** - Update the user ID to match auth-service user ID
4. **Migrate sessions** - Update foreign keys to new user IDs

**Migration Script Example:**

```sql
-- Create temporary mapping table
CREATE TABLE user_id_mapping (
    old_user_id UUID,
    new_user_id UUID,
    email TEXT
);

-- After first login with new system, populate mapping
-- Then update foreign keys:

UPDATE sessions
SET user_id = (SELECT new_user_id FROM user_id_mapping WHERE old_user_id = sessions.user_id)
WHERE user_id IN (SELECT old_user_id FROM user_id_mapping);

-- Similar for other tables with user_id foreign key
```

---

## Security Considerations

### 1. Token Validation
✅ Always validate JWT signature using auth-service public key
✅ Check token expiration
✅ Cache public key (refresh every hour)

### 2. CORS Configuration
```go
// Allow auth-service origin
cors.New(cors.Config{
    AllowOrigins: []string{
        "https://options.vibeoholic.com",
        "https://auth.vibeoholic.com",
    },
    AllowMethods:     []string{"GET", "POST", "PUT", "DELETE"},
    AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
    AllowCredentials: true,
})
```

### 3. Token Storage
- Store in `localStorage` (persists across tabs)
- OR `sessionStorage` (more secure, single tab)
- Never store in cookies (different domain)

### 4. Rate Limiting
- Apply rate limits to public endpoints
- Protected endpoints have JWT validation

---

## Future Enhancements (Phase 2)

Once Phase 1 of MULTI_SOLUTION_PLAN.md is complete:

1. **Register as Application** in auth-service applications table
2. **Get app_id in JWT** tokens
3. **Per-app roles** - Admin vs regular user
4. **Token refresh** - Implement refresh token flow
5. **Casbin integration** - Fine-grained permissions
6. **SSO** - Single sign-on across all apps

---

## Troubleshooting

### Issue: "Failed to fetch public key"

**Solution:** Check auth-service is running and `/api/auth/public-key` is accessible

```bash
curl https://auth.vibeoholic.com/api/auth/public-key
```

### Issue: "Invalid token" errors

**Causes:**
- Token expired (15 min default)
- Public key mismatch
- Token from wrong auth-service instance

**Solution:**
- Implement refresh token flow
- Verify public key is correct
- Check token expiration time

### Issue: CORS errors

**Solution:** Add auth-service origin to CORS config in option-platform backend

### Issue: User not found after login

**Solution:** Ensure `/api/users/auth/callback` creates user from JWT claims

---

## Testing Checklist

- [ ] Can log in via Google OAuth
- [ ] JWT token is stored in localStorage
- [ ] Token is sent in Authorization header
- [ ] Protected endpoints validate token
- [ ] Invalid tokens are rejected
- [ ] User profile loads correctly
- [ ] Can create trading sessions
- [ ] Can place orders
- [ ] Can log out
- [ ] Logout clears token and redirects to login

---

## Deployment Commands Quick Reference

```bash
# Backend
cd option-platform/backend
go get github.com/golang-jwt/jwt/v5
go mod tidy
go build -o app cmd/api/main.go

# Frontend
cd option-platform/frontend
bun install
bun run build

# Deploy (if using Docker)
docker-compose up --build

# Or push to Git for GitHub Actions
git push origin main
```

---

## Success Criteria

✅ Users can log in to options.vibeoholic.com using Google OAuth via auth-service
✅ JWT tokens are validated on every request
✅ Unauthorized requests return 401
✅ User sessions are tied to authenticated users
✅ All trading functionality works as before
✅ No security vulnerabilities (validated tokens, HTTPS, etc.)

---

## Timeline Estimate

| Phase | Task | Time |
|-------|------|------|
| 1 | Update Google OAuth config | 15 min |
| 2 | Backend JWT validation | 2-3 hours |
| 3 | Frontend OAuth flow | 2-3 hours |
| 4 | Testing & debugging | 1-2 hours |
| 5 | Deploy to production | 30 min |

**Total:** 6-9 hours of development time

**Recommendation:** Start this week, have it live by end of week!

---

## Next Steps

1. ✅ Read and approve this plan
2. 🔧 Implement backend JWT validation
3. 🎨 Implement frontend OAuth flow
4. 🧪 Test locally
5. 🚀 Deploy to production
6. 📊 Monitor for issues
7. 📝 Begin Phase 1 of MULTI_SOLUTION_PLAN.md (applications table, etc.)
