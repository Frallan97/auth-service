# Multi-Solution Auth Service - Implementation Plan

## Executive Summary

This document outlines the plan to transform the auth-service from a single-solution authentication system to a multi-tenant authentication platform that can serve multiple applications/solutions with centralized user management.

**Current State:** Auth-service supports Google OAuth for a single application with hardcoded configuration.

**Target State:** Auth-service supports multiple registered applications, each with their own OAuth configuration, CORS settings, and customization options, while maintaining centralized user management.

---

## Current Architecture Analysis

### What We Have

1. **Backend (Go + Chi Router)**
   - Google OAuth 2.0 authentication
   - JWT token generation (RS256)
   - Rotating refresh tokens
   - User management API
   - Audit logging
   - Deployed at: **https://auth.vibeoholic.com** (NOT auth.vibeoholic.com)

2. **Database (PostgreSQL)**
   - `users` - User accounts with Google OAuth info
   - `refresh_tokens` - Rotating refresh tokens
   - `auth_audit_log` - Authentication event logging

3. **Frontend (React + TypeScript)**
   - Admin dashboard for user management
   - Google OAuth login flow
   - Token management

4. **Kubernetes Deployment**
   - Deployed via ArgoCD
   - Running in auth-service namespace
   - Ingress at auth.vibeoholic.com
   - PostgreSQL StatefulSet for persistence

### Current Limitations for Multi-Solution Support

1. **Single OAuth Configuration**
   - Only one Google OAuth client ID/secret
   - Hardcoded redirect URL: `https://auth.vibeoholic.com/api/auth/google/callback`
   - No way to add additional OAuth providers or clients

2. **Single Origin Support**
   - CORS ALLOWED_ORIGINS is hardcoded to single domain
   - Cannot support multiple frontend applications

3. **No Application/Tenant Separation**
   - No concept of "applications" or "solutions" in the database
   - Cannot track which solution a user belongs to or is logging into
   - Cannot differentiate tokens between solutions

4. **Single Frontend**
   - Frontend is specific to auth-service admin UI
   - No embeddable login widget or redirect-based flow for other solutions

5. **Monolithic Configuration**
   - All configuration through environment variables
   - No runtime configuration or application registration

---

## Target Architecture

### Multi-Tenant Model

```
┌─────────────────────────────────────────────────────────────┐
│                    Auth Service (Central)                   │
│  https://auth.vibeoholic.com                               │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌────────────┐  ┌────────────┐  ┌────────────┐          │
│  │ Application│  │ Application│  │ Application│          │
│  │  Registry  │  │   Users    │  │   Sessions │          │
│  └────────────┘  └────────────┘  └────────────┘          │
│                                                             │
└─────────────────────────────────────────────────────────────┘
         │                    │                    │
         │                    │                    │
         ▼                    ▼                    ▼
┌─────────────┐      ┌─────────────┐      ┌─────────────┐
│  Solution A │      │  Solution B │      │  Solution C │
│  hackaton   │      │   sharon    │      │   option    │
│             │      │             │      │  platform   │
└─────────────┘      └─────────────┘      └─────────────┘
```

### Key Concepts

1. **Application Registration**: Each solution registers with the auth-service
2. **Centralized Users**: One user database, users can access multiple solutions
3. **Application Context**: Tokens include application ID for authorization
4. **Flexible OAuth**: Each application can have its own OAuth configuration
5. **Dynamic CORS**: CORS origins configured per application

---

## Critical Issue to Fix First: Localhost Redirect

### Problem

**Current Issue:** When users try to log in at https://auth.vibeoholic.com, they get redirected to `http://localhost:8080/api/auth/google/login` instead of the production URL.

**Root Cause:** The frontend React application has `VITE_API_URL` hardcoded to `http://localhost:8080` as a fallback value. During the Docker build, this environment variable is not set, so it defaults to localhost. Vite bakes environment variables into the JavaScript bundle at build time, not runtime.

**Location:** `frontend/src/services/api.ts`:
```typescript
const API_URL = import.meta.env.VITE_API_URL || 'http://localhost:8080'
```

### Solution

The Dockerfile needs to accept `VITE_API_URL` as a build argument and pass it during the build step:

**Updated Dockerfile:**
```dockerfile
# Build stage
FROM oven/bun:1 AS builder

# Build argument for API URL
ARG VITE_API_URL=https://auth.vibeoholic.com

WORKDIR /app

# Copy package files
COPY package.json bun.lock ./

# Install dependencies
RUN bun install --frozen-lockfile

# Copy source code
COPY . .

# Build the application with environment variable
ENV VITE_API_URL=$VITE_API_URL
RUN bun run build

# Production stage
FROM nginx:alpine

# Copy built assets from builder
COPY --from=builder /app/dist /usr/share/nginx/html

# Copy nginx configuration
COPY nginx.conf /etc/nginx/conf.d/default.conf

# Expose port
EXPOSE 3000

# Start nginx
CMD ["nginx", "-g", "daemon off;"]
```

