# Auth Service Plan

## Overview

A centralized authentication and authorization service that provides OAuth-based authentication and user management for multiple microservices. This service will handle user authentication via Google OAuth and provide JWT tokens that other services can validate. Each consuming service will handle its own authorization using Casbin for RBAC and ABAC policies.

## Architecture

```
┌─────────────────┐
│  React Frontend │ (Bun + Tailwind + shadcN)
│  (Port 3000)    │
└────────┬────────┘
         │ HTTP
         ▼
┌─────────────────┐
│   Go Backend    │
│   (Port 8080)   │
├─────────────────┤
│ • Google OAuth  │
│ • JWT Tokens    │
│ • User CRUD     │
│ • Token Refresh │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│   PostgreSQL    │
│   (Port 5432)   │
└─────────────────┘

Integration Flow:
┌──────────────┐      JWT Token       ┌──────────────┐
│ Auth Service │ ───────────────────> │ Other Service│
└──────────────┘                      │ + Casbin     │
                                      └──────────────┘
```

## Tech Stack

### Backend (Go)
- **Framework**: Chi router or Gin
- **Database**: pgx (PostgreSQL driver)
- **OAuth**: golang.org/x/oauth2
- **JWT**: golang-jwt/jwt
- **Migrations**: golang-migrate/migrate
- **Config**: viper or environment variables
- **Validation**: go-playground/validator

### Frontend (React)
- **Runtime**: Bun
- **Framework**: React 18+ with TypeScript
- **Styling**: Tailwind CSS
- **Components**: shadcn/ui
- **Routing**: React Router
- **HTTP Client**: fetch API or axios
- **State Management**: React Context or Zustand (lightweight)

### Database
- **PostgreSQL 15+**
- Connection pooling
- SSL mode for production

## Database Schema

```sql
-- Users table
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    google_id VARCHAR(255) UNIQUE,
    name VARCHAR(255),
    avatar_url TEXT,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Refresh tokens table
CREATE TABLE refresh_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash VARCHAR(255) UNIQUE NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    revoked_at TIMESTAMP,
    CONSTRAINT fk_user FOREIGN KEY (user_id) REFERENCES users(id)
);

-- Service registrations (optional - for tracking which services use this auth)
CREATE TABLE service_registrations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    service_name VARCHAR(100) UNIQUE NOT NULL,
    service_url TEXT NOT NULL,
    public_key TEXT, -- For JWT validation
    created_at TIMESTAMP DEFAULT NOW()
);

-- Audit log
CREATE TABLE auth_audit_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    action VARCHAR(50) NOT NULL, -- LOGIN, LOGOUT, TOKEN_REFRESH, etc.
    ip_address INET,
    user_agent TEXT,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Indexes
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_google_id ON users(google_id);
CREATE INDEX idx_refresh_tokens_user_id ON refresh_tokens(user_id);
CREATE INDEX idx_refresh_tokens_token_hash ON refresh_tokens(token_hash);
CREATE INDEX idx_audit_log_user_id ON auth_audit_log(user_id);
CREATE INDEX idx_audit_log_created_at ON auth_audit_log(created_at);
```

## Backend API Endpoints

### Authentication
- `POST /api/auth/google/login` - Initiate Google OAuth flow
- `GET /api/auth/google/callback` - Google OAuth callback
- `POST /api/auth/refresh` - Refresh access token
- `POST /api/auth/logout` - Revoke refresh token
- `GET /api/auth/me` - Get current user info (requires JWT)

### User Management (Admin)
- `GET /api/users` - List all users (paginated)
- `GET /api/users/:id` - Get user by ID
- `PUT /api/users/:id` - Update user
- `DELETE /api/users/:id` - Soft delete user
- `POST /api/users/:id/activate` - Activate user
- `POST /api/users/:id/deactivate` - Deactivate user

### Health & Metadata
- `GET /health` - Health check
- `GET /api/public-key` - Get JWT validation public key (for other services)

## JWT Token Structure

### Access Token (Short-lived: 15 minutes)
```json
{
  "sub": "user-uuid",
  "email": "user@example.com",
  "name": "User Name",
  "iat": 1234567890,
  "exp": 1234568790,
  "iss": "auth-service"
}
```

### Refresh Token (Long-lived: 7 days)
- Stored as hashed value in database
- Used to obtain new access tokens
- Can be revoked

## Google OAuth Flow

1. User clicks "Sign in with Google" on frontend
2. Frontend redirects to `/api/auth/google/login`
3. Backend redirects to Google OAuth consent screen
4. User approves, Google redirects to `/api/auth/google/callback`
5. Backend:
   - Exchanges code for Google tokens
   - Fetches user info from Google
   - Creates or updates user in database
   - Generates JWT access token and refresh token
   - Sets refresh token as HTTP-only cookie
   - Returns access token in response body
