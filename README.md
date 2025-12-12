# Auth Service

A centralized authentication and authorization service providing Google OAuth authentication and JWT token management for microservices.

## Architecture

- **Backend**: Go with Chi router, PostgreSQL, Google OAuth, JWT (RS256)
- **Frontend**: React + TypeScript + Tailwind CSS + Bun
- **Database**: PostgreSQL 15+

## Features

- Google OAuth 2.0 authentication
- JWT token generation and validation (RS256)
- Rotating refresh tokens
- User management API
- Admin dashboard
- Automatic token refresh
- Audit logging
- RESTful API
- Ready for Casbin integration in other services

## Deployment Options

### Local Development with Docker Compose

For local development and testing.

### Production Deployment to K3s

For production deployment with auto-scaling, TLS, and GitOps. See [K3S_DEPLOYMENT.md](K3S_DEPLOYMENT.md) for complete guide.

---

## Quick Start - Local Development

### Prerequisites

- Docker and Docker Compose
- Google OAuth credentials
- OpenSSL (for generating RSA keys)

### 1. Clone and Configure

```bash
cd auth-service
cp .env.example .env
```

Edit `.env` and add your Google OAuth credentials:
- Get credentials from [Google Cloud Console](https://console.cloud.google.com/)
- Create OAuth 2.0 client with redirect URI: `http://localhost:8080/api/auth/google/callback`

### 2. Generate RSA Keys

```bash
mkdir -p backend/keys
openssl genrsa -out backend/keys/private_key.pem 4096
openssl rsa -in backend/keys/private_key.pem -pubout -out backend/keys/public_key.pem
```

### 3. Start Services

```bash
docker-compose up -d
```

Services will be available at:
- Frontend: http://localhost:3000
- Backend API: http://localhost:8080
- PostgreSQL: localhost:5432

### 4. Run Migrations

```bash
# Install golang-migrate (if not already installed)
# macOS: brew install golang-migrate
# Linux: https://github.com/golang-migrate/migrate/releases

cd backend
export DATABASE_URL='postgresql://authuser:authpass@localhost:5432/authdb?sslmode=disable'
make migrate-up
```

### 5. Access the Application

1. Open http://localhost:3000
2. Click "Sign in with Google"
3. Authorize the application
4. You'll be redirected to the dashboard

## Development

### Backend Development

```bash
cd backend

# Install dependencies
go mod download

# Generate RSA keys
make keys

# Run migrations
make migrate-up

# Run the server
make run

# Run tests
make test
```

### Frontend Development

```bash
cd frontend

# Install dependencies
bun install

# Start dev server
bun dev

# Build for production
bun run build
```

## API Endpoints

### Public Endpoints

- `GET /health` - Health check
- `GET /api/public-key` - JWT public key
- `GET /api/auth/google/login` - Initiate OAuth
- `GET /api/auth/google/callback` - OAuth callback
- `POST /api/auth/refresh` - Refresh access token
- `POST /api/auth/logout` - Logout

### Protected Endpoints (Require JWT)

- `GET /api/auth/me` - Get current user
- `GET /api/users` - List users
- `GET /api/users/:id` - Get user
- `PUT /api/users/:id` - Update user
- `DELETE /api/users/:id` - Delete user
- `POST /api/users/:id/activate` - Activate user
- `POST /api/users/:id/deactivate` - Deactivate user

## Integration with Other Services

### Step 1: Get Public Key

Fetch the public key from the auth service once and cache it:

```bash
curl http://localhost:8080/api/public-key > public_key.pem
```

### Step 2: Validate JWT Tokens

Example Go code for validating tokens in your services:

```go
package main

import (
    "crypto/rsa"
    "crypto/x509"
    "encoding/pem"
    "fmt"
    "io/ioutil"
    "net/http"

    "github.com/golang-jwt/jwt/v5"
)

func loadPublicKey(path string) (*rsa.PublicKey, error) {
    data, err := ioutil.ReadFile(path)
    if err != nil {
        return nil, err
    }

    block, _ := pem.Decode(data)
    if block == nil {
        return nil, fmt.Errorf("failed to decode PEM block")
    }

    pub, err := x509.ParsePKIXPublicKey(block.Bytes)
    if err != nil {
        return nil, err
    }

    return pub.(*rsa.PublicKey), nil
}

func validateToken(tokenString string, publicKey *rsa.PublicKey) (*jwt.Token, error) {
    return jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
        if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
            return nil, fmt.Errorf("unexpected signing method")
        }
        return publicKey, nil
    })
}
```

### Step 3: Use Casbin for Authorization

```go
package main

import (
    "github.com/casbin/casbin/v2"
)

// Initialize Casbin enforcer
enforcer, _ := casbin.NewEnforcer("model.conf", "policy.csv")

// Extract user ID from JWT claims
claims := token.Claims.(jwt.MapClaims)
userID := claims["sub"].(string)

// Check permission
ok, _ := enforcer.Enforce(userID, "resource", "action")
if !ok {
    http.Error(w, "Forbidden", http.StatusForbidden)
    return
}
```

### Casbin Model Example (model.conf)

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

### Casbin Policy Example (policy.csv)

```csv
p, admin, tickets, read
p, admin, tickets, write
p, admin, tickets, delete
p, user, tickets, read
p, user, tickets, write
g, user-uuid-123, admin
g, user-uuid-456, user
```

## Project Structure

```
auth-service/
├── backend/                 # Go backend
│   ├── cmd/api/            # Entry point
│   ├── internal/           # Internal packages
│   │   ├── auth/          # Auth logic
│   │   ├── config/        # Configuration
│   │   ├── database/      # Database connection
│   │   ├── handlers/      # HTTP handlers
│   │   ├── middleware/    # Middleware
│   │   └── models/        # Data models
│   ├── pkg/jwt/           # JWT utilities
│   ├── migrations/        # Database migrations
│   ├── Dockerfile
│   ├── Makefile
│   └── README.md
├── frontend/              # React frontend
│   ├── src/
│   │   ├── components/   # React components
│   │   ├── contexts/     # React contexts
│   │   ├── pages/        # Page components
│   │   └── services/     # API client
│   ├── Dockerfile
│   ├── nginx.conf
│   └── README.md
├── docker-compose.yml    # Docker Compose config
├── plan.md              # Detailed implementation plan
└── README.md            # This file
```

## Security Features

- RS256 JWT signing (asymmetric)
- HTTP-only cookies for refresh tokens
- Rotating refresh tokens
- CORS protection
- Secure cookie settings
- Audit logging
- Token expiration (15 min access, 7 days refresh)
- Input validation
- SQL injection prevention (parameterized queries)

## Database Schema

- **users**: User accounts with Google OAuth info
- **refresh_tokens**: Rotating refresh tokens
- **auth_audit_log**: Authentication event logging

See `backend/migrations/001_initial_schema.up.sql` for full schema.

## Configuration

### Backend Environment Variables

See `backend/.env.example` for all available options:
- Server config (PORT, ENV)
- Database (DATABASE_URL)
- Google OAuth (CLIENT_ID, CLIENT_SECRET)
- JWT settings (key paths, expiry times)
- CORS (ALLOWED_ORIGINS)
- Admin emails

### Frontend Environment Variables

See `frontend/.env.example`:
- API URL (VITE_API_URL)

## Deployment

### Docker Compose (Recommended for Local/Development)

```bash
docker-compose up -d
```

### Kubernetes

Create Kubernetes manifests for:
- PostgreSQL StatefulSet
- Backend Deployment
- Frontend Deployment
- Services and Ingress

Example manifests available in the plan.md.

### Manual Deployment

1. Deploy PostgreSQL
2. Run migrations
3. Deploy backend with environment variables
4. Deploy frontend (static build to CDN/nginx)
5. Configure domain and SSL

## Monitoring & Logging

- Structured logging in backend
- Audit log for auth events
- Health check endpoint: `/health`
- Database connection health checks

## Troubleshooting

### Backend won't start
- Check DATABASE_URL is correct
- Ensure PostgreSQL is running
- Verify RSA keys exist in `backend/keys/`
- Check Google OAuth credentials

### Frontend can't connect to backend
- Verify VITE_API_URL in frontend/.env
- Check CORS ALLOWED_ORIGINS in backend
- Ensure backend is running on port 8080

### OAuth redirect fails
- Verify Google OAuth redirect URI matches exactly
- Check GOOGLE_REDIRECT_URL in backend
- Ensure cookies are enabled in browser

### Token refresh fails
- Check refresh token cookie is being set
- Verify cookie SameSite and Secure settings
- Check JWT expiry times

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Write tests
5. Submit a pull request

## License

MIT License - See LICENSE file for details

## K3s Production Deployment

For production deployment to Kubernetes (K3s), see the comprehensive deployment guide:

📘 **[K3S_DEPLOYMENT.md](K3S_DEPLOYMENT.md)** - Complete K3s deployment guide

Key features of K3s deployment:
- **GitOps with ArgoCD** - Automated deployment and synchronization
- **Auto-scaling** - Horizontal pod autoscaling based on load
- **TLS/SSL** - Automatic HTTPS with Let's Encrypt certificates
- **High Availability** - Multiple replicas with load balancing
- **Persistent Storage** - PostgreSQL data persistence
- **Sealed Secrets** - Secure secret management
- **Health Checks** - Liveness and readiness probes
- **Monitoring** - Integration with Prometheus and Grafana

### Quick K3s Deploy

```bash
# 1. Prepare secrets and configuration
./scripts/prepare-k3s-deployment.sh

# 2. Push to GitHub (triggers CI/CD)
git add .
git commit -m "Deploy to K3s"
git push

# 3. Add to ArgoCD app-of-apps
# Edit k3s-infra/clusters/main/apps/app-of-apps.yaml

# 4. Apply configuration
kubectl apply -f k3s-infra/clusters/main/apps/app-of-apps.yaml

# 5. Monitor deployment
argocd app get auth-service
kubectl get pods -n auth-service -w
```

The service will be available at: **https://auth.w.viboholic.com**

## Resources

- [Plan Document](plan.md) - Detailed implementation plan and architecture decisions
- [K3s Deployment Guide](K3S_DEPLOYMENT.md) - Complete Kubernetes deployment guide
- [Integration Guide](INTEGRATION.md) - Guide for integrating other services
- [Backend README](backend/README.md) - Backend-specific documentation
- [Frontend README](frontend/README.md) - Frontend-specific documentation
- [Google OAuth 2.0](https://developers.google.com/identity/protocols/oauth2)
- [Casbin](https://casbin.org/docs/overview)
- [JWT Best Practices](https://datatracker.ietf.org/doc/html/rfc8725)