**Update values.yaml or build arguments:**

If using GitHub Actions to build:
```yaml
- name: Build frontend image
  run: |
    docker build \
      --build-arg VITE_API_URL=https://auth.vibeoholic.com \
      -t ghcr.io/frallan97/auth-service-frontend:latest \
      ./frontend
```

**Immediate Fix (Manual):**

1. Update the Dockerfile
2. Rebuild the frontend image with the correct build arg
3. Push to registry
4. Restart the frontend pods

```bash
cd auth-service/frontend
docker build --build-arg VITE_API_URL=https://auth.vibeoholic.com \
  -t ghcr.io/frallan97/auth-service-frontend:latest .
docker push ghcr.io/frallan97/auth-service-frontend:latest

# Restart pods
kubectl rollout restart deployment/auth-service-frontend -n auth-service
```

---

## Authorization Strategy: Casbin Implementation

### Question: Should Casbin be centralized in the auth-service or in each application?

**Answer: Implement Casbin in EACH APPLICATION SEPARATELY**

### Reasoning

#### Auth Service Responsibilities (Authentication)
- **Who are you?** - Verify user identity
- **User management** - Create, read, update user profiles
- **Token generation** - Issue JWT tokens with user identity
- **Basic roles** - Provide a basic role per application (user, admin, owner)

#### Application Responsibilities (Authorization)
- **What can you do?** - Define and enforce permissions
- **Resource ownership** - Manage application-specific resources
- **Custom policies** - Define application-specific access rules
- **Performance** - Make fast local authorization decisions

### Why Distribute Casbin?

1. **Different Authorization Requirements**
   - Each application has unique resources and permissions
   - hackaton: hackathon submissions, judging, team management
   - sharon: shared items, friend requests, email invitations
   - option-platform: trading strategies, backtests, portfolios
   - These don't map to a common permission model

2. **Scalability**
   - Centralized authorization becomes a bottleneck
   - Every request to any app would need to call auth-service
   - Adds network latency and single point of failure

3. **Autonomy**
   - Applications can evolve their permission models independently
   - No cross-application dependencies for authorization changes
   - Faster development and deployment cycles

4. **Performance**
   - Local Casbin enforcement is microseconds
   - Remote API calls are milliseconds
   - Critical for high-throughput applications

5. **Security**
   - Application-specific data stays within application
   - Auth service doesn't need knowledge of all application resources
   - Reduced attack surface

### Implementation Pattern

```
┌─────────────────────────────────────────────────────┐
│              Auth Service                          │
│  - Authenticates user                              │
│  - Issues JWT with:                                │
│    * user_id                                       │
│    * email                                         │
│    * app_id                                        │
│    * role (basic: user/admin)                      │
└─────────────────────────────────────────────────────┘
                      │
                      │ JWT Token
                      ▼
┌─────────────────────────────────────────────────────┐
│              Application (e.g., hackaton)          │
│  1. Validate JWT signature (using public key)     │
│  2. Extract user_id and role from JWT             │
│  3. Use Casbin to check permissions:              │
│     - enforcer.Enforce(user_id, resource, action) │
│  4. Allow/Deny request                            │
│                                                     │
│  Casbin Policies (stored in app's database):      │
│  - user-123, submission:456, read                 │
│  - user-123, submission:456, write                │
│  - admin-role, submissions, delete                │
└─────────────────────────────────────────────────────┘
```

### What Auth Service Provides

The JWT token includes a **basic role** per application:

```go
type Claims struct {
    UserID        string `json:"sub"`
    Email         string `json:"email"`
    Name          string `json:"name"`
    ApplicationID string `json:"app_id"`
    Role          string `json:"role"`   // "user", "admin", or "owner"
    jwt.RegisteredClaims
}
```

### What Each Application Does with It

**Example: hackaton-web2**

```go
// 1. Validate JWT (authentication)
claims := validateJWT(token, publicKey)

// 2. Initialize Casbin with app-specific policies
enforcer, _ := casbin.NewEnforcer("hackaton_model.conf", "hackaton_policies.db")

// 3. Check permissions (authorization)
allowed, _ := enforcer.Enforce(claims.UserID, "submission:123", "read")
if !allowed {
    return http.StatusForbidden
}

// If user has admin role from JWT, grant them admin permissions
if claims.Role == "admin" {
    enforcer.AddRoleForUser(claims.UserID, "hackaton-admin")
}
```