6. Frontend stores access token in memory/context
7. Frontend redirects to dashboard

## Frontend Structure

```
auth-frontend/
├── src/
│   ├── components/
│   │   ├── ui/              # shadcn components
│   │   ├── Auth/
│   │   │   ├── LoginButton.tsx
│   │   │   └── LogoutButton.tsx
│   │   ├── Layout/
│   │   │   ├── Header.tsx
│   │   │   └── Layout.tsx
│   │   └── Users/
│   │       ├── UserList.tsx
│   │       └── UserCard.tsx
│   ├── contexts/
│   │   └── AuthContext.tsx  # Auth state management
│   ├── hooks/
│   │   └── useAuth.ts       # Auth operations
│   ├── services/
│   │   └── api.ts           # API client
│   ├── pages/
│   │   ├── Home.tsx
│   │   ├── Login.tsx
│   │   ├── Dashboard.tsx
│   │   └── Admin/
│   │       └── Users.tsx
│   ├── App.tsx
│   └── main.tsx
├── tailwind.config.js
├── components.json          # shadcn config
└── package.json
```

### Key Frontend Features
- Protected routes with auth guards
- Automatic token refresh
- Logout functionality
- User profile display
- Admin panel for user management

## Casbin Integration for Other Services

### Strategy
The auth service provides authentication only. Other services handle authorization using Casbin locally.

### Integration Steps for Other Services

1. **Receive JWT from Client**
   ```go
   // In other service
   token := extractTokenFromHeader(r)
   claims, err := validateJWT(token, authServicePublicKey)
   ```

2. **Extract User Info**
   ```go
   userID := claims["sub"].(string)
   userEmail := claims["email"].(string)
   ```

3. **Check Permissions with Casbin**
   ```go
   // Load Casbin policy from local database/file
   enforcer, _ := casbin.NewEnforcer("model.conf", "policy.csv")

   // Check permission
   ok, _ := enforcer.Enforce(userID, "resource", "action")
   if !ok {
       http.Error(w, "Forbidden", http.StatusForbidden)
       return
   }
   ```

### Casbin Model Example (RBAC + ABAC)
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

### Example Policies
```csv
p, admin, tickets, read
p, admin, tickets, write
p, admin, tickets, delete
p, user, tickets, read
p, user, tickets, write
g, user-uuid-123, admin
g, user-uuid-456, user
```

### Syncing User Roles
Each service can:
- Maintain its own role assignments in Casbin
- Optionally sync user existence from auth service
- Use webhooks or periodic sync for user updates

## Security Considerations

1. **Token Security**
   - Access tokens: Short-lived (15 min), stored in memory
   - Refresh tokens: HTTP-only cookies, longer-lived (7 days)
   - Use secure, sameSite cookies in production

2. **CORS Configuration**
   - Whitelist specific origins
   - Allow credentials for cookie-based auth

3. **Environment Variables**
   - Never commit secrets to git
   - Use .env files for local dev
   - Use secret managers in production

4. **Rate Limiting**
   - Implement rate limiting on auth endpoints
   - Protect against brute force attacks

5. **HTTPS Only**
   - Enforce HTTPS in production
   - Use HSTS headers

6. **Input Validation**
   - Validate all inputs
   - Sanitize data before database operations

7. **SQL Injection Prevention**
   - Use parameterized queries
   - Use pgx prepared statements

## Environment Variables

### Backend
```env
# Server
PORT=8080
ENV=development

# Database
DATABASE_URL=postgresql://user:password@localhost:5432/authdb?sslmode=disable

# Google OAuth
GOOGLE_CLIENT_ID=your-client-id
GOOGLE_CLIENT_SECRET=your-client-secret
GOOGLE_REDIRECT_URL=http://localhost:8080/api/auth/google/callback

# JWT
JWT_SECRET=your-secret-key
JWT_ACCESS_TOKEN_EXPIRY=15m
JWT_REFRESH_TOKEN_EXPIRY=168h

# CORS
ALLOWED_ORIGINS=http://localhost:3000,http://localhost:5173

# Optional
LOG_LEVEL=debug
```

### Frontend
```env
VITE_API_URL=http://localhost:8080
VITE_GOOGLE_CLIENT_ID=your-client-id
```

## Deployment Strategy

### Development
- Use docker-compose for local development
- Hot reload for both frontend and backend

### Production
- Deploy backend as containerized service
- Deploy frontend to CDN or static hosting
- Use managed PostgreSQL (AWS RDS, Google Cloud SQL, etc.)
- Use Kubernetes for orchestration (optional)
- Set up monitoring and logging