### Casbin Model Examples

**RBAC Model (for most applications):**
```ini
[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act

[role_definition]
g = _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r.sub, p.sub) && r.obj == p.obj && r.act == p.act
```

**Policies for hackaton:**
```csv
p, hackaton-admin, submissions, *
p, hackaton-admin, users, *
p, hackaton-judge, submissions, read
p, hackaton-judge, submissions, grade
p, user, submission:own, *
g, user-uuid-123, hackaton-admin
g, user-uuid-456, hackaton-judge
```

### When to Use the Basic Role from JWT

The `role` field in the JWT (user/admin/owner) serves two purposes:

1. **Bootstrap permissions** - When a user first accesses an application
2. **Global admin access** - "owner" role = full access to everything

```go
// Example: Auto-assign Casbin role based on JWT role
if claims.Role == "owner" {
    enforcer.AddRoleForUser(claims.UserID, "global-admin")
} else if claims.Role == "admin" {
    enforcer.AddRoleForUser(claims.UserID, "app-admin")
} else {
    enforcer.AddRoleForUser(claims.UserID, "user")
}
```

### Summary: Separation of Concerns

| Responsibility | Auth Service | Applications |
|----------------|--------------|--------------|
| User authentication | ✅ Yes | ❌ No |
| User identity (who) | ✅ Yes | ❌ No |
| JWT tokens | ✅ Yes | ❌ No |
| Basic role (user/admin) | ✅ Yes | ❌ No |
| Resource permissions | ❌ No | ✅ Yes |
| Detailed authorization | ❌ No | ✅ Yes |
| Casbin policies | ❌ No | ✅ Yes |
| Business logic | ❌ No | ✅ Yes |

**Key Principle:** Auth service authenticates, applications authorize.

---

## Implementation Plan

## Phase 1: Database Schema Changes

### 1.1 Add Applications Table

Create a new table to register applications/solutions:

```sql
CREATE TABLE applications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(100) UNIQUE NOT NULL, -- e.g., 'hackaton', 'sharon', 'option-platform'
    description TEXT,

    -- OAuth Configuration
    google_client_id VARCHAR(255),
    google_client_secret VARCHAR(255),
    google_redirect_url VARCHAR(500),

    -- Frontend Configuration
    allowed_origins TEXT[], -- Array of allowed CORS origins
    login_redirect_url VARCHAR(500), -- Where to redirect after login
    logout_redirect_url VARCHAR(500), -- Where to redirect after logout

    -- Customization
    logo_url TEXT,
    primary_color VARCHAR(7), -- Hex color code

    -- Security
    is_active BOOLEAN DEFAULT true,
    require_email_verification BOOLEAN DEFAULT false,
    allowed_email_domains TEXT[], -- Optional: restrict to specific email domains

    -- Metadata
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    created_by UUID REFERENCES users(id),

    CONSTRAINT chk_slug_format CHECK (slug ~ '^[a-z0-9-]+$')
);

CREATE INDEX idx_applications_slug ON applications(slug);
CREATE INDEX idx_applications_is_active ON applications(is_active);
```

### 1.2 Add Application-User Relationship

Track which users have access to which applications:

```sql
CREATE TABLE application_users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    application_id UUID NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    -- Role within this application (optional, for basic RBAC)
    role VARCHAR(50) DEFAULT 'user', -- e.g., 'user', 'admin', 'owner'

    -- Access control
    is_active BOOLEAN DEFAULT true,

    -- Metadata
    created_at TIMESTAMP DEFAULT NOW(),
    last_accessed_at TIMESTAMP,

    CONSTRAINT uq_application_user UNIQUE(application_id, user_id)
);

CREATE INDEX idx_application_users_app_id ON application_users(application_id);
CREATE INDEX idx_application_users_user_id ON application_users(user_id);
CREATE INDEX idx_application_users_is_active ON application_users(is_active);
```

### 1.3 Add Application Context to Refresh Tokens

Track which application a refresh token belongs to:

```sql
ALTER TABLE refresh_tokens
ADD COLUMN application_id UUID REFERENCES applications(id) ON DELETE CASCADE;

CREATE INDEX idx_refresh_tokens_application_id ON refresh_tokens(application_id);
```

### 1.4 Enhance Audit Log

Add application context to audit logs:

```sql
ALTER TABLE auth_audit_log
ADD COLUMN application_id UUID REFERENCES applications(id) ON DELETE SET NULL;

CREATE INDEX idx_audit_log_application_id ON auth_audit_log(application_id);
```

### 1.5 Migration Strategy

1. Create migration files:
   - `002_add_applications.up.sql`
   - `002_add_applications.down.sql`

2. Seed default application for existing auth-service:
   ```sql
   INSERT INTO applications (name, slug, description, google_client_id, google_client_secret, google_redirect_url, allowed_origins, login_redirect_url, logout_redirect_url)
   VALUES (
       'Auth Service Admin',
       'auth-admin',
       'Authentication service administration portal',
       '<existing_google_client_id>',
       '<existing_google_client_secret>',
       'https://auth.vibeoholic.com/api/auth/google/callback',
       ARRAY['https://auth.vibeoholic.com'],
       'https://auth.vibeoholic.com/dashboard',
       'https://auth.vibeoholic.com'
   );
   ```

3. Migrate existing users to the default application:
   ```sql
   INSERT INTO application_users (application_id, user_id, role)
   SELECT
       (SELECT id FROM applications WHERE slug = 'auth-admin'),
       id,
       'user'
   FROM users;
   ```

---

## Phase 2: Backend API Changes

### 2.1 New Models

Add Go structs in `backend/internal/models/models.go`:

```go
type Application struct {
    ID                    uuid.UUID      `json:"id" db:"id"`
    Name                  string         `json:"name" db:"name"`
    Slug                  string         `json:"slug" db:"slug"`
    Description           *string        `json:"description,omitempty" db:"description"`
    GoogleClientID        *string        `json:"-" db:"google_client_id"` // Never expose in API
    GoogleClientSecret    *string        `json:"-" db:"google_client_secret"` // Never expose in API
    GoogleRedirectURL     *string        `json:"google_redirect_url,omitempty" db:"google_redirect_url"`
    AllowedOrigins        []string       `json:"allowed_origins" db:"allowed_origins"`
    LoginRedirectURL      *string        `json:"login_redirect_url,omitempty" db:"login_redirect_url"`
    LogoutRedirectURL     *string        `json:"logout_redirect_url,omitempty" db:"logout_redirect_url"`
    LogoURL               *string        `json:"logo_url,omitempty" db:"logo_url"`
    PrimaryColor          *string        `json:"primary_color,omitempty" db:"primary_color"`
    IsActive              bool           `json:"is_active" db:"is_active"`
    RequireEmailVerif     bool           `json:"require_email_verification" db:"require_email_verification"`
    AllowedEmailDomains   []string       `json:"allowed_email_domains,omitempty" db:"allowed_email_domains"`
    CreatedAt             time.Time      `json:"created_at" db:"created_at"`
    UpdatedAt             time.Time      `json:"updated_at" db:"updated_at"`
    CreatedBy             *uuid.UUID     `json:"created_by,omitempty" db:"created_by"`
}

type ApplicationUser struct {
    ID              uuid.UUID  `json:"id" db:"id"`
    ApplicationID   uuid.UUID  `json:"application_id" db:"application_id"`
    UserID          uuid.UUID  `json:"user_id" db:"user_id"`
    Role            string     `json:"role" db:"role"`
    IsActive        bool       `json:"is_active" db:"is_active"`
    CreatedAt       time.Time  `json:"created_at" db:"created_at"`
    LastAccessedAt  *time.Time `json:"last_accessed_at,omitempty" db:"last_accessed_at"`
}
```

### 2.2 Update JWT Token Claims

Modify `backend/pkg/jwt/jwt.go` to include application context:

```go
type Claims struct {
    UserID        string `json:"sub"`
    Email         string `json:"email"`
    Name          string `json:"name"`
    ApplicationID string `json:"app_id"` // NEW: Application context
    Role          string `json:"role"`   // NEW: Role within application
    jwt.RegisteredClaims
}

func GenerateAccessToken(userID, email, name string, applicationID uuid.UUID, role string, privateKey *rsa.PrivateKey, expiry time.Duration) (string, error) {
    claims := Claims{
        UserID:        userID,
        Email:         email,
        Name:          name,
        ApplicationID: applicationID.String(),
        Role:          role,
        RegisteredClaims: jwt.RegisteredClaims{
            ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiry)),
            IssuedAt:  jwt.NewNumericDate(time.Now()),
            Issuer:    "auth.vibeoholic.com",
        },
    }

    token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
    return token.SignedString(privateKey)
}
```

### 2.3 New API Endpoints

Add application management endpoints to `backend/internal/handlers/`:

#### Application Management (Admin Only)

```
POST   /api/applications                 - Create new application
GET    /api/applications                 - List all applications
GET    /api/applications/:slug           - Get application by slug
PUT    /api/applications/:id             - Update application
DELETE /api/applications/:id             - Delete application
POST   /api/applications/:id/activate    - Activate application
POST   /api/applications/:id/deactivate  - Deactivate application
```

#### Application User Management