### Docker Compose Example
```yaml
version: '3.8'

services:
  postgres:
    image: postgres:15
    environment:
      POSTGRES_DB: authdb
      POSTGRES_USER: authuser
      POSTGRES_PASSWORD: authpass
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data

  backend:
    build: ./auth-backend
    ports:
      - "8080:8080"
    depends_on:
      - postgres
    environment:
      DATABASE_URL: postgresql://authuser:authpass@postgres:5432/authdb?sslmode=disable
      # ... other env vars

  frontend:
    build: ./auth-frontend
    ports:
      - "3000:3000"
    depends_on:
      - backend

volumes:
  postgres_data:
```

## Implementation Phases

### Phase 1: Core Infrastructure (Week 1)
- Set up Go project structure
- Set up React project with Bun
- Configure PostgreSQL and create schema
- Set up Docker Compose for local dev

### Phase 2: Google OAuth (Week 1-2)
- Implement Google OAuth flow in backend
- Create JWT generation and validation
- Implement refresh token mechanism
- Create login UI in frontend

### Phase 3: User Management (Week 2)
- Implement user CRUD endpoints
- Create admin UI for user management
- Add user profile page
- Implement logout functionality

### Phase 4: Integration & Documentation (Week 2-3)
- Create integration guide for other services
- Provide example Casbin configurations
- Create API documentation
- Set up monitoring and logging

### Phase 5: Security & Testing (Week 3)
- Add rate limiting
- Security audit
- Write unit and integration tests
- Performance testing

## Integration Guide for Other Services

### Quick Start
1. Get JWT public key from auth service
2. Validate incoming JWT tokens
3. Extract user ID from token
4. Use Casbin for local authorization

### Example Go Code for Other Services
```go
package main

import (
    "github.com/casbin/casbin/v2"
    "github.com/golang-jwt/jwt/v5"
)

// Middleware to validate JWT
func AuthMiddleware(publicKey []byte) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            tokenString := extractToken(r)

            token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
                return jwt.ParseRSAPublicKeyFromPEM(publicKey)
            })

            if err != nil || !token.Valid {
                http.Error(w, "Unauthorized", http.StatusUnauthorized)
                return
            }

            claims := token.Claims.(jwt.MapClaims)
            ctx := context.WithValue(r.Context(), "userID", claims["sub"])
            ctx = context.WithValue(ctx, "email", claims["email"])

            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}

// Casbin authorization
func CheckPermission(userID, resource, action string) bool {
    enforcer, _ := casbin.NewEnforcer("model.conf", "policy.csv")
    ok, _ := enforcer.Enforce(userID, resource, action)
    return ok
}
```

## API Documentation

Use Swagger/OpenAPI for API documentation:
- Generate swagger.json from Go code
- Host Swagger UI at `/api/docs`
- Keep documentation in sync with code

## Monitoring & Observability

- **Logging**: Structured logging with zerolog or zap
- **Metrics**: Prometheus metrics
- **Tracing**: OpenTelemetry (optional)
- **Health Checks**: Liveness and readiness probes
- **Alerts**: Set up alerts for failed logins, high error rates

## Future Enhancements

1. **Multi-factor Authentication (MFA)**
2. **Additional OAuth Providers** (GitHub, Microsoft)
3. **Email/Password Authentication** (as fallback)
4. **Password Reset Flow**
5. **Email Verification**
6. **User Session Management** (view active sessions, revoke)
7. **Audit Log UI**
8. **Rate Limiting Dashboard**
9. **Webhook System** (notify services of user changes)
10. **GraphQL API** (alternative to REST)

## Resources & References

- [Google OAuth 2.0 Documentation](https://developers.google.com/identity/protocols/oauth2)
- [JWT Best Practices](https://datatracker.ietf.org/doc/html/rfc8725)
- [Casbin Documentation](https://casbin.org/docs/overview)
- [shadcn/ui Components](https://ui.shadcn.com/)
- [Chi Router](https://github.com/go-chi/chi)
- [pgx PostgreSQL Driver](https://github.com/jackc/pgx)

## Questions to Consider

1. **User Roles**: Should the auth service store basic roles (admin/user), or leave all roles to individual services?
2. **User Data Sync**: How should other services sync user data (webhooks, polling, event bus)?
3. **Session Management**: Should there be a max number of concurrent sessions per user?
4. **Multi-tenancy**: Will this auth service need to support multiple tenants/organizations?
5. **API Versioning**: Should we implement API versioning from the start?
6. **Backup Strategy**: What's the backup and disaster recovery plan for the auth database?

## Success Criteria

- User can sign in with Google OAuth
- JWT tokens are generated and validated correctly
- Other services can validate tokens without calling auth service
- Admin can manage users through UI
- Tokens can be refreshed without re-authentication
- All sensitive data is properly secured
- System is horizontally scalable
- Integration with other services is straightforward