```
POST   /api/applications/:slug/users            - Add user to application
GET    /api/applications/:slug/users            - List users in application
DELETE /api/applications/:slug/users/:user_id  - Remove user from application
PUT    /api/applications/:slug/users/:user_id  - Update user role in application
```

#### Public Endpoints

```
GET    /api/applications/:slug/config    - Get public application config (for frontend)
GET    /api/auth/google/login            - Modified to accept ?app=slug parameter
GET    /api/auth/google/callback         - Modified to handle application context
```

### 2.4 Update Authentication Flow

Modify `backend/internal/handlers/auth.go`:

```go
func (h *Handler) GoogleLogin(w http.ResponseWriter, r *http.Request) {
    // Get application slug from query parameter
    appSlug := r.URL.Query().Get("app")
    if appSlug == "" {
        appSlug = "auth-admin" // Default to auth-admin
    }

    // Fetch application from database
    app, err := h.getApplicationBySlug(r.Context(), appSlug)
    if err != nil || !app.IsActive {
        http.Error(w, "Invalid or inactive application", http.StatusBadRequest)
        return
    }

    // Store application ID in session/state for callback
    state := generateState() // Include app ID in state

    // Use application-specific OAuth config
    authURL := h.authService.GetGoogleAuthURLForApp(app, state)
    http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
}

func (h *Handler) GoogleCallback(w http.ResponseWriter, r *http.Request) {
    // Extract application ID from state
    state := r.URL.Query().Get("state")
    appID, err := extractAppFromState(state)

    // Fetch application
    app, err := h.getApplicationByID(r.Context(), appID)

    // Exchange code for Google user info (using app-specific OAuth config)
    googleUser, err := h.authService.ExchangeGoogleCodeForApp(r.Context(), r.URL.Query().Get("code"), app)

    // Create or update user
    user, err := h.authService.CreateOrUpdateUser(r.Context(), googleUser)

    // Check if user has access to this application
    appUser, err := h.getOrCreateApplicationUser(r.Context(), app.ID, user.ID)
    if !appUser.IsActive {
        http.Error(w, "User not authorized for this application", http.StatusForbidden)
        return
    }

    // Generate tokens with application context
    tokens, err := h.authService.GenerateTokensForApp(r.Context(), user, app, appUser.Role)

    // Set refresh token cookie
    // Redirect to application's login redirect URL
    http.Redirect(w, r, app.LoginRedirectURL, http.StatusTemporaryRedirect)
}
```

### 2.5 Dynamic CORS Configuration

Update `backend/cmd/api/main.go` to support dynamic CORS:

```go
// Create custom CORS middleware that checks application origins
func dynamicCORSMiddleware(db *database.DB) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            origin := r.Header.Get("Origin")

            // Query database for allowed origins across all active applications
            var allowed bool
            query := `
                SELECT EXISTS(
                    SELECT 1 FROM applications
                    WHERE is_active = true
                    AND $1 = ANY(allowed_origins)
                )
            `
            db.QueryRow(context.Background(), query, origin).Scan(&allowed)

            if allowed {
                w.Header().Set("Access-Control-Allow-Origin", origin)
                w.Header().Set("Access-Control-Allow-Credentials", "true")
                w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
                w.Header().Set("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type, X-CSRF-Token")
            }

            if r.Method == "OPTIONS" {
                w.WriteHeader(http.StatusOK)
                return
            }

            next.ServeHTTP(w, r)
        })
    }
}
```

**Note**: For performance, cache allowed origins in memory and refresh periodically.

---

## Phase 3: Frontend Changes

### 3.1 Universal Login Page

Create a new universal login page that works for all applications:

**Route**: `/login?app=<slug>&redirect=<url>`

Features:
- Displays application logo and branding
- Shows "Sign in to [Application Name]"
- Google OAuth button with application context
- Redirect back to application after login

### 3.2 Application Admin UI

Add new admin pages for managing applications:

```
/admin/applications           - List all applications
/admin/applications/new       - Create new application
/admin/applications/:id/edit  - Edit application
/admin/applications/:id/users - Manage application users
```

### 3.3 API Client Library (Optional)

Create a lightweight TypeScript library for easy integration:

```typescript
// @auth-service/client
import { AuthClient } from '@auth-service/client'

const auth = new AuthClient({
  authServiceURL: 'https://auth.vibeoholic.com',
  applicationSlug: 'hackaton',
  redirectURL: window.location.origin + '/auth/callback',
})

// Redirect to login
auth.login()

// Handle callback
auth.handleCallback()

// Get current user
const user = auth.getCurrentUser()

// Logout
auth.logout()
```

---

## Phase 4: Integration with Existing Solutions

### 4.1 Register Each Solution as an Application

For each existing solution (hackaton, sharon, option-platform), create an application entry:

```bash
# Via API or database seeding
curl -X POST https://auth.vibeoholic.com/api/applications \
  -H "Authorization: Bearer <admin_token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Hackaton Web2",
    "slug": "hackaton",
    "description": "Web3 hackathon platform",
    "google_client_id": "<existing_google_client_id>",
    "google_client_secret": "<existing_google_client_secret>",
    "google_redirect_url": "https://hackaton.vibeoholic.com/api/auth/callback",
    "allowed_origins": ["https://hackaton.vibeoholic.com"],
    "login_redirect_url": "https://hackaton.vibeoholic.com/dashboard",
    "logout_redirect_url": "https://hackaton.vibeoholic.com"
  }'
```

### 4.2 Update Solution Authentication

Each solution needs to be updated to use the central auth service:

#### Option 1: Redirect-Based Flow (Recommended for new integrations)

1. Remove local authentication code
2. Redirect users to auth service for login:
   ```
   https://auth.vibeoholic.com/login?app=hackaton&redirect=https://hackaton.vibeoholic.com/auth/callback
   ```
3. Handle callback with JWT token
4. Store token and use for API requests

#### Option 2: Keep Local Auth, Validate with Auth Service

1. Keep existing Google OAuth in solution
2. After login, call auth service to register/get user
3. Use auth service's JWT tokens
4. Validate tokens using public key

### 4.3 Update Each Solution's Backend

For solutions that validate JWTs locally:

1. Fetch public key from auth service
2. Validate JWT signature
3. Extract `app_id` claim and verify it matches your application
4. Use `user_id` and `role` for authorization

Example middleware:

```go
func authMiddleware(publicKey *rsa.PublicKey, expectedAppID string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            token := extractToken(r)

            claims, err := validateToken(token, publicKey)
            if err != nil {
                http.Error(w, "Unauthorized", http.StatusUnauthorized)
                return
            }

            // Verify application context
            if claims.ApplicationID != expectedAppID {
                http.Error(w, "Token not valid for this application", http.StatusForbidden)
                return
            }

            // Add user context and continue
            ctx := context.WithValue(r.Context(), "userID", claims.UserID)
            ctx = context.WithValue(ctx, "role", claims.Role)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}
```

---

## Phase 5: Advanced Features (Optional)

### 5.1 Email/Password Authentication

Add support for email/password in addition to Google OAuth:

- Password hashing with bcrypt
- Email verification
- Password reset flow
- Per-application configuration (allow/disallow email/password)

### 5.2 Additional OAuth Providers

Support for more OAuth providers:

- GitHub
- Microsoft
- Facebook
- Custom OAuth2 providers

### 5.3 SSO (Single Sign-On)

Allow users to sign in once and access all applications:

- Session sharing across applications
- Automatic token refresh
- Cross-domain cookie handling (complex)

### 5.4 User Consent Screen

Show consent screen when user first accesses a new application:

```
[Application Logo]
"Hackaton Web2" wants to access your profile

This application will have access to:
- Your name and email address
- Your profile picture

[Deny] [Allow]
```

### 5.5 Application API Keys

For server-to-server authentication:

- Generate API keys for applications
- Key rotation
- Scoped permissions

### 5.6 Webhooks

Notify applications of auth events:

- User logged in
- User logged out
- User profile updated
- User access revoked

---

## Implementation Timeline

### Pre-work: Fix Current Deployment Issues (1-2 days)
**MUST DO FIRST - See DEPLOYMENT_FIXES.md for details**

- [ ] Fix localhost redirect in frontend
  - Update Dockerfile with VITE_API_URL build arg
  - Update GitHub Actions workflow
  - Rebuild and deploy frontend
- [ ] Move migrations to backend startup
  - Add golang-migrate to backend
  - Update main.go to run migrations
  - Remove migration job from Helm chart
  - Test and deploy
- [ ] Fix automatic deployment
  - Install ArgoCD Image Updater OR
  - Use commit SHA tags OR
  - Add manual restart to workflow

**Why this is critical:** Current deployment is broken (localhost redirects, stuck migration job). Must fix before adding multi-solution features.

### Week 1-2: Database and Backend Foundation
- [ ] Create database migrations (002_add_applications.up.sql)
  - applications table
  - application_users table
  - Update refresh_tokens and auth_audit_log
- [ ] Add Application and ApplicationUser models
- [ ] Seed default auth-admin application
- [ ] Update JWT token generation with app context
  - Add app_id and role to JWT claims
  - Update token generation functions

### Week 2-3: Core API Implementation
- [ ] Implement application CRUD endpoints
  - POST /api/applications
  - GET /api/applications
  - GET /api/applications/:slug
  - PUT /api/applications/:id
  - DELETE /api/applications/:id
- [ ] Implement application user management endpoints
  - POST /api/applications/:slug/users
  - GET /api/applications/:slug/users
  - PUT /api/applications/:slug/users/:user_id
  - DELETE /api/applications/:slug/users/:user_id
- [ ] Update OAuth flow with application context
  - Add ?app=slug parameter support
  - Store app context in OAuth state
  - Generate tokens with app context
- [ ] Implement dynamic CORS middleware
  - Query database for allowed origins
  - Cache origins in memory
  - Refresh cache periodically

### Week 3-4: Frontend Updates
- [ ] Create universal login page
  - Accept ?app=slug parameter
  - Display application branding
  - Handle OAuth redirect with app context
- [ ] Build application management UI
  - List applications
  - Create/edit application
  - View application details
- [ ] Create application user management UI
  - List users per application
  - Add/remove users
  - Update user roles
- [ ] Update admin dashboard
  - Show multi-application stats
  - Quick switcher between apps

### Week 4-5: Testing and Documentation
- [ ] Write integration tests
  - Test OAuth flow with multiple apps
  - Test token validation with app context
  - Test CORS with multiple origins
- [ ] Test multi-application flows
  - User accessing multiple apps
  - Token isolation between apps
  - Role differences per app
- [ ] Document integration guide for solutions
  - How to register an application
  - How to validate JWT tokens
  - How to implement Casbin locally
- [ ] Create migration guide for existing solutions
  - Step-by-step migration process
  - Code examples for each solution
  - Testing checklist

### Week 5-6: Solution Integration
- [ ] Register hackaton-web2 as application
  - Create application entry
  - Configure OAuth settings
  - Update hackaton backend to validate tokens
  - Migrate existing users
- [ ] Register sharon as application
  - Create application entry
  - Configure OAuth settings
  - Update sharon backend to validate tokens
  - Migrate existing users
- [ ] Register option-platform as application
  - Create application entry
  - Configure OAuth settings
  - Update option-platform backend to validate tokens
  - Migrate existing users

### Week 6+: Advanced Features (Optional)
- [ ] Email/password authentication
- [ ] Additional OAuth providers
- [ ] SSO capabilities
- [ ] User consent screens
- [ ] Application API keys
- [ ] Webhooks for auth events

---

## Security Considerations

### 1. Application Isolation

- Ensure tokens include application ID
- Validate application ID in all operations
- Prevent cross-application token usage

### 2. Secret Management

- Never expose OAuth secrets in API responses
- Store secrets encrypted in database (consider using sealed secrets)
- Rotate secrets regularly

### 3. CORS Security

- Validate origins against database
- Use exact match, not wildcards
- Cache allowed origins for performance

### 4. Rate Limiting

- Implement rate limiting per application
- Prevent abuse from malicious applications
- Monitor authentication attempts

### 5. Audit Logging

- Log all application creation/updates
- Log all user access grants/revocations
- Log all authentication attempts with application context

### 6. Application Validation

- Validate redirect URLs to prevent open redirects
- Enforce HTTPS for all production redirects
- Validate slug format and uniqueness

---

## Migration Strategy for Existing Solutions

### Zero-Downtime Migration

1. **Add auth-service as application** without removing existing auth
2. **Run both auth systems in parallel**
3. **Gradually migrate users** to new system
4. **Monitor for issues**
5. **Remove old auth code** once migration complete

### User Data Migration

1. **Match users by email** between old and new system
2. **Create application_users entries** for existing users
3. **Preserve user roles and permissions**
4. **Notify users** of auth system change (if needed)

### Rollback Plan

1. Keep old authentication code for 2-4 weeks
2. Monitor error rates and user complaints
3. If issues arise, revert to old system
4. Fix issues and retry migration

---

## Testing Strategy

### Unit Tests

- Test application CRUD operations
- Test application user management
- Test JWT generation with app context
- Test dynamic CORS validation

### Integration Tests

- Test full OAuth flow with application context
- Test token validation across applications
- Test application isolation
- Test cross-origin requests

### End-to-End Tests

- Test login from multiple applications
- Test user accessing multiple applications
- Test token refresh with application context
- Test logout from one application

### Load Tests

- Test concurrent logins from multiple applications
- Test token validation performance
- Test CORS validation with many allowed origins
- Test database query performance with many applications

---

## Monitoring and Observability

### Metrics to Track

1. **Authentication Metrics**
   - Logins per application
   - Failed login attempts
   - Token generation rate
   - Token validation rate

2. **Application Metrics**
   - Active applications
   - Applications by user count
   - Most used applications
   - Application errors

3. **User Metrics**
   - Users per application
   - Multi-application users
   - New user registrations
   - User access revocations

4. **Performance Metrics**
   - API response times
   - Database query performance
   - Token validation latency
   - CORS check latency

### Alerts

- Failed authentication rate threshold
- Application creation by unknown user
- Unusual token validation failures
- Database connection issues
- High API error rate

---

## Documentation Requirements

### For Application Developers

1. **Integration Guide**
   - How to register an application
   - How to integrate OAuth flow
   - How to validate JWT tokens
   - Example code for common frameworks

2. **API Reference**
   - All endpoints with examples
   - Error responses
   - Rate limits
   - Webhook documentation

3. **SDK Documentation**
   - TypeScript client library
   - Go client library
   - Python client library (if created)

### For End Users

1. **Privacy Policy** - How user data is shared across applications
2. **Terms of Service** - Rules for using the auth service
3. **FAQ** - Common questions about multi-application access

### For Administrators

1. **Admin Guide** - How to manage applications and users
2. **Security Best Practices** - How to secure applications
3. **Troubleshooting Guide** - Common issues and solutions

---

## Success Criteria

### Phase 1 Success (Database + Backend)
- [ ] Can create, read, update, delete applications via API
- [ ] Can add/remove users from applications
- [ ] JWT tokens include application ID
- [ ] OAuth flow works with application context
- [ ] Dynamic CORS works for multiple origins

### Phase 2 Success (Frontend)
- [ ] Universal login page works for all applications
- [ ] Admin can manage applications via UI
- [ ] Admin can manage application users via UI
- [ ] Application branding appears correctly

### Phase 3 Success (Integration)
- [ ] At least 2 existing solutions use central auth
- [ ] Users can log in from multiple solutions
- [ ] Tokens work across integrated solutions
- [ ] No authentication-related bugs reported

### Final Success
- [ ] Zero authentication-related incidents for 30 days
- [ ] 100% of solutions use central auth service
- [ ] Positive feedback from solution developers
- [ ] Auth service handles 10,000+ authentications/day
- [ ] API response times < 200ms p95

---

## Risks and Mitigations

### Risk 1: Breaking Existing Solutions
**Mitigation**:
- Maintain backward compatibility
- Run old and new systems in parallel during migration
- Comprehensive testing before rollout

### Risk 2: Performance Degradation
**Mitigation**:
- Cache allowed origins in memory
- Use database connection pooling
- Implement rate limiting
- Load test before production deployment

### Risk 3: Security Vulnerabilities
**Mitigation**:
- Security review of all code changes
- Penetration testing
- Regular dependency updates
- Audit logging for all sensitive operations

### Risk 4: Complex Migration
**Mitigation**:
- Detailed migration guide for each solution
- Phased rollout (one solution at a time)
- Dedicated support during migration
- Rollback plan for each solution

### Risk 5: User Confusion
**Mitigation**:
- Clear communication about changes
- Consistent branding across login flows
- FAQ and support documentation
- Monitor user support tickets

---

## Open Questions

1. **Should we support email/password authentication in addition to OAuth?**
   - Pros: More flexibility, works without OAuth providers
   - Cons: More complexity, password management burden

2. **Should we implement true SSO (single sign-on across all apps)?**
   - Pros: Better user experience
   - Cons: Complex implementation, cookie/session challenges

3. **How should we handle application-specific user roles?**
   - Option A: Simple role string in application_users table
   - Option B: Full RBAC with permissions table
   - Option C: Leave role management to individual solutions

4. **Should applications share user data or keep it isolated?**
   - Current plan: Centralized users, applications can access basic profile
   - Alternative: Each application has its own user data, auth service only handles authentication

5. **What happens when a user is removed from an application?**
   - Should we invalidate all their tokens for that application?
   - Should we notify the application via webhook?

6. **Should we support custom authentication providers per application?**
   - E.g., Enterprise SAML for corporate applications
   - Adds significant complexity but increases flexibility

---

## Conclusion

This plan transforms the auth-service from a single-application authentication system to a robust, multi-tenant authentication platform. The phased approach allows for iterative development and testing, minimizing risk to existing solutions while providing a path to centralized user management.

**Key Benefits:**
- Centralized user management across all solutions
- Simplified authentication for new solutions
- Consistent authentication experience
- Reduced development time for new solutions
- Better security through centralized management

**Next Steps:**
1. Review and approve this plan
2. Create detailed task breakdown for Phase 1
3. Set up development environment and branch
4. Begin database schema implementation
5. Create CI/CD pipeline for testing

**Estimated Total Timeline**: 6-8 weeks for core functionality, 10-12 weeks including advanced features and full solution migration.
